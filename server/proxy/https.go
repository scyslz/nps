package proxy

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/cache"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"github.com/pkg/errors"
)

// CertCache 用于存放证书内容及缓存时间
type CertCache struct {
	CertContent string
	KeyContent  string
	CachedAt    time.Time
	Changed     bool
}

type HttpsServer struct {
	httpServer
	listener         net.Listener
	httpsListenerMap sync.Map // key: host.Id (int), value: *HttpsListener
	hostIdCertMap    sync.Map // key: host.Id (int), value: *CertCache

	sslCacheTimeout int    // 证书缓存超时时间，单位秒
	defaultCertFile string // 默认证书文件路径
	defaultKeyFile  string // 默认私钥文件路径
}

func NewHttpsServer(l net.Listener, bridge NetBridge, useCache bool, cacheLen int, task *file.Tunnel) *HttpsServer {
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
	}

	https.useCache = useCache
	if useCache {
		https.cache = cache.New(cacheLen)
	}

	// 读取证书缓存超时时间配置，默认60秒
	https.sslCacheTimeout = 60
	if cacheTime, err := beego.AppConfig.Int("ssl_cache_timeout"); err == nil {
		https.sslCacheTimeout = cacheTime
	}

	// 默认证书路径配置
	https.defaultCertFile = beego.AppConfig.String("https_default_cert_file")
	https.defaultKeyFile = beego.AppConfig.String("https_default_key_file")

	// 启动异步清理任务
	https.startCacheCleaner(30 * time.Second)

	return https
}

// startCacheCleaner 定期清理无效缓存
func (https *HttpsServer) startCacheCleaner(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			i := 0
			https.hostIdCertMap.Range(func(key, value interface{}) bool {
				i++
				hostId, ok := key.(int)
				if !ok {
					return true
				}

				// 获取 Host 信息
				host, err := file.GetDb().GetHostById(hostId)
				if err != nil {
					// Host 不存在，清理资源
					logs.Info("异步清理：hostId %d 不存在，释放资源", hostId)
					https.cleanupListener(hostId)
					// 删除 hostId 证书缓存
					https.hostIdCertMap.Delete(hostId)
				} else {
					// Host 存在，检查是否需要释放 Listener
					if host.HttpsJustProxy {
						logs.Info("异步清理：hostId %d 仅代理模式，释放 Listener", hostId)
						https.cleanupListener(hostId)
					} else if cache, ok := https.hostIdCertMap.Load(hostId); ok {
						certCache := cache.(*CertCache)
						if certCache.CertContent == "" || certCache.KeyContent == "" {
							logs.Info("异步清理：hostId %d 证书为空，释放 Listener", hostId)
							https.cleanupListener(hostId)
						}
					}
				}
				return true
			})

			logs.Debug("当前 Listener 数量: ", i)
		}
	}()
}

// 清理指定 hostId 的 Listener
func (https *HttpsServer) cleanupListener(hostId int) {
	if listener, ok := https.httpsListenerMap.Load(hostId); ok {
		err := listener.(*HttpsListener).Close()
		if err != nil {
			logs.Error("关闭 Listener 失败: %v", err)
		}
		https.httpsListenerMap.Delete(hostId)
	}
}

