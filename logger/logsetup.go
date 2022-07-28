package logger

import (
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

//--------------------------------------------------------------------------
type LogrusFileHook struct {
	file      *os.File
	flag      int
	chmod     os.FileMode
	formatter *log.TextFormatter
}

const numLogsToKeep = 5

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
func InitLogging(filename string, timestamp time.Time) error {
	// todo: rotate log files
	// todo: somehow differentiate between debug/release programmatically

	dir, file := filepath.Split(filename)
	ext := filepath.Ext(file)

	lfname := fmt.Sprintf("%v.%v%v", file[0:len(ext)+1], timestamp.Format("20060102_150405"), ext)
	logfile := filepath.Join(dir, lfname)

	//log.SetLevel(logrus.WarnLevel)
	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true, FullTimestamp: true})
	log.SetLevel(log.TraceLevel)
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)

	filehook, err := NewLogrusFileHook(logfile, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err == nil {
		log.AddHook(filehook)
	} else {
		fmt.Print("failed creating logfile hook: ", err)
		return err
	}
	log.Infof("logging initialized on '%v'; creating symlink to latest", logfile)
	symlink := filepath.Join(dir, fmt.Sprintf("%v.latest%v", file[0:len(ext)+1], ext))

	// remove the symlink before recreating it..
	if (os.Remove(symlink)) != nil {
		log.Warn("failed to remove latest symlink: ", err)
	} else if err := os.Symlink(logfile, symlink); err != nil {
		log.Warn("failed to create symlink: ", err)
	}

	// rotate logfiles
	if err := podutils.RotateFiles(dir, "gopod.[0-9]{8}_[0-9]{6}.log", numLogsToKeep); err != nil {
		log.Warn("failed to rotate logs: ", err)
	}

	return nil
}
