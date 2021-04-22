package g2cache

import (
	"log"
)

// 外部调用者可实现此日志接口用于将日志导出
type LoggerInterface interface {
	LogInfoF(f string, s ...interface{})
	LogInfo(s ...interface{})
	LogDebug(s ...interface{})
	LogDebugF(f string, s ...interface{})
	LogErr(s ...interface{})
	LogErrF(f string, s ...interface{})
}

func LogInfoF(f string, s ...interface{}) {
	Logger.LogInfoF(f, s)
}

func LogInfo(s ...interface{}) {
	Logger.LogInfo(s)
}

func LogDebug(s ...interface{}) {
	Logger.LogDebug(s)
}

func LogDebugF(f string, s ...interface{}) {
	Logger.LogDebugF(f, s)
}

func LogErr(s ...interface{}) {
	Logger.LogErr(s)
}

func LogErrF(f string, s ...interface{}) {
	Logger.LogErrF(f, s)
}

type sysLogger struct{}

var (
	Logger LoggerInterface = &sysLogger{}
)

func (l *sysLogger) LogInfo(s ...interface{}) {
	log.Println("[\u001B[32mg2cache\u001B[0m] [\u001B[32minfo\u001B[0m] ", s)
}

func (l *sysLogger) LogInfoF(f string, s ...interface{}) {
	log.Printf("[\u001B[32mg2cache\u001B[0m] [\u001B[32minfo\u001B[0m] "+f, s)
}

func (l *sysLogger) LogDebug(s ...interface{}) {
	log.Println("[\u001B[32mg2cache\u001B[0m] [\u001B[33mdebug\u001B[0m] ", s)
}

func (l *sysLogger) LogDebugF(f string, s ...interface{}) {
	log.Printf("[\u001B[32mg2cache\u001B[0m] [\u001B[33mdebug\u001B[0m] "+f, s)
}

func (l *sysLogger) LogErr(s ...interface{}) {
	log.Println("[\u001B[32mg2cache\u001B[0m] [\u001B[31merr\u001B[0m] ", s)
}

func (l *sysLogger) LogErrF(f string, s ...interface{}) {
	log.Printf("[\u001B[32mg2cache\u001B[0m] [\u001B[31merr\u001B[0m] "+f, s)
}