// Start HTTPS 服务器
func (https *HttpsServer) Start() error {
	conn.Accept(https.listener, func(c net.Conn) {
		serverName, rb := GetServerNameFromClientHello(c)
		// 判断是否为纯 IP 访问（SNI 为空）
		if serverName == "" {
			logs.Debug("IP access to HTTPS port is not allowed. Remote address: %s", c.RemoteAddr().String())
			c.Close()
			return
		}

		r := buildHttpsRequest(serverName)
		host, err := file.GetDb().GetInfoByHost(serverName, r)
		if err != nil {
			c.Close()
			logs.Debug("The URL %s can't be parsed! Remote address: %s", serverName, c.RemoteAddr().String())
			return
		}

		// 处理仅代理模式
		if host.HttpsJustProxy {
			logs.Debug("由后端处理证书")
			https.handleHttps2(host, c, rb, r)
			return
		}

		// 先从 hostIdCertMap 以 host.Id 为 key获取缓存
		cacheVal, ok := https.hostIdCertMap.Load(host.Id)
		var certCache *CertCache
		if ok {
			certCache = cacheVal.(*CertCache)
			// 如果缓存已过期，则重新读取
			if time.Since(certCache.CachedAt) >= time.Duration(https.sslCacheTimeout)*time.Second {
				// 读取 host 指定的证书
				certContent, keyContent := https.getCertAndKey(host)
				// 判断是否发生了更改
				changed := (certCache != nil) && (certCache.CertContent != certContent || certCache.KeyContent != keyContent)
				certCache = &CertCache{
					CertContent: certContent,
					KeyContent:  keyContent,
					CachedAt:    time.Now(),
					Changed:     changed,
				}
				https.hostIdCertMap.Store(host.Id, certCache)
			}
		} else {
			// 第一次加载
			certContent, keyContent := https.getCertAndKey(host)
			certCache = &CertCache{
				CertContent: certContent,
				KeyContent:  keyContent,
				CachedAt:    time.Now(),
				Changed:     false,
			}
			https.hostIdCertMap.Store(host.Id, certCache)
		}

		// 如果缓存中证书为空，则由后端处理
		if certCache.CertContent == "" || certCache.KeyContent == "" {
			logs.Debug("由后端处理证书")
			https.handleHttps2(host, c, rb, r)
		} else {
			// 使用缓存的证书进入 cert 函数
			logs.Debug("使用上传或默认证书")
			https.cert(host, c, rb, certCache)
		}
	})
	return nil
}

func (https *HttpsServer) getCertAndKey(host *file.Host) (string, string) {
	// 获取 host 配置的证书内容
	certContent, certErr := getCertOrKeyContent(host.CertFilePath, "CERTIFICATE")
	if certErr != nil {
		certContent = ""
	}

	keyContent, keyErr := getCertOrKeyContent(host.KeyFilePath, "PRIVATE")
	if keyErr != nil {
		keyContent = ""
	}

	// 如果 host 配置的证书无效，则使用默认证书
	if certContent == "" || keyContent == "" {
		logs.Debug("加载默认证书")
		certContent, certErr = getCertOrKeyContent(https.defaultCertFile, "CERTIFICATE")
		if certErr != nil {
			certContent = ""
		}
		keyContent, keyErr = getCertOrKeyContent(https.defaultKeyFile, "PRIVATE")
		if keyErr != nil {
			keyContent = ""
		}
	}

	return certContent, keyContent
}

// cert 处理 HTTPS 证书
func (https *HttpsServer) cert(host *file.Host, c net.Conn, rb []byte, certCache *CertCache) {
	var l *HttpsListener

	// 检测 Listener 是否存在
	if v, ok := https.httpsListenerMap.Load(host.Id); ok {
		if certCache.Changed {
			// 证书修改过，释放旧 Listener
			err := v.(*HttpsListener).Close()
			if err != nil {
				logs.Error(err)
			}
			https.httpsListenerMap.Delete(host.Id)
		} else {
			l = v.(*HttpsListener)
		}
	}

	if l == nil {
		// 加载新的 HTTPS 监听
		l = NewHttpsListener(https.listener)
		https.NewHttps(l, certCache.CertContent, certCache.KeyContent)
		https.httpsListenerMap.Store(host.Id, l)

		// 更新缓存，表示证书已经生效
		certCache.Changed = false
		https.hostIdCertMap.Store(host.Id, certCache)
	}

	acceptConn := conn.NewConn(c)
	acceptConn.Rb = rb
	l.acceptConn <- acceptConn
}

