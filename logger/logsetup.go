package logger

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

//--------------------------------------------------------------------------
type LogrusFileHook struct {
	file      *os.File
	flag      int
	chmod     os.FileMode
	formatter *log.TextFormatter
}

//--------------------------------------------------------------------------
func NewLogrusFileHook(file string, flag int, chmod os.FileMode) (*LogrusFileHook, error) {
	plainFormatter := &log.TextFormatter{DisableColors: true}
	// todo: are wa leaking resources by not closing the file??
	logFile, err := os.OpenFile(file, flag, chmod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}

	return &LogrusFileHook{logFile, flag, chmod, plainFormatter}, err
}

//--------------------------------------------------------------------------
// Fire event for LogrusFileHook
func (hook *LogrusFileHook) Fire(entry *log.Entry) error {

	plainformat, err := hook.formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
		return err
	}
	line := string(plainformat)
	_, err = hook.file.WriteString(line)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
		return err
	}

	return nil
}

//--------------------------------------------------------------------------
// Levels entry
func (hook *LogrusFileHook) Levels() []log.Level {
	return log.AllLevels
}

//--------------------------------------------------------------------------
func InitLogging(filename string) error {
	// todo: rotate log files
	// todo: somehow differentiate between debug/release programmatically

	//log.SetLevel(logrus.WarnLevel)
	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true, FullTimestamp: true})
	log.SetLevel(log.TraceLevel)
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)

	filehook, err := NewLogrusFileHook(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err == nil {
		log.AddHook(filehook)
	} else {
		fmt.Print("failed creating logfile hook: ", err)
		return err
	}
	return nil
}
