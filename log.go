package glink

import (
	"log"
	"sync"
)

type Logger struct {
	enabled bool
}

var (
	instance *Logger
	once     sync.Once
)

// GetLogger returns the singleton instance of Logger
func GetLogger() *Logger {
	once.Do(func() {
		instance = &Logger{enabled: false} // Default: logging enabled
	})
	return instance
}

// Enable turns logging on
func (l *Logger) Enable() {
	l.enabled = true
}

// Disable turns logging off
func (l *Logger) Disable() {
	l.enabled = false
}

// Println logs a message only if logging is enabled
func (l *Logger) Println(v ...interface{}) {
	if l.enabled {
		log.Println(v...)
	}
}

func (l *Logger) Printf(format string, v ...interface{}) {
	if l.enabled {
		log.Printf(format, v...)
	}
}
