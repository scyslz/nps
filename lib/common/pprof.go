package common

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/beego/beego/v2/core/logs"
	"github.com/beego/beego/v2/server/web"
)

func InitPProfFromFile() {
	ip, _ := web.AppConfig.String("pprof_ip")
	p, _ := web.AppConfig.String("pprof_port")
	if len(ip) > 0 && len(p) > 0 && IsPort(p) {
		runPProf(ip + ":" + p)
	}
}

func InitPProfFromArg(arg string) {
	if len(arg) > 0 {
		runPProf(arg)
	}
}

func runPProf(ipPort string) {
	go func() {
		_ = http.ListenAndServe(ipPort, nil)
	}()
	logs.Info("PProf debug listen on", ipPort)
}
