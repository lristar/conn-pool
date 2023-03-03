package pool

import (
	log "github.com/sirupsen/logrus"
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
	log.SetReportCaller(true)
	log.SetFormatter(&log.JSONFormatter{TimestampFormat: "2006-01-02 15:04:05"})
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
