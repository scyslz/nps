package proxy

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/goroutine"
	"github.com/astaxie/beego/logs"
)

type HTTPError struct {
	error
	HTTPCode int
}

type HttpReverseProxy struct {
	//BaseServer
	proxy                 *ReverseProxy
	responseHeaderTimeout time.Duration
}

type flowConn struct {
	io.ReadWriteCloser
	fakeAddr net.Addr
	host     *file.Host
	flowIn   int64
	flowOut  int64
	once     sync.Once
}

func (rp *HttpReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request, host *file.Host) {
	var (
		//host       *file.Host
		targetAddr string
		err        error
	)
	//if host, err = file.GetDb().GetInfoByHost(req.Host, req); err != nil {
	//	rw.WriteHeader(http.StatusNotFound)
	//	rw.Write([]byte(req.Host + " not found"))
	//	return
	//}

	// 删除对认证信息的检查，让后端服务器处理 WebSocket 认证
	/*
	var accountMap map[string]string
	if rp.task.MultiAccount == nil {
		accountMap = nil
	} else {
		accountMap = rp.task.MultiAccount.AccountMap
	}
	if host.Client.Cnf.U != "" && host.Client.Cnf.P != "" && !common.CheckAuth(req, host.Client.Cnf.U, host.Client.Cnf.P, accountMap) {
		rw.WriteHeader(http.StatusUnauthorized)
		rw.Write([]byte("Unauthorized"))
		return
	}
	*/

	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		rw.Write([]byte("502 Bad Gateway"))
		return
	}
	host.Client.CutConn()

	req = req.WithContext(context.WithValue(req.Context(), "host", host))
	req = req.WithContext(context.WithValue(req.Context(), "target", targetAddr))
	req = req.WithContext(context.WithValue(req.Context(), "req", req))

	rp.proxy.ServeHTTP(rw, req, host)

	defer host.Client.AddConn()
}

func (c *flowConn) Read(p []byte) (n int, err error) {
	n, err = c.ReadWriteCloser.Read(p)
	return n, err
}

func (c *flowConn) Write(p []byte) (n int, err error) {
	n, err = c.ReadWriteCloser.Write(p)
	return n, err
}

func (c *flowConn) Close() error {
	//c.once.Do(func() { c.host.Flow.Add(c.flowIn, c.flowOut) })
	return c.ReadWriteCloser.Close()
}

func (c *flowConn) LocalAddr() net.Addr { return c.fakeAddr }

func (c *flowConn) RemoteAddr() net.Addr { return c.fakeAddr }

func (*flowConn) SetDeadline(t time.Time) error { return nil }

func (*flowConn) SetReadDeadline(t time.Time) error { return nil }

func (*flowConn) SetWriteDeadline(t time.Time) error { return nil }

func GetClientAddr(r *http.Request) (*net.TCPAddr, error) {
	// 从 RemoteAddr 提取 IP 和端口
	host, portStr, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil, err // 返回解析错误
	}

	// 解析 IP 地址
	ip := net.ParseIP(host)
	if ip == nil {
		return nil, &net.AddrError{Err: "invalid IP address", Addr: host}
	}

	// 转换端口为整数
	port, err := net.LookupPort("tcp", portStr)
	if err != nil {
		return nil, err // 返回端口解析错误
	}

	// 构造并返回 *net.TCPAddr
	return &net.TCPAddr{
		IP:   ip,
		Port: port,
	}, nil
}

