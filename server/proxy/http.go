package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beego/beego"
	"github.com/beego/beego/logs"
	"github.com/djylb/nps/bridge"
	"github.com/djylb/nps/lib/common"
	"github.com/djylb/nps/lib/conn"
	"github.com/djylb/nps/lib/file"
	"github.com/djylb/nps/lib/goroutine"
	"github.com/djylb/nps/server/connection"
)

var localTCPAddr = &net.TCPAddr{IP: net.ParseIP("127.0.0.1")}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 32*1024)
	},
}

type flowConn struct {
	io.ReadWriteCloser
	fakeAddr net.Addr
	host     *file.Host
	flowIn   int64
	flowOut  int64
	once     sync.Once
}

func (c *flowConn) Read(p []byte) (int, error) {
	n, err := c.ReadWriteCloser.Read(p)
	n64 := int64(n)
	atomic.AddInt64(&c.flowIn, n64)
	c.host.Client.Flow.Add(n64, n64)
	return n, err
}

func (c *flowConn) Write(p []byte) (int, error) {
	n, err := c.ReadWriteCloser.Write(p)
	n64 := int64(n)
	atomic.AddInt64(&c.flowOut, n64)
	c.host.Client.Flow.Add(n64, n64)
	return n, err
}

func (c *flowConn) Close() error {
	c.once.Do(func() {
		//c.host.Client.Flow.Add(c.flowIn, c.flowOut)
		c.host.Flow.Add(c.flowOut, c.flowIn)
	})
	return c.ReadWriteCloser.Close()
}

func (c *flowConn) LocalAddr() net.Addr              { return c.fakeAddr }
func (c *flowConn) RemoteAddr() net.Addr             { return c.fakeAddr }
func (*flowConn) SetDeadline(t time.Time) error      { return nil }
func (*flowConn) SetReadDeadline(t time.Time) error  { return nil }
func (*flowConn) SetWriteDeadline(t time.Time) error { return nil }

type httpServer struct {
	BaseServer
	httpPort      int
	httpsPort     int
	httpServer    *http.Server
	httpsServer   *http.Server
	httpsListener net.Listener
	httpOnlyPass  string
	addOrigin     bool
	httpPortStr   string
	httpsPortStr  string
}

func NewHttp(bridge *bridge.Bridge, task *file.Tunnel, httpPort, httpsPort int, httpOnlyPass string, addOrigin bool) *httpServer {
	allowLocalProxy, _ := beego.AppConfig.Bool("allow_local_proxy")
	return &httpServer{
		BaseServer: BaseServer{
			task:            task,
			bridge:          bridge,
			allowLocalProxy: allowLocalProxy,
			Mutex:           sync.Mutex{},
		},
		httpPort:     httpPort,
		httpsPort:    httpsPort,
		httpOnlyPass: httpOnlyPass,
		addOrigin:    addOrigin,
		httpPortStr:  strconv.Itoa(httpPort),
		httpsPortStr: strconv.Itoa(httpsPort),
	}
}

func (s *httpServer) Start() error {
	var err error
	s.errorContent, err = common.ReadAllFromFile(filepath.Join(common.GetRunPath(), "web", "static", "page", "error.html"))
	if err != nil {
		s.errorContent = []byte("nps 404")
	}

	if s.httpPort > 0 {
		s.httpServer = s.NewServer(s.httpPort, "http")
		go func() {
			l, err := connection.GetHttpListener()
			if err != nil {
				logs.Error("Failed to start HTTP listener: %v", err)
				os.Exit(0)
			}
			logs.Info("HTTP server listening on port %d", s.httpPort)
			if err := s.httpServer.Serve(l); err != nil {
				logs.Error("HTTP server stopped: %v", err)
				os.Exit(0)
			}
		}()
	}

	if s.httpsPort > 0 {
		s.httpsServer = s.NewServer(s.httpsPort, "https")
		go func() {
			s.httpsListener, err = connection.GetHttpsListener()
			if err != nil {
				logs.Error("Failed to start HTTPS listener: %v", err)
				os.Exit(0)
			}
			logs.Info("HTTPS server listening on port %d", s.httpsPort)
			if err := NewHttpsServer(s.httpsListener, s.bridge, s.task).Start(); err != nil {
				logs.Error("HTTPS server stopped: %v", err)
				os.Exit(0)
			}
		}()
	}
	return nil
}

func (s *httpServer) Close() error {
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	if s.httpsServer != nil {
		s.httpsServer.Close()
	}
	if s.httpsListener != nil {
		s.httpsListener.Close()
	}
	return nil
}

