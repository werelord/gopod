package logger

import (
	"errors"
	"fmt"
	"os"

	// "os"
	"path/filepath"
	// "runtime"
	// "strings"
	"time"

	charmlog "github.com/charmbracelet/log"

	// log "github.com/sirupsen/logrus"
	log "gopod/multilogger"
	"gopod/podutils"
)

// --------------------------------------------------------------------------
// type LogrusFileHook struct {
// 	file      *os.File
// 	formatter *log.TextFormatter
// 	levels    []log.Level
// }

const numLogsToKeep = 5

var (
	logdir string
)

// --------------------------------------------------------------------------
// func NewLogrusFileHook(file string, levels []log.Level, relpath string) (*LogrusFileHook, error) {

// 	plainFormatter := &log.TextFormatter{DisableColors: true, CallerPrettyfier: generatePrettyfier(relpath)}
// 	// gc will close the file handle; fuck the finalizer
// 	logFile, err := os.OpenFile(file, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0666)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "unable to write file on filehook %v", err)
// 		return nil, err
// 	}
// 	return &LogrusFileHook{logFile, plainFormatter, levels}, err
// }

// --------------------------------------------------------------------------
// Fire event for LogrusFileHook
// func (hook *LogrusFileHook) Fire(entry *log.Entry) error {

// 	plainformat, err := hook.formatter.Format(entry)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
// 		return err
// 	}
// 	line := string(plainformat)
// 	_, err = hook.file.WriteString(line)
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "unable to write file on filehook(entry.String)%v", err)
// 		return err
// 	}

// 	return nil
// }

// --------------------------------------------------------------------------
// Levels entry
// func (hook *LogrusFileHook) Levels() []log.Level {
// 	return hook.levels
// }

// --------------------------------------------------------------------------
func InitLogging(workingdir string, shortname string, timestamp time.Time) error {
	// todo: somehow differentiate between debug/release programmatically

	// var relPath = getRelPathCaller()

	var chOpt = charmlog.Options{
		Level:           charmlog.DebugLevel,
		ReportTimestamp: true,
		ReportCaller:    true,
	}

	var console = charmlog.NewWithOptions(os.Stderr, chOpt)

	log.SetConsoleLogger(console)

	logdir = filepath.Join(workingdir, ".logs")
	// make sure dir exists
	if err := podutils.MkdirAll(logdir); err != nil {
		fmt.Printf("error making log directory: %v", err)
		return err
	}

	allLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.allTEST.log", shortname,
		// timestamp.Format(podutils.TimeFormatStr)
		))
	errorLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.errorTEST.log", shortname,
		// timestamp.Format(podutils.TimeFormatStr)
		))

	if allFile, err := os.Create(allLevelsFile); err != nil {
		return err
	} else if errFile, err := os.Create(errorLevelsFile); err != nil {
		return err
	} else {
		log.AddLogger(charmlog.NewWithOptions(allFile, chOpt))
		log.AddLogger(charmlog.NewWithOptions(errFile, charmlog.Options{
			Level: charmlog.WarnLevel,
			ReportTimestamp: true,
			ReportCaller: true,
		}))
	}

	log.Debug("foo")
	log.Info("bar")
	log.Warnf("foo %v", "warn")
	log.Errorf("Error %v", errors.New("test err"))
	log.Print("print me foo")

	var foo = log.With("foo", 13, "bar", 42)
	foo.Debugf("bar ''%v''", 42)
	foo.Info("motherfucker this should be info")

	foo.Debug("foo")
	foo.Info("bar")
	foo.Warnf("foo %v", "warn")
	foo.Errorf("Error %v", errors.New("test err"))
	foo.Print("print me foo")

	// multi := io.MultiWriter(os.Stderr /*file*/)
	// log.SetOutput(multi)
	// log.SetOutput(os.Stdout)

	// log.Debug("fuckme")
	// log.Debugf("mee too %v", allLevelsFile)

	fmt.Print("done")
	// errorLevelsFile := filepath.Join(logdir, fmt.Sprintf("%v.error.%v.%v", shortname,
	// 	timestamp.Format(podutils.TimeFormatStr), "log"))

	// if err := addFileHook(allLevelsFile, log.AllLevels, relPath); err != nil {
	// 	fmt.Print("failed creating logfile hook (all levels): ", err)
	// 	return err

	// } else {
	// 	// set error/warn file hooks
	// 	errLevels := []log.Level{log.PanicLevel, log.FatalLevel, log.ErrorLevel, log.WarnLevel}
	// 	if err := addFileHook(errorLevelsFile, errLevels, relPath); err != nil {
	// 		log.Error("failed creating logfile hook (error levels): ", err)
	// 		// don't fail on this one
	// 	}
	// }

	// log.Infof("logging initialized on '%v'; creating symlinks to latest", filepath.Base(allLevelsFile))
	// // create symlinks in workingdir, pointing to files in logdir
	// allSymlink := filepath.Join(workingdir, fmt.Sprintf("%v.all.latest.log", shortname))
	// errSymlink := filepath.Join(workingdir, fmt.Sprintf("%v.error.latest.log", shortname))

	// if err := podutils.CreateSymlink(allLevelsFile, allSymlink); err != nil {
	// 	log.Warn("failed to create symlink file (all levels): ", err)

	// } else if err2 := podutils.CreateSymlink(errorLevelsFile, errSymlink); err != nil {
	// 	log.Warn("failed to create symlink file (error levels): ", err2)
	// }

	return nil
}

// --------------------------------------------------------------------------
// func getRelPathCaller() string {

// 	// this is relative to main.go.. if main ever moves, then this relative path may need to change
// 	if _, file, _, ok := runtime.Caller(2); ok {
// 		// fmt.Printf("dir:%#v\n", filepath.Dir(file))
// 		return filepath.Dir(file)

// 	} else {
// 		return ""
// 	}
// }

// --------------------------------------------------------------------------
// func generatePrettyfier(relPath string) func(*runtime.Frame) (string, string) {
// 	return func(frame *runtime.Frame) (functionName string, filename string) {
// 		if frame != nil {

// 			// strip qualified path; first dot after first slash
// 			var function = frame.Function
// 			if idx := strings.Index(function, "/"); idx > -1 {
// 				if idy := strings.Index(function[idx:], "."); idy > -1 {
// 					function = function[idx+idy+1:]
// 				}
// 			}

// 			// return relative path to file
// 			var file = frame.File
// 			if relPath, err := filepath.Rel(relPath, frame.File); err == nil {
// 				file = relPath
// 			}
// 			return fmt.Sprintf("%v()", function), fmt.Sprintf(" %s:%d", file, frame.Line)
// 		}
// 		return
// 	}
// }

// --------------------------------------------------------------------------
// func addFileHook(filename string, levels []log.Level, relPath string) error {
// 	if filehook, err := NewLogrusFileHook(filename, levels, relPath); err != nil {
// 		return err

// 	} else {
// 		// stupid GC shit..
// 		log.AddHook(filehook)
// 		return nil
// 	}
// }

func RotateLogFiles(numKeep int) error {

	if numKeep < 0 {
		log.With("numkeep", numKeep).Debug("number of logs to keep is negative, not rotating")
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
