package app

import "log"

// Logger абстрагирует вывод — можно подставить log.Printf или gin-логгер
type Logger interface {
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Debugf(format string, args ...interface{})
}

// ConsoleLogger — обёртка над стандартным log
type ConsoleLogger struct{}

func (l *ConsoleLogger) Infof(format string, args ...interface{}) { log.Printf(format, args...) }
func (l *ConsoleLogger) Errorf(format string, args ...interface{}) {
	log.Printf("❌ "+format, args...)
}
func (l *ConsoleLogger) Debugf(format string, args ...interface{}) {
	log.Printf("🔍 "+format, args...)
}
