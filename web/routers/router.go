package routers

import (
	"net/http"

	"ehang.io/nps/web/controllers"
	"github.com/beego/beego/v2/server/web"
)

func Init() {
	// 配置 404 错误处理
	web.ErrorHandler("404", func(rw http.ResponseWriter, r *http.Request) {
		rw.WriteHeader(http.StatusNotFound)
		// 返回空内容
	})

	web_base_url, _ := web.AppConfig.String("web_base_url")
	if len(web_base_url) > 0 {
		ns := web.NewNamespace(web_base_url,
			web.NSRouter("/", &controllers.IndexController{}, "*:Index"),
			web.NSAutoRouter(&controllers.IndexController{}),
			web.NSAutoRouter(&controllers.LoginController{}),
			web.NSAutoRouter(&controllers.ClientController{}),
			web.NSAutoRouter(&controllers.AuthController{}),
			web.NSAutoRouter(&controllers.GlobalController{}),
		)
		web.AddNamespace(ns)
	} else {
		web.Router("/", &controllers.IndexController{}, "*:Index")
		web.AutoRouter(&controllers.IndexController{})
		web.AutoRouter(&controllers.LoginController{})
		web.AutoRouter(&controllers.ClientController{})
		web.AutoRouter(&controllers.AuthController{})
		web.AutoRouter(&controllers.GlobalController{})

	}
}
