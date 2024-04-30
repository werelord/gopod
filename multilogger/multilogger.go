package multilogger

import (
	"io"

	charmlog "github.com/charmbracelet/log"
)

type Logger interface {
	Debug(msg any, keyvals ...any)
	Info(msg any, keyvals ...any)
	Warn(msg any, keyvals ...any)
	Error(msg any, keyvals ...any)
	// Fatal(msg any, keyvals ...any)
	Print(msg any, keyvals ...any)
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
	// Fatalf(format string, args ...any)
	Printf(format string, args ...any)
	With(keyvals ...any) Logger
}

type multilog struct {
	console *charmlog.Logger
	logList []*charmlog.Logger
}

var def = multilog{console: charmlog.Default(), logList: make([]*charmlog.Logger, 0)}

func SetConsoleWithOptions(w io.Writer, opt charmlog.Options) { def.SetConsoleWithOptions(w, opt) }
func SetConsoleStyles(s *charmlog.Styles) { def.SetConsoleStyles(s) }
func AddWithOptions(w io.Writer, opt charmlog.Options)        { def.AddWithOptions(w, opt) }
func Debug(msg any, keyvals ...any) {
	if def.console.GetLevel() <= charmlog.DebugLevel {
		def.console.Helper()
		def.console.Debug(msg, keyvals...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.DebugLevel {
			log.Helper()
			log.Debug(msg, keyvals...)
		}
	}
}
func Info(msg any, keyvals ...any) {
	if def.console.GetLevel() <= charmlog.InfoLevel {
		def.console.Helper()
		def.console.Info(msg, keyvals...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.InfoLevel {
			log.Helper()
			log.Info(msg, keyvals...)
		}
	}
}
func Warn(msg any, keyvals ...any) {
	if def.console.GetLevel() <= charmlog.WarnLevel {
		def.console.Helper()
		def.console.Warn(msg, keyvals...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.WarnLevel {
			log.Helper()
			log.Warn(msg, keyvals...)
		}
	}
}
func Error(msg any, keyvals ...any) {
	if def.console.GetLevel() <= charmlog.ErrorLevel {
		def.console.Helper()
		def.console.Error(msg, keyvals...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.ErrorLevel {
			log.Helper()
			log.Error(msg, keyvals...)
		}
	}
}
func Print(msg any, keyvals ...any) {
	def.console.Helper()
	def.console.Print(msg, keyvals...)
	for _, log := range def.logList {
		log.Helper()
		log.Print(msg, keyvals...)
	}
}
func Debugf(format string, args ...any) {
	if def.console.GetLevel() <= charmlog.DebugLevel {
		def.console.Helper()
		def.console.Debugf(format, args...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.DebugLevel {
			log.Helper()
			log.Debugf(format, args...)
		}
	}
}
func Infof(format string, args ...any) {
	if def.console.GetLevel() <= charmlog.InfoLevel {
		def.console.Helper()
		def.console.Infof(format, args...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.InfoLevel {
			log.Helper()
			log.Infof(format, args...)
		}
	}
}
func Warnf(format string, args ...any) {
	if def.console.GetLevel() <= charmlog.WarnLevel {
		def.console.Helper()
		def.console.Warnf(format, args...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.WarnLevel {
			log.Helper()
			log.Warnf(format, args...)
		}
	}
}
func Errorf(format string, args ...any) {
	if def.console.GetLevel() <= charmlog.ErrorLevel {
		def.console.Helper()
		def.console.Errorf(format, args...)
	}
	for _, log := range def.logList {
		if log.GetLevel() <= charmlog.ErrorLevel {
			log.Helper()
			log.Errorf(format, args...)
		}
	}
}
func Printf(format string, args ...any) {
	def.console.Helper()
	def.console.Printf(format, args...)
	for _, log := range def.logList {
		log.Helper()
		log.Printf(format, args...)
	}
}
func With(keyvals ...any) Logger { return def.With(keyvals...) }

func (m *multilog) SetConsoleWithOptions(w io.Writer, opt charmlog.Options) {
	m.console = charmlog.NewWithOptions(w, opt)
}
func (m *multilog) SetConsoleStyles(style *charmlog.Styles) {
	m.console.SetStyles(style)
}
func (m *multilog) AddWithOptions(w io.Writer, opt charmlog.Options) {
	m.logList = append(m.logList, charmlog.NewWithOptions(w, opt))
}
func (m multilog) Debug(msg any, keyvals ...any) {
	if m.console.GetLevel() <= charmlog.DebugLevel {
		m.console.Helper()
		m.console.Debug(msg, keyvals...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.DebugLevel {
			log.Helper()
			log.Debug(msg, keyvals...)
		}
	}
}
func (m multilog) Info(msg any, keyvals ...any) {
	if m.console.GetLevel() <= charmlog.InfoLevel {
		m.console.Helper()
		m.console.Info(msg, keyvals...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.InfoLevel {
			log.Helper()
			log.Info(msg, keyvals...)
		}
	}
}
func (m multilog) Warn(msg any, keyvals ...any) {
	if m.console.GetLevel() <= charmlog.WarnLevel {
		m.console.Helper()
		m.console.Warn(msg, keyvals...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.WarnLevel {
			log.Helper()
			log.Warn(msg, keyvals...)
		}
	}
}
func (m multilog) Error(msg any, keyvals ...any) {
	if m.console.GetLevel() <= charmlog.ErrorLevel {
		m.console.Helper()
		m.console.Error(msg, keyvals...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.ErrorLevel {
			log.Helper()
			log.Error(msg, keyvals...)
		}
	}
}
func (m multilog) Print(msg any, keyvals ...any) {
	m.console.Helper()
	m.console.Print(msg, keyvals...)
	for _, log := range m.logList {
		log.Helper()
		log.Print(msg, keyvals...)
	}
}
func (m multilog) Debugf(format string, args ...any) {
	if m.console.GetLevel() <= charmlog.DebugLevel {
		m.console.Helper()
		m.console.Debugf(format, args...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.DebugLevel {
			log.Helper()
			log.Debugf(format, args...)
		}
	}
}
func (m multilog) Infof(format string, args ...any) {
	if m.console.GetLevel() <= charmlog.InfoLevel {
		m.console.Helper()
		m.console.Infof(format, args...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.InfoLevel {
			log.Helper()
			log.Infof(format, args...)
		}
	}
}
func (m multilog) Warnf(format string, args ...any) {
	if m.console.GetLevel() <= charmlog.WarnLevel {
		m.console.Helper()
		m.console.Warnf(format, args...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.WarnLevel {
			log.Helper()
			log.Warnf(format, args...)
		}
	}
}
func (m multilog) Errorf(format string, args ...any) {
	if m.console.GetLevel() <= charmlog.ErrorLevel {
		m.console.Helper()
		m.console.Errorf(format, args...)
	}
	for _, log := range m.logList {
		if log.GetLevel() <= charmlog.ErrorLevel {
			log.Helper()
			log.Errorf(format, args...)
		}
	}
}
func (m multilog) Printf(format string, args ...any) {
	m.console.Helper()
	m.console.Printf(format, args...)
	for _, log := range m.logList {
		log.Helper()
		log.Printf(format, args...)
	}
}
func (m multilog) With(keyvals ...any) Logger {
	var withLogger = multilog{
		m.console.With(keyvals...),
		make([]*charmlog.Logger, 0, len(m.logList)),
	}
	for _, log := range m.logList {
		withLogger.logList = append(withLogger.logList, log.With(keyvals...))
	}
	return withLogger
}
