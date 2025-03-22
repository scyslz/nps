package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/beego/beego"
	"github.com/beego/beego/logs"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/crypt"
	"github.com/djylb/nps/lib/file"
	"github.com/pkg/errors"
)

type HttpsEntry struct {
	Listener    *HttpsListener
	CertContent string
	KeyContent  string
}

type HttpsServer struct {
	httpServer
	listener net.Listener
	entryMap sync.Map

	sslCacheTimeout int
	defaultCertFile string
	defaultKeyFile  string
	exitChan        chan struct{}
}

func NewHttpsServer(l net.Listener, bridge NetBridge, task *file.Tunnel) *HttpsServer {
	allowLocalProxy, _ := beego.AppConfig.Bool("allow_local_proxy")
	https := &HttpsServer{
		listener: l,
		httpServer: httpServer{
			BaseServer: BaseServer{
				task:            task,
				bridge:          bridge,
				allowLocalProxy: allowLocalProxy,
				Mutex:           sync.Mutex{},
			},
		},
		exitChan: make(chan struct{}),
	}

	https.sslCacheTimeout = 60
	if cacheTime, err := beego.AppConfig.Int("ssl_cache_timeout"); err == nil {
		https.sslCacheTimeout = cacheTime
	}

	https.defaultCertFile = beego.AppConfig.String("https_default_cert_file")
	https.defaultKeyFile = beego.AppConfig.String("https_default_key_file")

	https.startCacheCleaner(time.Duration(https.sslCacheTimeout) * time.Second)

	return https
}

func (https *HttpsServer) startCacheCleaner(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				count := 0
				https.entryMap.Range(func(key, value interface{}) bool {
					count++
					hostId, ok := key.(int)
					if !ok {
						return true
					}
					entry, ok := value.(*HttpsEntry)
					if !ok {
						return true
					}

					host, err := file.GetDb().GetHostById(hostId)
					if err != nil {
						logs.Info("Asynchronous cleanup: hostId %d does not exist, releasing resource", hostId)
						https.cleanupEntry(hostId, entry)
						return true
					}

					if host.HttpsJustProxy {
						logs.Info("Asynchronous cleanup: hostId %d is in proxy-only mode, releasing Listener", hostId)
						https.cleanupEntry(hostId, entry)
						return true
					}

					certContent, keyContent := https.getCertAndKey(host)
					if certContent == "" || keyContent == "" {
						logs.Info("Asynchronous cleanup: hostId %d certificate is empty, releasing Listener", hostId)
						https.cleanupEntry(hostId, entry)
						return true
					}

					if entry.CertContent != certContent || entry.KeyContent != keyContent {
						logs.Info("Asynchronous cleanup: hostId %d certificate has changed, releasing Listener", hostId)
						https.cleanupEntry(hostId, entry)
					}

					return true
				})
				logs.Debug("Current number of Listeners: %d", count)
			case <-https.exitChan:
				logs.Info("Cache cleaner stopping")
				return
			}
		}
	}()
}

func (https *HttpsServer) cleanupEntry(hostId int, entry *HttpsEntry) {
	if entry.Listener != nil {
		err := entry.Listener.Close()
		if err != nil {
			logs.Error("Failed to close Listener for hostId %d: %v", hostId, err)
		}
	}
	https.entryMap.Delete(hostId)
}

func (https *HttpsServer) Start() error {
	conn.Accept(https.listener, func(c net.Conn) {
		helloInfo, rb, err := crypt.ReadClientHello(c)
		if err != nil || helloInfo == nil {
			logs.Warn("Failed to read clientHello from %s, err=%v", c.RemoteAddr(), err)
			// Check if the request is an HTTP request.
			checkHTTPAndRedirect(c, rb)
			return
		}

		serverName := helloInfo.ServerName
		if serverName == "" {
			logs.Debug("IP access to HTTPS port is not allowed. Remote address: %s", c.RemoteAddr().String())
			c.Close()
			return
		}

		host, err := file.GetDb().FindCertByHost(serverName)
		if err != nil {
			c.Close()
			logs.Debug("The URL %s cannot be parsed! Remote address: %s", serverName, c.RemoteAddr().String())
			return
		}

		if host.HttpsJustProxy {
			logs.Debug("Certificate handled by backend")
			https.handleHttpsProxy(host, c, rb, serverName)
			return
		}

		if value, ok := https.entryMap.Load(host.Id); ok {
			entry := value.(*HttpsEntry)
			acceptConn := conn.NewConn(c)
			acceptConn.Rb = rb
			entry.Listener.acceptConn <- acceptConn
		} else {
			certContent, keyContent := https.getCertAndKey(host)
			if certContent == "" || keyContent == "" {
				logs.Debug("Certificate handled by backend")
				https.handleHttpsProxy(host, c, rb, serverName)
				return
			}
			l := NewHttpsListener(https.listener)
			https.NewHttps(l, certContent, keyContent)
			newEntry := &HttpsEntry{
				Listener:    l,
				CertContent: certContent,
				KeyContent:  keyContent,
			}
			https.entryMap.Store(host.Id, newEntry)
			acceptConn := conn.NewConn(c)
			acceptConn.Rb = rb
			l.acceptConn <- acceptConn
		}
	})
	return nil
}

