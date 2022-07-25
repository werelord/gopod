package podutils

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

//--------------------------------------------------------------------------
type LogrusFileHook struct {
	file      *os.File
	flag      int
	chmod     os.FileMode
	formatter *logrus.TextFormatter
}

//--------------------------------------------------------------------------
func NewLogrusFileHook(file string, flag int, chmod os.FileMode) (*LogrusFileHook, error) {
	plainFormatter := &logrus.TextFormatter{DisableColors: true}
	logFile, err := os.OpenFile(file, flag, chmod)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}

	return &LogrusFileHook{logFile, flag, chmod, plainFormatter}, err
}

//--------------------------------------------------------------------------
// Fire event for LogrusFileHook
func (hook *LogrusFileHook) Fire(entry *logrus.Entry) error {

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
func (hook *LogrusFileHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

//--------------------------------------------------------------------------
func InitLogging(filename string) (log *logrus.Logger) {
	// todo: rotate log files
	// todo: somehow differentiate between debug/release programmatically

	log = logrus.New()

	//log.SetLevel(logrus.WarnLevel)
	log.SetLevel(logrus.TraceLevel)
	log.Formatter = &logrus.TextFormatter{ForceColors: true, FullTimestamp: true}
	log.Level = logrus.TraceLevel
	log.Out = os.Stdout
	log.SetReportCaller(true)

	filehook, err := NewLogrusFileHook(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err == nil {
		log.AddHook(filehook)
	} else {
		fmt.Print("failed creating logfile hook: ", err)
	}

	return
}