func NewHttpReverseProxy(s *httpServer) *HttpReverseProxy {
	rp := &HttpReverseProxy{
		//BaseServer: BaseServer{
		//	task: s.task, // 从 httpServer 传入 task，确保 task 被正确初始化
		//	allowLocalProxy: s.allowLocalProxy,
		//},
		responseHeaderTimeout: 30 * time.Second,
	}
	local, _ := net.ResolveTCPAddr("tcp", "127.0.0.1")

	proxy := NewReverseProxy(&httputil.ReverseProxy{
		Director: func(r *http.Request) {
			host := r.Context().Value("host").(*file.Host)

			// 检查是否为 HTTP-only 请求
			isHttpOnlyRequest := s.httpOnlyPass != "" && r.Header.Get("X-NPS-Http-Only") == s.httpOnlyPass
			if isHttpOnlyRequest {
				r.Header.Del("X-NPS-Http-Only") // 删除该头部
			}

			// 保存 Connection 和 Upgrade 头信息
			upgradeHeader := r.Header.Get("Upgrade")
			connectionHeader := r.Header.Get("Connection")

			// 修改 Host 和其他头信息
			//logs.Debug("websocket %s, isHttpOnlyRequest %s", r.RemoteAddr, isHttpOnlyRequest)
			common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, r.RemoteAddr, isHttpOnlyRequest)

			// 恢复 Connection 和 Upgrade 头信息
			if upgradeHeader != "" {
				r.Header.Set("Upgrade", upgradeHeader)
			}
			if connectionHeader != "" {
				r.Header.Set("Connection", connectionHeader)
			}
		},
		Transport: &http.Transport{
			ResponseHeaderTimeout: rp.responseHeaderTimeout,
			DisableKeepAlives:     true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var (
					host       *file.Host
					target     net.Conn
					err        error
					connClient io.ReadWriteCloser
					targetAddr string
					lk         *conn.Link
				)

				r := ctx.Value("req").(*http.Request)
				host = ctx.Value("host").(*file.Host)
				targetAddr = ctx.Value("target").(string)

				lk = conn.NewLink("http", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, s.allowLocalProxy && host.Target.LocalProxy)
				if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
					logs.Notice("connect to target %s error %s", lk.Host, err)
					return nil, NewHTTPError(http.StatusBadGateway, "Cannot connect to the server")
				}
				connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
				// 发送 Proxy Protocol 头部
				//if host.Target.ProxyProtocol != 0 {
				//	clientAddr, _ := GetClientAddr(r)
				//	proxyHeader := conn.BuildProxyProtocolHeaderByAddr(clientAddr, clientAddr, host.Target.ProxyProtocol)
				//	if proxyHeader != nil {
				//		logs.Debug("Sending Proxy Protocol v%d header to backend: %v", host.Target.ProxyProtocol, proxyHeader)
				//		connClient.Write(proxyHeader)
				//	}
				//}
				return &flowConn{
					ReadWriteCloser: connClient,
					fakeAddr:        local,
					host:            host,
				}, nil
			},
		},
		ErrorHandler: func(rw http.ResponseWriter, req *http.Request, err error) {
			logs.Warn("do http proxy request error: %v", err)
			rw.WriteHeader(http.StatusNotFound)
		},
	})
	proxy.WebSocketDialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		var (
			host       *file.Host
			target     net.Conn
			err        error
			connClient io.ReadWriteCloser
			targetAddr string
			lk         *conn.Link
		)
		r := ctx.Value("req").(*http.Request)
		host = ctx.Value("host").(*file.Host)
		targetAddr = ctx.Value("target").(string)

		lk = conn.NewLink("tcp", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, r.RemoteAddr, s.allowLocalProxy && host.Target.LocalProxy)
		if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
			logs.Notice("connect to target %s error %s", lk.Host, err)
			return nil, NewHTTPError(http.StatusBadGateway, "Cannot connect to the target")
		}
		connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
		// 发送 Proxy Protocol 头部
		//if host.Target.ProxyProtocol != 0 {
		//	clientAddr, _ := GetClientAddr(r)
		//	proxyHeader := conn.BuildProxyProtocolHeaderByAddr(clientAddr, clientAddr, host.Target.ProxyProtocol)
		//	if proxyHeader != nil {
		//		logs.Debug("Sending Proxy Protocol v%d header to backend: %v", host.Target.ProxyProtocol, proxyHeader)
		//		connClient.Write(proxyHeader)
		//	}
		//}
		return &flowConn{
			ReadWriteCloser: connClient,
			fakeAddr:        local,
			host:            host,
		}, nil
	}
	rp.proxy = proxy
	return rp
}