func (s *httpServer) handleProxy(w http.ResponseWriter, r *http.Request) {
	// 获取 host 配置
	host, err := file.GetDb().GetInfoByHost(r.Host, r)
	if err != nil {
		http.Error(w, "404 Host not found", http.StatusNotFound)
		logs.Debug("Host not found: %s %s %s", r.URL.Scheme, r.Host, r.RequestURI)
		return
	}

	// TCP 连接数统计
	//host.Client.CutConn()
	defer host.Client.AddConn()

	// IP 黑名单检查
	clientIP := common.GetIpByAddr(r.RemoteAddr)
	if IsGlobalBlackIp(clientIP) || common.IsBlackIp(clientIP, host.Client.VerifyKey, host.Client.BlackIpList) {
		//http.Error(w, "403 Forbidden", http.StatusForbidden)
		logs.Warn("Blocked IP: %s", clientIP)
		return
	}

	// HTTP-Only 请求处理
	isHttpOnlyRequest := (s.httpOnlyPass != "" && r.Header.Get("X-NPS-Http-Only") == s.httpOnlyPass)
	if isHttpOnlyRequest {
		r.Header.Del("X-NPS-Http-Only")
	}

	// 自动 301 跳转 HTTPS
	if !isHttpOnlyRequest && host.AutoHttps && r.TLS == nil {
		redirectHost := common.RemovePortFromHost(r.Host)
		if s.httpsPort != 443 {
			redirectHost += ":" + s.httpsPortStr
		}
		http.Redirect(w, r, "https://"+redirectHost+r.RequestURI, http.StatusMovedPermanently)
		return
	}

	// 连接数和流量控制
	if err := s.CheckFlowAndConnNum(host.Client); err != nil {
		http.Error(w, "Access denied: "+err.Error(), http.StatusTooManyRequests)
		logs.Warn("Connection limit exceeded, client id %d, host id %d, error %s", host.Client.Id, host.Id, err.Error())
		return
	}

	// HTTP 认证
	if r.Header.Get("Upgrade") == "" {
		if err := s.auth(r, nil, host.Client.Cnf.U, host.Client.Cnf.P, s.task); err != nil {
			logs.Warn("Unauthorized request from %s", r.RemoteAddr)
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "401 Unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// 获取目标地址
	targetAddr, err := host.Target.GetRandomTarget()
	if err != nil {
		logs.Warn("No backend found for host: %s Err: %s", r.Host, err.Error())
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}

	logs.Info("%s request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.URL.Path, r.RemoteAddr, targetAddr)

	// WebSocket 请求单独处理
	if r.Method == "CONNECT" || r.Header.Get("Upgrade") != "" || r.Header.Get(":protocol") != "" {
		s.handleWebsocket(w, r, host, targetAddr, isHttpOnlyRequest)
		return
	}

	// 创建 HTTP 反向代理
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			//req = req.WithContext(context.WithValue(req.Context(), "origReq", r))
			if host.TargetIsHttps {
				req.URL.Scheme = "https"
			} else {
				req.URL.Scheme = "http"
			}
			req.URL.Host = r.Host
			//logs.Debug("Director: set req.URL.Scheme=%s, req.URL.Host=%s", req.URL.Scheme, req.URL.Host)
			common.ChangeHostAndHeader(req, host.HostChange, host.HeaderChange, isHttpOnlyRequest)
			if isHttpOnlyRequest {
				// 传递 X-Forwarded 头
				req.Header.Set("X-Forwarded-Proto", r.URL.Scheme)
				req.Header.Set("X-Scheme", r.URL.Scheme)
				if r.URL.Scheme == "https" {
					req.Header.Set("X-Forwarded-Ssl", "on")
					req.Header.Set("X-Forwarded-Port", s.httpsPortStr)
				} else {
					req.Header.Set("X-Forwarded-Port", s.httpPortStr)
				}
			}
		},
		Transport: &http.Transport{
			ResponseHeaderTimeout: 60 * time.Second,
			//DisableKeepAlives:     true,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName: func() string {
					if host.TargetIsHttps {
						if host.HostChange != "" {
							return host.HostChange
						}
						return common.RemovePortFromHost(r.Host)
					}
					return ""
				}(),
			},
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				//logs.Debug("DialContext: start dialing; network=%s, addr=%s, using targetAddr=%s", network, addr, targetAddr)
				link := conn.NewLink("tcp", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, s.allowLocalProxy && host.Target.LocalProxy)
				target, err := s.bridge.SendLinkInfo(host.Client.Id, link, nil)
				if err != nil {
					logs.Notice("DialContext: connection to host %s (target %s) failed: %v", r.Host, targetAddr, err)
					return nil, err
				}
				rawConn := conn.GetConn(target, link.Crypt, link.Compress, host.Client.Rate, true)
				return &flowConn{
					ReadWriteCloser: rawConn,
					fakeAddr:        localTCPAddr,
					host:            host,
				}, nil
			},
		},
		ModifyResponse: func(resp *http.Response) error {
			// 处理 CORS
			if host.AutoCORS {
				origin := resp.Request.Header.Get("Origin")
				if origin != "" && resp.Header.Get("Access-Control-Allow-Origin") == "" {
					logs.Debug("ModifyResponse: setting CORS headers for origin=%s", origin)
					resp.Header.Set("Access-Control-Allow-Origin", origin)
					resp.Header.Set("Access-Control-Allow-Credentials", "true")
				}
			}
			return nil
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			if err == io.EOF {
				logs.Info("ErrorHandler: io.EOF encountered, writing 521")
				rw.WriteHeader(521)
				return
			}
			logs.Warn("ErrorHandler: proxy error: method=%s, URL=%s, error=%v", req.Method, req.URL.String(), err)
			http.Error(rw, "502 Bad Gateway", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(w, r)
}

func (s *httpServer) handleWebsocket(w http.ResponseWriter, r *http.Request, host *file.Host, targetAddr string, isHttpOnlyRequest bool) {
	logs.Info("%s websocket request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.URL.Path, r.RemoteAddr, targetAddr)

	link := conn.NewLink("tcp", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, host.Target.LocalProxy)
	targetConn, err := s.bridge.SendLinkInfo(host.Client.Id, link, nil)
	if err != nil {
		logs.Notice("handleWebsocket: connection to target %s failed: %v", link.Host, err)
		http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
		return
	}
	rawConn := conn.GetConn(targetConn, link.Crypt, link.Compress, host.Client.Rate, true)
	wsConn := &flowConn{
		ReadWriteCloser: rawConn,
		fakeAddr:        localTCPAddr,
		host:            host,
	}
	var netConn net.Conn = wsConn

	if host.TargetIsHttps {
		serverName := host.HostChange
		if serverName == "" {
			serverName = common.RemovePortFromHost(r.Host)
		}
		//logs.Debug("handleWebsocket: performing TLS handshake, serverName=%s", serverName)
		tlsConf := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         serverName,
		}
		netConn = tls.Client(netConn, tlsConf)
		if err := netConn.(*tls.Conn).Handshake(); err != nil {
			logs.Error("handleWebsocket: TLS handshake with backend failed: %v", err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
			return
		}
		//logs.Debug("handleWebsocket: TLS handshake succeeded")
	}

	common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, isHttpOnlyRequest)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "WebSocket hijacking not supported", http.StatusInternalServerError)
		logs.Error("handleWebsocket: WebSocket hijacking not supported.")
		return
	}
	clientConn, clientBuf, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, "WebSocket hijacking failed", http.StatusInternalServerError)
		logs.Error("handleWebsocket: WebSocket hijacking failed.")
		return
	}
	//defer clientConn.Close()
	if err := r.Write(netConn); err != nil {
		logs.Error("handleWebsocket: failed to write handshake to backend: %v", err)
		netConn.Close()
		clientConn.Close()
		return
	}

	backendReader := bufio.NewReader(netConn)
	resp, err := http.ReadResponse(backendReader, r)
	if err != nil {
		logs.Error("handleWebsocket: failed to read handshake response from backend: %v", err)
		netConn.Close()
		clientConn.Close()
		return
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		logs.Error("handleWebsocket: unexpected status code in handshake: %d", resp.StatusCode)
		netConn.Close()
		clientConn.Close()
		return
	}

	if err := resp.Write(clientBuf); err != nil {
		logs.Error("handleWebsocket: failed to write handshake response to client: %v", err)
		netConn.Close()
		clientConn.Close()
		return
	}
	if err := clientBuf.Flush(); err != nil {
		logs.Error("handleWebsocket: failed to flush handshake response to client: %v", err)
		netConn.Close()
		clientConn.Close()
		return
	}

	join(clientConn, netConn, host.Flow, s.task, r.RemoteAddr)
}

func join(c1, c2 net.Conn, flow *file.Flow, task *file.Tunnel, remote string) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		if err := goroutine.CopyBuffer(c1, c2, flow, task, remote); err != nil {
			c1.Close()
			c2.Close()
		}
		wg.Done()
	}()
	go func() {
		if err := goroutine.CopyBuffer(c2, c1, flow, task, remote); err != nil {
			c1.Close()
			c2.Close()
		}
		wg.Done()
	}()
	wg.Wait()
}

func (s *httpServer) NewServer(port int, scheme string) *http.Server {
	return &http.Server{
		Addr: ":" + strconv.Itoa(port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Scheme = scheme
			s.handleProxy(w, r)
		}),
		// Disable HTTP/2.
		//TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}
