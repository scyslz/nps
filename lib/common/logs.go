package common

import (
	"github.com/beego/beego/v2/core/logs"
)

const MaxMsgLen = 5000

var logMsgs string

func init() {
	logs.Register("store", func() logs.Logger {
		return new(StoreMsg)
	})
}

func GetLogMsg() string {
	return logMsgs
}

type StoreMsg struct {
}

func (lg *StoreMsg) Init(config string) error {
	return nil
}

func (lg *StoreMsg) WriteMsg(lm *logs.LogMsg) error {
	when := lm.When.Format("2006-01-02 15:04:05")
	msg := lm.OldStyleFormat()
	m := when + " " + msg + "\r\n"

	if len(logMsgs) > MaxMsgLen {
		start := MaxMsgLen - len(m)
		if start <= 0 {
			start = MaxMsgLen
		}
		logMsgs = logMsgs[start:]
	}
	logMsgs += m
	return nil
}

func (lg *StoreMsg) Destroy() {
	return
}

func (lg *StoreMsg) Flush() {
	return
}

func (lg *StoreMsg) SetFormatter(f logs.LogFormatter) {
	return
}
