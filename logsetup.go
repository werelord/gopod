package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// LogrusFileHook
type LogrusFileHook struct {
	file      *os.File
	flag      int
	chmod     os.FileMode
	formatter *logrus.TextFormatter
}

// NewLogrusFileHook shit
func NewLogrusFileHook(file string, flag int, chmod os.FileMode) (*LogrusFileHook, error) {
	plainFormatter := &logrus.TextFormatter{DisableColors: true}
	logFile, err := os.OpenFile(file, flag, chmod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}

	return &LogrusFileHook{logFile, flag, chmod, plainFormatter}, err
}

// Fire event for LogrusFileHook
func (hook *LogrusFileHook) Fire(entry *logrus.Entry) error {

	plainformat, err := hook.formatter.Format(entry)
	line := string(plainformat)
	_, err = hook.file.WriteString(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
		return err
	}

	return nil
}

// Levels entry
func (hook *LogrusFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

var log *logrus.Logger

func initLogging(filename string) {
	// todo: somehow differentiate between debug/release programmatically
	//log.SetLevel(logrus.WarnLevel)

	log = logrus.New()

	//log.Formatter = new(logrus.JSONFormatter)
	log.SetLevel(logrus.TraceLevel)
	log.Formatter = &logrus.TextFormatter{ForceColors: true, FullTimestamp: true}
	log.Level = logrus.TraceLevel
	log.Out = os.Stdout
	log.SetReportCaller(true)

	filehook, err := NewLogrusFileHook("./"+filename, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err == nil {
		log.AddHook(filehook)
	}

	// file, err := os.OpenFile("./logrus.log", os.O_CREATE|os.O_WRONLY, 0666)
	// if err == nil {
	// 	log.Out = file
	// } else {
	// 	log.Info("Failed to log to file, using default stderr")
	// }

	// log.SetOutput(os.Stdout)

	// log.SetReportCaller(true)
	// log.SetLevel(logrus.DebugLevel)

	// file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY, 0666)
	// if err == nil {
	// 	log.Out = file
	// } else {
	// 	log.Info("Failed to log to file, using default stderr")
	// }
}
