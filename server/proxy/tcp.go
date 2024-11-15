package proxy

import (
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"encoding/binary"
	"strconv"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server/connection"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type TunnelModeServer struct {
	BaseServer
	process  process
	listener net.Listener
	stopChan chan struct{} // 新增停止通道
	activeConnections map[net.Conn]struct{} // 新增连接池
}

// tcp|http|host
func NewTunnelModeServer(process process, bridge NetBridge, task *file.Tunnel) *TunnelModeServer {
	s := new(TunnelModeServer)
	s.bridge = bridge
	s.process = process
	s.task = task
	s.stopChan = make(chan struct{}) // 初始化停止通道
	s.activeConnections = make(map[net.Conn]struct{}) // 初始化连接池
	return s
}

// 开始
func (s *TunnelModeServer) Start() error {
	return conn.NewTcpListenerAndProcess(s.task.ServerIp+":"+strconv.Itoa(s.task.Port), func(c net.Conn) {
		// 将新连接加入到连接池中
		s.activeConnections[c] = struct{}{}
		defer func() {
			// 确保连接关闭时从连接池中移除
			delete(s.activeConnections, c)
			if c != nil {
				c.Close()
			}
		}()
		select {
		case <-s.stopChan: // 如果接收到停止信号，立即关闭连接
			logs.Info("Connection closed due to configuration change")
			c.Close()
			return
		default:
			if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
				logs.Warn("client id %d, task id %d, error %s, when tcp connection", s.task.Client.Id, s.task.Id, err.Error())
				c.Close()
				return
			}

			logs.Trace("new tcp connection, local port %d, client %d, remote address %s", s.task.Port, s.task.Client.Id, c.RemoteAddr())

			// 如果启用了 Proxy Protocol，构造并发送 Proxy Protocol 头部
			if s.task.ProxyProtocol == 1 {
				// 生成并发送 Proxy Protocol v1 头部
				proxyHeader := buildProxyProtocolV1Header(c)
				logs.Debug("Sending Proxy Protocol v1 header: %s", proxyHeader)
				_, err := c.Write([]byte(proxyHeader))
				if err != nil {
					logs.Error("Failed to send Proxy Protocol v1 header:", err)
					c.Close()
					return
				}
			} else if s.task.ProxyProtocol == 2 {
				// 生成并发送 Proxy Protocol v2 头部
				proxyHeader := buildProxyProtocolV2Header(c)
				logs.Debug("Sending Proxy Protocol v2 header: %v", proxyHeader)
				_, err := c.Write(proxyHeader)
				if err != nil {
					logs.Error("Failed to send Proxy Protocol v2 header:", err)
					c.Close()
					return
				}
			}

			err := s.process(conn.NewConn(c), s)
			if err == nil {
				s.task.Client.AddConn()
			}
		}
	}, &s.listener)
}

// Close 停止服务器并关闭所有连接
func (s *TunnelModeServer) Close() error {
	// 发送停止信号，通知所有连接断开
	close(s.stopChan)

	// 关闭所有活跃连接
	for conn := range s.activeConnections {
		if conn != nil {
			conn.Close()
		}
	}

	// 关闭监听器
	return s.listener.Close()
}

// web管理方式
type WebServer struct {
	BaseServer
}

// 开始
func (s *WebServer) Start() error {
	p, _ := beego.AppConfig.Int("web_port")
	if p == 0 {
		stop := make(chan struct{})
		<-stop
	}
	beego.BConfig.WebConfig.Session.SessionOn = true
	beego.SetStaticPath(beego.AppConfig.String("web_base_url")+"/static", filepath.Join(common.GetRunPath(), "web", "static"))
	beego.SetViewsPath(filepath.Join(common.GetRunPath(), "web", "views"))
	err := errors.New("Web management startup failure ")
	var l net.Listener
	if l, err = connection.GetWebManagerListener(); err == nil {
		beego.InitBeforeHTTPRun()
		if beego.AppConfig.String("web_open_ssl") == "true" {
			keyPath := beego.AppConfig.String("web_key_file")
			certPath := beego.AppConfig.String("web_cert_file")
			err = http.ServeTLS(l, beego.BeeApp.Handlers, certPath, keyPath)
		} else {
			err = http.Serve(l, beego.BeeApp.Handlers)
		}
	} else {
		logs.Error(err)
	}
	return err
}

func (s *WebServer) Close() error {
	return nil
}

