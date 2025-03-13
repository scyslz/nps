package connection

import (
	"net"
	"os"
	"strconv"

	"ehang.io/nps/lib/pmux"
	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
)

var pMux *pmux.PortMux
var bridgePort string
var httpsPort string
var httpPort string
var webPort string
var webHost string
var bridgeIp string
var httpProxyIp string
var webIp string

func InitConnectionService() {
	bridgePort, _ = web.AppConfig.String("bridge_port")
	httpsPort, _ = web.AppConfig.String("https_proxy_port")
	httpPort, _ = web.AppConfig.String("http_proxy_port")
	webPort, _ = web.AppConfig.String("web_port")
	webHost, _ = web.AppConfig.String("web_host")
	bridgeIp, _ = web.AppConfig.String("bridge_ip")
	httpProxyIp, _ = web.AppConfig.String("http_proxy_ip")
	webIp, _ = web.AppConfig.String("web_ip")

	if httpPort == bridgePort || httpsPort == bridgePort || webPort == bridgePort {
		port, err := strconv.Atoi(bridgePort)
		if err != nil {
			logs.Error(err)
			os.Exit(0)
		}
		pMux = pmux.NewPortMux(port, webHost)
	}
}

func GetBridgeListener(tp string) (net.Listener, error) {
	logs.Info("server start, the bridge type is %s, the bridge port is %s", tp, bridgePort)
	var p int
	var err error
	if p, err = strconv.Atoi(bridgePort); err != nil {
		return nil, err
	}
	if pMux != nil {
		return pMux.GetClientListener(), nil
	}
	return net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(bridgeIp), p, ""})
}

func GetHttpListener() (net.Listener, error) {
	if pMux != nil && httpPort == bridgePort {
		logs.Info("start http listener, port is", bridgePort)
		return pMux.GetHttpListener(), nil
	}
	logs.Info("start http listener, port is", httpPort)
	return getTcpListener(httpProxyIp, httpPort)
}

func GetHttpsListener() (net.Listener, error) {
	if pMux != nil && httpsPort == bridgePort {
		logs.Info("start https listener, port is", bridgePort)
		return pMux.GetHttpsListener(), nil
	}
	logs.Info("start https listener, port is", httpsPort)
	return getTcpListener(httpProxyIp, httpsPort)
}

func GetWebManagerListener() (net.Listener, error) {
	if pMux != nil && webPort == bridgePort {
		logs.Info("Web management start, access port is", bridgePort)
		return pMux.GetManagerListener(), nil
	}
	logs.Info("web management start, access port is", webPort)
	return getTcpListener(webIp, webPort)
}

func getTcpListener(ip, p string) (net.Listener, error) {
	port, err := strconv.Atoi(p)
	if err != nil {
		logs.Error(err)
		os.Exit(0)
	}
	if ip == "" {
		ip = "0.0.0.0"
	}
	return net.ListenTCP("tcp", &net.TCPAddr{net.ParseIP(ip), port, ""})
}