// handle the https which is just proxy to other client
func (https *HttpsServer) handleHttps2(host *file.Host, c net.Conn, rb []byte, r *http.Request) {
	var targetAddr string
	var err error
	if err := https.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Debug("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	defer host.Client.AddConn()
	if err = https.auth(r, conn.NewConn(c), host.Client.Cnf.U, host.Client.Cnf.P, https.task); err != nil {
		logs.Warn("auth error", err, r.RemoteAddr)
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		logs.Warn(err.Error())
	}
	logs.Info("new https connection, clientId %d, host %s, remote address %s", host.Client.Id, r.Host, c.RemoteAddr().String())
	https.DealClient(conn.NewConn(c), host.Client, targetAddr, rb, common.CONN_TCP, nil, host.Client.Flow, host.Target.ProxyProtocol, host.Target.LocalProxy, nil)
}

// close
func (https *HttpsServer) Close() error {
	return https.listener.Close()
}

// new https server by cert and key file
func (https *HttpsServer) NewHttps(l net.Listener, certFile string, keyFile string) {
	go func() {
		//logs.Error(https.NewServer(0, "https").ServeTLS(l, certFile, keyFile))
		logs.Error(https.NewServerWithTls(0, "https", l, certFile, keyFile))
	}()
}

// handle the https which is just proxy to other client
func (https *HttpsServer) handleHttps(c net.Conn) {
	hostName, rb := GetServerNameFromClientHello(c)
	var targetAddr string
	r := buildHttpsRequest(hostName)
	host, err := file.GetDb().GetInfoByHost(hostName, r)
	if err != nil {
		c.Close()
		logs.Notice("the url %s can't be parsed!", hostName)
		return
	}
	if err := https.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Warn("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	defer host.Client.AddConn()
	if err = https.auth(r, conn.NewConn(c), host.Client.Cnf.U, host.Client.Cnf.P, https.task); err != nil {
		logs.Warn("auth error", err, r.RemoteAddr)
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		logs.Warn(err.Error())
	}
	logs.Trace("new https connection, clientId %d, host %s, remote address %s", host.Client.Id, r.Host, c.RemoteAddr().String())
	https.DealClient(conn.NewConn(c), host.Client, targetAddr, rb, common.CONN_TCP, nil, host.Client.Flow, host.Target.ProxyProtocol, host.Target.LocalProxy, nil)
}

type HttpsListener struct {
	acceptConn     chan *conn.Conn
	parentListener net.Listener
}

// https listener
func NewHttpsListener(l net.Listener) *HttpsListener {
	return &HttpsListener{parentListener: l, acceptConn: make(chan *conn.Conn)}
}

// accept
func (httpsListener *HttpsListener) Accept() (net.Conn, error) {
	httpsConn := <-httpsListener.acceptConn
	if httpsConn == nil {
		return nil, errors.New("get connection error")
	}
	return httpsConn, nil
}

// close
func (httpsListener *HttpsListener) Close() error {
	return nil
}

// addr
func (httpsListener *HttpsListener) Addr() net.Addr {
	return httpsListener.parentListener.Addr()
}

// Read Cert
func getCertOrKeyContent(filePath string, header string) (string, error) {
	if filePath == "" || strings.Contains(filePath, header) {
		return filePath, nil
	}
	fileBytes, err := common.ReadAllFromFile(filePath)
	if err != nil || !strings.Contains(string(fileBytes), header) {
		return "", err
	}
	return string(fileBytes), nil
}

// get server name from connection by read client hello bytes
func GetServerNameFromClientHello(c net.Conn) (string, []byte) {
	buf := make([]byte, 4096)
	data := make([]byte, 4096)
	n, err := c.Read(buf)
	if err != nil {
		return "", nil
	}
	if n < 42 {
		return "", nil
	}
	copy(data, buf[:n])
	clientHello := new(crypt.ClientHelloMsg)
	clientHello.Unmarshal(data[5:n])
	return clientHello.GetServerName(), buf[:n]
}

// build https request
func buildHttpsRequest(hostName string) *http.Request {
	r := new(http.Request)
	r.RequestURI = "/"
	r.URL = new(url.URL)
	r.URL.Scheme = "https"
	r.Host = hostName
	return r
}