// new
func NewWebServer(bridge *bridge.Bridge) *WebServer {
	s := new(WebServer)
	s.bridge = bridge
	return s
}

type process func(c *conn.Conn, s *TunnelModeServer) error

// tcp proxy
func ProcessTunnel(c *conn.Conn, s *TunnelModeServer) error {
	targetAddr, err := s.task.Target.GetRandomTarget()
	if err != nil {
		c.Close()
		logs.Warn("tcp port %d ,client id %d,task id %d connect error %s", s.task.Port, s.task.Client.Id, s.task.Id, err.Error())
		return err
	}

	return s.DealClient(c, s.task.Client, targetAddr, nil, common.CONN_TCP, nil, s.task.Client.Flow, s.task.Target.LocalProxy, s.task)
}

// http proxy
func ProcessHttp(c *conn.Conn, s *TunnelModeServer) error {
	_, addr, rb, err, r := c.GetHost()
	if err != nil {
		c.Close()
		logs.Info(err)
		return err
	}

	if err := s.auth(r, c, s.task.Client.Cnf.U, s.task.Client.Cnf.P, s.task); err != nil {
		return err
	}

	if r.Method == "CONNECT" {
		c.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
		rb = nil
	}

	return s.DealClient(c, s.task.Client, addr, rb, common.CONN_TCP, nil, s.task.Client.Flow, s.task.Target.LocalProxy, nil)
}

// 构造 Proxy Protocol v1 头部
func buildProxyProtocolV1Header(c *conn.Conn) string {
	clientAddr := c.RemoteAddr().(*net.TCPAddr)
	targetAddr := c.LocalAddr().(*net.TCPAddr)

	var protocol, clientIP, targetIP string
	if clientAddr.IP.To4() != nil {
		protocol = "TCP4"
		clientIP = clientAddr.IP.String()
		targetIP = targetAddr.IP.String()
	} else {
		protocol = "TCP6"
		clientIP = "[" + clientAddr.IP.String() + "]"
		targetIP = "[" + targetAddr.IP.String() + "]"
	}

	header := "PROXY " + protocol + " " + clientIP + " " + targetIP + " " +
		strconv.Itoa(clientAddr.Port) + " " + strconv.Itoa(targetAddr.Port) + "\r\n"
	return header
}

// 构造 Proxy Protocol v2 头部
func buildProxyProtocolV2Header(c *conn.Conn) []byte {
	clientAddr := c.RemoteAddr().(*net.TCPAddr)
	targetAddr := c.LocalAddr().(*net.TCPAddr)

	var header []byte
	if clientAddr.IP.To4() != nil {
		// IPv4
		header = make([]byte, 16+12) // v2 头部长度为 16 字节固定头 + 12 字节的 IPv4 地址信息
		copy(header[0:12], []byte{0x0d, 0x0a, 0x0d, 0x0a, 0x00, 0x0d, 0x0a, 0x51, 0x55, 0x49, 0x54, 0x0a})
		header[12] = 0x21 // Proxy Protocol v2 的版本和命令
		header[13] = 0x11 // 地址族和传输协议 (TCP over IPv4)
		binary.BigEndian.PutUint16(header[14:16], 12) // 地址信息长度
		copy(header[16:20], clientAddr.IP.To4())
		copy(header[20:24], targetAddr.IP.To4())
		binary.BigEndian.PutUint16(header[24:26], uint16(clientAddr.Port))
		binary.BigEndian.PutUint16(header[26:28], uint16(targetAddr.Port))
	} else {
		// IPv6
		header = make([]byte, 16+36) // v2 头部长度为 16 字节固定头 + 36 字节的 IPv6 地址信息
		copy(header[0:12], []byte{0x0d, 0x0a, 0x0d, 0x0a, 0x00, 0x0d, 0x0a, 0x51, 0x55, 0x49, 0x54, 0x0a})
		header[12] = 0x21 // Proxy Protocol v2 的版本和命令
		header[13] = 0x21 // 地址族和传输协议 (TCP over IPv6)
		binary.BigEndian.PutUint16(header[14:16], 36) // 地址信息长度
		copy(header[16:32], clientAddr.IP.To16())
		copy(header[32:48], targetAddr.IP.To16())
		binary.BigEndian.PutUint16(header[48:50], uint16(clientAddr.Port))
		binary.BigEndian.PutUint16(header[50:52], uint16(targetAddr.Port))
	}

	return header
}
