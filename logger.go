package pool

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"runtime"
)

var (
	Error  func(args ...interface{})
	Errorf func(format string, args ...interface{})
	Warn   func(args ...interface{})
	Warnf  func(format string, args ...interface{})
	Panic  func(args ...interface{})
	Panicf func(format string, args ...interface{})

	Info     func(args ...interface{})
	Infof    func(format string, args ...interface{})
	APIInfo  func(args ...interface{})
	APIInfof func(format string, args ...interface{})
	GINInfo  func(args ...interface{})
	GINInfof func(format string, args ...interface{})
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	Info = log.Info
	Infof = log.Infof
	APIInfo = log.WithFields(log.Fields{
		"type": "API"}).Info
	APIInfof = log.WithFields(log.Fields{
		"type": "API"}).Infof
	GINInfo = log.WithFields(log.Fields{
		"type": "GIN"}).Info
	GINInfof = log.WithFields(log.Fields{
		"type": "GIN"}).Infof

	errorLog := log.New()
	errorLog.SetLevel(log.WarnLevel)
	errorLog.SetFormatter(&log.JSONFormatter{})
	errorLog.SetReportCaller(true)
	Error = errorLog.Error
	Errorf = errorLog.Errorf
	Warn = errorLog.Warn
	Warnf = errorLog.Warnf
	Panic = errorLog.Panic
	Panicf = errorLog.Panicf
}

// ErrorStack 用于打印
func ErrorStack() {
	var buf [4096]byte
	n := runtime.Stack(buf[:], false)
	fmt.Println(string(buf[:n]))
}

// GinLog 打印gin请求的日志
func GinLog(method, requestURI, body string) {
	log.WithFields(log.Fields{
		"type":   "GIN",
		"method": method,
		"uri":    requestURI,
		"body":   body}).Info()
}
