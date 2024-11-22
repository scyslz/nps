package proxy

import (
	"errors"
	"net"
	"net/http"
	"sort"
	"sync"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type Service interface {
	Start() error
	Close() error
}

type NetBridge interface {
	SendLinkInfo(clientId int, link *conn.Link, t *file.Tunnel) (target net.Conn, err error)
}

//BaseServer struct
type BaseServer struct {
	id              int
	bridge          NetBridge
	task            *file.Tunnel
	errorContent    []byte
	allowLocalProxy bool
	sync.Mutex
}

func NewBaseServer(bridge *bridge.Bridge, task *file.Tunnel) *BaseServer {
	allowLocalProxy, _ := beego.AppConfig.Bool("allow_local_proxy")
	return &BaseServer{
		bridge:          bridge,
		task:            task,
		errorContent:    nil,
		allowLocalProxy: allowLocalProxy,
		Mutex:           sync.Mutex{},
	}
}

//add the flow
func (s *BaseServer) FlowAdd(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.task.Flow.ExportFlow += out
	s.task.Flow.InletFlow += in
}

//change the flow
func (s *BaseServer) FlowAddHost(host *file.Host, in, out int64) {
	s.Lock()
	defer s.Unlock()
	host.Flow.ExportFlow += out
	host.Flow.InletFlow += in
}

//write fail bytes to the connection
func (s *BaseServer) writeConnFail(c net.Conn) {
	c.Write([]byte(common.ConnectionFailBytes))
	c.Write(s.errorContent)
}

//auth check
func (s *BaseServer) auth(r *http.Request, c *conn.Conn, u, p string, task *file.Tunnel) error {
	var accountMap map[string]string
	if task.MultiAccount == nil {
		accountMap = nil
	} else {
		accountMap = task.MultiAccount.AccountMap
	}
	if !common.CheckAuth(r, u, p, accountMap) {
		c.Write([]byte(common.UnauthorizedBytes))
		c.Close()
		return errors.New("401 Unauthorized")
	}
	return nil
}

//check flow limit of the client ,and decrease the allow num of client
func (s *BaseServer) CheckFlowAndConnNum(client *file.Client) error {
	if client.Flow.FlowLimit > 0 && (client.Flow.FlowLimit<<20) < (client.Flow.ExportFlow+client.Flow.InletFlow) {
		return errors.New("Traffic exceeded")
	}
	if !client.GetConn() {
		return errors.New("Connections exceed the current client limit")
	}
	return nil
}

func in(target string, str_array []string) bool {
	sort.Strings(str_array)
	index := sort.SearchStrings(str_array, target)
	if index < len(str_array) && str_array[index] == target {
		return true
	}
	return false
}

// 处理客户端连接
func (s *BaseServer) DealClient(c *conn.Conn, client *file.Client, addr string,
	rb []byte, tp string, f func(), flow *file.Flow, proxyProtocol int, localProxy bool, task *file.Tunnel) error {

	// 判断访问地址是否在全局黑名单内
	if IsGlobalBlackIp(c.RemoteAddr().String()) {
		c.Close()
		return nil
	}

	// 判断访问地址是否在黑名单内
	if common.IsBlackIp(c.RemoteAddr().String(), client.VerifyKey, client.BlackIpList) {
		c.Close()
		return nil
	}

	// 创建连接链接
	link := conn.NewLink(tp, addr, client.Cnf.Crypt, client.Cnf.Compress, c.Conn.RemoteAddr().String(), s.allowLocalProxy && localProxy)

	// 获取目标连接
	target, err := s.bridge.SendLinkInfo(client.Id, link, s.task)
	if err != nil {
		logs.Warn("get connection from client id %d  error %s", client.Id, err.Error())
		c.Close()
		return err
	}

	// 发送 Proxy Protocol 头部
	if err := conn.SendProxyProtocolHeader(target, c, proxyProtocol); err != nil {
		c.Close()
		return err
	}

	// 执行回调函数
	if f != nil {
		f()
	}

	// 开始数据转发
	conn.CopyWaitGroup(target, c.Conn, link.Crypt, link.Compress, client.Rate, flow, true, rb, task)
	return nil
}

// 判断访问地址是否在全局黑名单内
func IsGlobalBlackIp(ipPort string) bool {
	// 判断访问地址是否在全局黑名单内
	global := file.GetDb().GetGlobal()
	if global != nil {
		ip := common.GetIpByAddr(ipPort)
		if in(ip, global.BlackIpList) {
			logs.Error("IP地址[" + ip + "]在全局黑名单列表内")
			return true
		}
	}

	return false
}
