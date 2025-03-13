package controllers

import (
	"encoding/hex"
	"time"

	"ehang.io/nps/lib/crypt"
	"github.com/beego/beego/v2/server/web"
)

type AuthController struct {
	web.Controller
}

func (s *AuthController) GetAuthKey() {
	m := make(map[string]interface{})
	defer func() {
		s.Data["json"] = m
		s.ServeJSON()
	}()
	if cryptKey, _ := web.AppConfig.String("auth_crypt_key"); len(cryptKey) != 16 {
		m["status"] = 0
		return
	} else {
		authKey, _ := web.AppConfig.String("auth_key")
		b, err := crypt.AesEncrypt([]byte(authKey), []byte(cryptKey))
		if err != nil {
			m["status"] = 0
			return
		}
		m["status"] = 1
		m["crypt_auth_key"] = hex.EncodeToString(b)
		m["crypt_type"] = "aes cbc"
		return
	}
}

func (s *AuthController) GetTime() {
	m := make(map[string]interface{})
	m["time"] = time.Now().Unix()
	s.Data["json"] = m
	s.ServeJSON()
}
