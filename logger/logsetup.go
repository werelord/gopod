package logger

import (
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

// --------------------------------------------------------------------------
type LogrusFileHook struct {
	file      *os.File
	formatter *log.TextFormatter
	levels    []log.Level
}

const numLogsToKeep = 5

var (
	logdir          string
)

// --------------------------------------------------------------------------
func NewLogrusFileHook(file string, levels []log.Level) (*LogrusFileHook, error) {

	plainFormatter := &log.TextFormatter{DisableColors: true}
	// gc will close the file handle; fuck the finalizer
	logFile, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
		return nil, err
	}
	return &LogrusFileHook{logFile, plainFormatter, levels}, err
}

// --------------------------------------------------------------------------
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

// --------------------------------------------------------------------------
// Levels entry
func (hook *LogrusFileHook) Levels() []log.Level {
	return hook.levels
}

// --------------------------------------------------------------------------
func InitLogging(workingdir string, shortname string, timestamp time.Time) error {
	// todo: somehow differentiate between debug/release programmatically

	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true, FullTimestamp: true})
	log.SetLevel(log.TraceLevel)
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)

	logdir = filepath.Join(workingdir, ".logs")
	allLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.all.%v.%v", shortname,
		timestamp.Format(podutils.TimeFormatStr), "log"))
	errorLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.error.%v.%v", shortname,
		timestamp.Format(podutils.TimeFormatStr), "log"))

	if err := addFileHook(allLevelsFile, log.AllLevels); err != nil {
		fmt.Print("failed creating logfile hook (all levels): ", err)
		return err

	} else {
		// set error/warn file hooks
		errLevels := []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel, log.WarnLevel}
		if err := addFileHook(errorLevelsFile, errLevels); err != nil {
			log.Error("failed creating logfile hook (error levels): ", err)
			// don't fail on this one
		}
	}

	log.Infof("logging initialized on '%v'; creating symlinks to latest", filepath.Base(allLevelsFile))
	// create symlinks in workingdir, pointing to files in logdir
	allSymlink := filepath.Join(workingdir, fmt.Sprintf("%v.all.latest.log", shortname))
	errSymlink := filepath.Join(workingdir, fmt.Sprintf("%v.error.latest.log", shortname))

	if err := podutils.CreateSymlink(allLevelsFile, allSymlink); err != nil {
		log.Warn("failed to create symlink file (all levels): ", err)

	} else if err2 := podutils.CreateSymlink(errorLevelsFile, errSymlink); err != nil {
		log.Warn("failed to create symlink file (error levels): ", err2)
	}

	return nil
}

// --------------------------------------------------------------------------
func addFileHook(filename string, levels []log.Level) error {
	if filehook, err := NewLogrusFileHook(filename, levels); err != nil {
		return err

	} else {
		// stupid GC shit..
		log.AddHook(filehook)
		return nil
	}
}

func RotateLogFiles() error {
	// rotate logfiles
	if err := podutils.RotateFiles(logdir, "gopod.all.*.log", numLogsToKeep); err != nil {
		log.Warn("failed to rotate logs: ", err)
	}
	if err := podutils.RotateFiles(logdir, "gopod.error.*.log", numLogsToKeep); err != nil {
		log.Warn("failed to rotate logs: ", err)
	}

	return nil
}
