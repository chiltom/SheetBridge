package logger

import (
	"io"
	stdlog "log" // Alias to avoid conflict with Logger type
	"os"
)

// Logger represents the detailed application logger
type Logger struct {
	infoLog  *stdlog.Logger
	errorLog *stdlog.Logger
}

// New returns a new detailed application logger
func New(infoHandle io.Writer, errorHandle io.Writer) *Logger {
	return &Logger{
		infoLog:  stdlog.New(infoHandle, "INFO\t", stdlog.Ldate|stdlog.Ltime),
		errorLog: stdlog.New(errorHandle, "ERROR\t", stdlog.Ldate|stdlog.Ltime|stdlog.Lshortfile),
	}
}

// NewStdLogger creates a logger that writes to os.Stdout and os.Stderr
func NewStdLogger() *Logger {
	return New(os.Stdout, os.Stderr)
}

// Info logs a standard informational message
func (l *Logger) Info(message string) {
	l.infoLog.Println(message)
}

// Infof logs an informational message with the supplied arguments
func (l *Logger) Infof(format string, v ...any) {
	l.infoLog.Printf(format, v...)
}

// Error logs a standard error message
func (l *Logger) Error(err error) {
	l.errorLog.Println(err.Error())
}

// Errorf logs an error message with the supplied arguments
func (l *Logger) Errorf(format string, v ...any) {
	l.errorLog.Printf(format, v...)
}

// ErrorOutput returns the output destination of the Logger
func (l *Logger) ErrorOutput() io.Writer {
	return l.errorLog.Writer()
}