func checkHTTPAndRedirect(c net.Conn, rb []byte) {
	c.SetDeadline(time.Now().Add(10 * time.Second))
	defer c.Close()

	logs.Debug("Pre-read rb content: %q", string(rb))

	reader := bufio.NewReader(io.MultiReader(bytes.NewReader(rb), c))
	req, err := http.ReadRequest(reader)
	if err != nil {
		logs.Warn("Failed to parse HTTP request from %s, err=%v", c.RemoteAddr(), err)
		return
	}
	logs.Debug("HTTP Request Sent to HTTPS Port")
	req.URL.Scheme = "https"
	c.SetDeadline(time.Time{})

	_, err = file.GetDb().GetInfoByHost(req.Host, req)
	if err != nil {
		logs.Debug("Host not found: %s %s %s", req.URL.Scheme, req.Host, req.RequestURI)
		return
	}

	redirectURL := "https://" + req.Host + req.RequestURI

	response := "HTTP/1.1 302 Found\r\n" +
		"Location: " + redirectURL + "\r\n" +
		"Content-Length: 0\r\n" +
		"Connection: close\r\n\r\n"

	if _, writeErr := c.Write([]byte(response)); writeErr != nil {
		logs.Error("Failed to write redirect response to %s, err=%v", c.RemoteAddr(), writeErr)
	} else {
		logs.Info("Redirected HTTP request from %s to %s", c.RemoteAddr(), redirectURL)
	}
}

func (https *HttpsServer) getCertAndKey(host *file.Host) (string, string) {
	certContent, keyContent, ok := loadCertPair(host.CertFilePath, host.KeyFilePath)
	if !ok {
		return https.loadDefaultCert()
	}
	return certContent, keyContent
}

func (https *HttpsServer) loadDefaultCert() (string, string) {
	logs.Debug("Loading default certificate")
	certContent, keyContent, ok := loadCertPair(https.defaultCertFile, https.defaultKeyFile)
	if !ok {
		return "", ""
	}
	return certContent, keyContent
}

func loadCertPair(certFile, keyFile string) (certContent, keyContent string, ok bool) {
	var wg sync.WaitGroup
	var certErr, keyErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		certContent, certErr = common.GetCertContent(certFile, "CERTIFICATE")
	}()
	go func() {
		defer wg.Done()
		keyContent, keyErr = common.GetCertContent(keyFile, "PRIVATE")
	}()
	wg.Wait()

	if certErr != nil || keyErr != nil || certContent == "" || keyContent == "" {
		return "", "", false
	}
	return certContent, keyContent, true
}

func (https *HttpsServer) NewHttps(l net.Listener, certText string, keyText string) {
	go func() {
		cert, err := tls.X509KeyPair([]byte(certText), []byte(keyText))
		if err != nil {
			logs.Error("Failed to load certificate: %v", err)
			return
		}
		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h2", "http/1.1"},
		}
		tlsListener := tls.NewListener(l, tlsConfig)
		err = https.NewServer(0, "https").Serve(tlsListener)
		if err != nil {
			logs.Error("HTTPS server error: %v", err)
		}
	}()
}

func (https *HttpsServer) handleHttpsProxy(host *file.Host, c net.Conn, rb []byte, sni string) {
	if err := https.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Debug("Client id %d, host id %d, error %s during https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	defer host.Client.CutConn()

	targetAddr, err := host.Target.GetRandomTarget()
	if err != nil {
		logs.Warn(err.Error())
		c.Close()
		return
	}
	logs.Info("New HTTPS connection, clientId %d, host %s, remote address %s", host.Client.Id, sni, c.RemoteAddr().String())
	https.DealClient(conn.NewConn(c), host.Client, targetAddr, rb, common.CONN_TCP, nil, []*file.Flow{host.Flow, host.Client.Flow}, host.Target.ProxyProtocol, host.Target.LocalProxy, host)
}

func (https *HttpsServer) Close() error {
	close(https.exitChan)
	return https.listener.Close()
}

// HttpsListener wraps a parent listener.
type HttpsListener struct {
	acceptConn     chan *conn.Conn
	parentListener net.Listener
}

func NewHttpsListener(l net.Listener) *HttpsListener {
	return &HttpsListener{parentListener: l, acceptConn: make(chan *conn.Conn)}
}

func (httpsListener *HttpsListener) Accept() (net.Conn, error) {
	httpsConn := <-httpsListener.acceptConn
	if httpsConn == nil {
		return nil, errors.New("failed to get connection")
	}
	return httpsConn, nil
}

func (httpsListener *HttpsListener) Close() error {
	return nil
}

func (httpsListener *HttpsListener) Addr() net.Addr {
	return httpsListener.parentListener.Addr()
}
