package logger

import (
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	logdir string
)

// --------------------------------------------------------------------------
func NewLogrusFileHook(file string, levels []log.Level, relpath string) (*LogrusFileHook, error) {

	plainFormatter := &log.TextFormatter{DisableColors: true, CallerPrettyfier: generatePrettyfier(relpath)}
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

	var relPath = getRelPathCaller()

	log.SetLevel(log.TraceLevel)
	log.SetFormatter(&log.TextFormatter{ForceColors: true, FullTimestamp: true, CallerPrettyfier: generatePrettyfier(relPath)})
	log.SetLevel(log.TraceLevel)
	log.SetOutput(os.Stdout)
	log.SetReportCaller(true)

	logdir = filepath.Join(workingdir, ".logs")
	allLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.all.%v.%v", shortname,
		timestamp.Format(podutils.TimeFormatStr), "log"))
	errorLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.error.%v.%v", shortname,
		timestamp.Format(podutils.TimeFormatStr), "log"))

	if err := addFileHook(allLevelsFile, log.AllLevels, relPath); err != nil {
		fmt.Print("failed creating logfile hook (all levels): ", err)
		return err

	} else {
		// set error/warn file hooks
		errLevels := []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel, log.WarnLevel}
		if err := addFileHook(errorLevelsFile, errLevels, relPath); err != nil {
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
func getRelPathCaller() string {

	// this is relative to main.go.. if main ever moves, then this relative path may need to change
	if _, file, _, ok := runtime.Caller(2); ok {
		// fmt.Printf("dir:%#v\n", filepath.Dir(file))
		return filepath.Dir(file)

	} else {
		return ""
	}
}

// --------------------------------------------------------------------------
func generatePrettyfier(relPath string) func(*runtime.Frame) (string, string) {
	return func(frame *runtime.Frame) (functionName string, filename string) {
		if frame != nil {

			// strip qualified path; first dot after first slash
			var function = frame.Function
			if idx := strings.Index(function, "/"); idx > -1 {
				if idy := strings.Index(function[idx:], "."); idy > -1 {
					function = function[idx+idy+1:]
				}
			}

			// return relative path to file
			var file = frame.File
			if relPath, err := filepath.Rel(relPath, frame.File); err == nil {
				file = relPath
			}
			return fmt.Sprintf("%v()", function), fmt.Sprintf(" %s:%d", file, frame.Line)
		}
		return
	}
}

// --------------------------------------------------------------------------
func addFileHook(filename string, levels []log.Level, relPath string) error {
	if filehook, err := NewLogrusFileHook(filename, levels, relPath); err != nil {
		return err

	} else {
		// stupid GC shit..
		log.AddHook(filehook)
		return nil
	}
}

func RotateLogFiles(numKeep int) error {

	if numKeep < 0 {
		log.WithField("numkeep", numKeep).Debug("number of logs to keep is negative, not rotating")
	} else if numKeep == 0 {
		// undefined or set to 0
		numKeep = numLogsToKeep
	}
	// rotate logfiles
	if err := podutils.RotateFiles(logdir, "gopod.all.*.log", uint(numKeep)); err != nil {
		log.Warn("failed to rotate logs: ", err)
	}
	if err := podutils.RotateFiles(logdir, "gopod.error.*.log", uint(numKeep)); err != nil {
		log.Warn("failed to rotate logs: ", err)
	}

	return nil
}
