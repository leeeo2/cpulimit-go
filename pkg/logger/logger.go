package logger

import "fmt"

type Logger interface {
	Printf(format string, args ...interface{})
	Tracef(format string, args ...interface{})
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

type DefaultLogger struct{}

var defaultLogger *DefaultLogger

func Default() *DefaultLogger {
	if defaultLogger == nil {
		defaultLogger = &DefaultLogger{}
	}
	return defaultLogger
}

func (l DefaultLogger) Printf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (l DefaultLogger) Tracef(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (l DefaultLogger) Debugf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (l DefaultLogger) Infof(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (l DefaultLogger) Warnf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (l DefaultLogger) Errorf(format string, args ...interface{}) {
	fmt.Println(fmt.Sprintf(format, args...))
}
