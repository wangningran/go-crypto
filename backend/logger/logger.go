package logger

import "go.uber.org/zap"

var Log *zap.SugaredLogger

func init() {
	l, _ := zap.NewDevelopment()
	Log = l.Sugar()
}
