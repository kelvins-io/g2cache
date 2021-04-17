package g2cache

import (
	"log"
)

// 外部调用者可实现此日志接口用于将日志导出
type LoggerInterface interface {
	LogInf(f string, s ...interface{})
	LogInfo(s ...interface{})
	LogDebug(s ...interface{})
	LogErr(s ...interface{})
}

func LogInf(f string, s ...interface{}) {
	Logger.LogInf(f, s)
}

func LogInfo(s ...interface{}) {
	Logger.LogInfo(s)
}

func LogDebug(s ...interface{}) {
	Logger.LogDebug(s)
}

func LogErr(s ...interface{}) {
	Logger.LogErr(s)
}

type sysLogger struct {}

var (
	Logger LoggerInterface = &sysLogger{}
)

func init()  {
	log.SetPrefix("[\u001B[32mg2cache\u001B[0m] ")
}

func (l *sysLogger) LogInf(f string, s ...interface{}) {
	log.Printf("[info] "+f, s)
}

func (l *sysLogger) LogInfo(s ...interface{}) {
	log.Println("[info] ", s)
}

func (l *sysLogger) LogDebug(s ...interface{}) {
	log.Println("[debug] ", s)
}

func (l *sysLogger) LogErr(s ...interface{}) {
	log.Println("[err] ", s)
}