func NewHTTPError(code int, errmsg string) error {
	return &HTTPError{
		error:    errors.New(errmsg),
		HTTPCode: code,
	}
}

type ReverseProxy struct {
	*httputil.ReverseProxy
	WebSocketDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

func IsWebsocketRequest(req *http.Request) bool {
	containsHeader := func(name, value string) bool {
		items := strings.Split(req.Header.Get(name), ",")
		for _, item := range items {
			if value == strings.ToLower(strings.TrimSpace(item)) {
				return true
			}
		}
		return false
	}
	return containsHeader("Connection", "upgrade") && containsHeader("Upgrade", "websocket")
}

func NewReverseProxy(orp *httputil.ReverseProxy) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy:         orp,
		WebSocketDialContext: nil,
	}
	rp.ErrorHandler = rp.errHandler
	return rp
}

func (p *ReverseProxy) errHandler(rw http.ResponseWriter, r *http.Request, e error) {
	if e == io.EOF {
		rw.WriteHeader(521)
		//rw.Write(getWaitingPageContent())
	} else {
		if httperr, ok := e.(*HTTPError); ok {
			rw.WriteHeader(httperr.HTTPCode)
		} else {
			rw.WriteHeader(http.StatusNotFound)
		}
		rw.Write([]byte("error: " + e.Error()))
	}
}

func (p *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request, host *file.Host) {
	if IsWebsocketRequest(req) {
		p.serveWebSocket(rw, req, host)
	}
}

func (p *ReverseProxy) serveWebSocket(rw http.ResponseWriter, req *http.Request, host *file.Host) {
	if p.WebSocketDialContext == nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	targetConn, err := p.WebSocketDialContext(req.Context(), "tcp", "")
	if err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		return
	}
	defer targetConn.Close()

	// 确保使用 HTTP/1.1
	req.Proto = "HTTP/1.1"
	req.ProtoMajor = 1
	req.ProtoMinor = 1

	// 设置 Host 头信息
	if host.HostChange != "" {
		req.Host = host.HostChange
		req.Header.Set("Host", host.HostChange)
	}

	// 确保请求头中包含正确的 Connection 和 Upgrade 头信息
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	hijacker, ok := rw.(http.Hijacker)
	if !ok {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	clientConn, bufrw, errHijack := hijacker.Hijack()
	if errHijack != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// 将客户端的请求写入目标服务器
	err = req.Write(targetConn)
	if err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	// 从目标服务器读取响应，并确保响应成功
	targetReader := bufio.NewReader(targetConn)
	resp, err := http.ReadResponse(targetReader, req)
	if err != nil || resp.StatusCode != http.StatusSwitchingProtocols {
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	// 将响应写回客户端
	err = resp.Write(bufrw)
	if err != nil {
		rw.WriteHeader(http.StatusBadGateway)
		return
	}

	// 刷新缓冲区
	err = bufrw.Flush()
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	// WebSocket 握手完成，开始传输数据
	Join(clientConn, targetConn, host)
}

func Join(c1 io.ReadWriteCloser, c2 io.ReadWriteCloser, host *file.Host) (inCount int64, outCount int64) {
	var wait sync.WaitGroup
	pipe := func(to io.ReadWriteCloser, from io.ReadWriteCloser, count *int64) {
		defer to.Close()
		defer from.Close()
		defer wait.Done()
		goroutine.CopyBuffer(to, from, host.Client.Flow, nil, "")
		//*count, _ = io.Copy(to, from)
	}

	wait.Add(2)

	go pipe(c1, c2, &inCount)
	go pipe(c2, c1, &outCount)
	wait.Wait()
	return
}
