package dhcp

import (
	"fmt"
	"log"
)

const (
	levelExtraDebug = 2
	levelDebug      = 1
	levelInfo       = 0
)

type RLogger interface {
	Errorf(err error, format string, args ...interface{})
	Infof(msg string, args ...interface{})
	Debugf(format string, args ...interface{})
	WithName(string) RLogger
}

type GenericLogger struct{}

func (s *GenericLogger) Errorf(err error, format string, args ...interface{}) {
	log.Printf("ERROR: %s\n%s", err, fmt.Sprintf(format, args...))
}

func (s *GenericLogger) Infof(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (s *GenericLogger) Debugf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func (s *GenericLogger) WithName(_ string) RLogger {
	return &GenericLogger{}
}
