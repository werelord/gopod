package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	charmlog "github.com/charmbracelet/log"

	log "gopod/multilogger"
	"gopod/podutils"
)

const numLogsToKeep = 5

var (
	relPath string
	logdir  string
)

// --------------------------------------------------------------------------
func InitLogging(workingdir string, timestamp time.Time) error {
	// todo: somehow differentiate between debug/release programmatically

	relPath = getRelPathCaller()

	var chOpt = charmlog.Options{
		Level:           charmlog.DebugLevel,
		ReportTimestamp: true,
		ReportCaller:    true,
		CallerFormatter: genCallFormatter(false),
		TimeFormat:      "2006-01-02 15:04:05.000",
	}
	var chErr = charmlog.Options{
		Level:           charmlog.WarnLevel,
		ReportTimestamp: true,
		ReportCaller:    true,
		CallerFormatter: genCallFormatter(true),
		TimeFormat:      "2006-01-02 15:04:05.000",
	}

	// global styles for console
	charmlog.TimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a75ff")).Italic(true)
	charmlog.CallerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	charmlog.KeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#66ffff")).Italic(true)
	charmlog.ValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#66ff66")).Italic(true)

	log.SetConsoleWithOptions(os.Stderr, chOpt)

	logdir = filepath.Join(workingdir, ".logs")
	// make sure dir exists
	if err := podutils.MkdirAll(logdir); err != nil {
		fmt.Printf("error making log directory: %v", err)
		return err
	}

	var timestampStr = timestamp.Format(podutils.TimeFormatStr)

	allLevelsFile := filepath.Join(logdir, fmt.Sprintf("gopod.all.%v.log", timestampStr))
	errorLevelsFile := filepath.Join(logdir, fmt.Sprintf("gopod.error.%v.log", timestampStr))

	if allFile, err := os.Create(allLevelsFile); err != nil {
		return err
	} else if errFile, err := os.Create(errorLevelsFile); err != nil {
		return err
	} else {
		log.AddWithOptions(allFile, chOpt)
		log.AddWithOptions(errFile, chErr)
	}

	// log.Debug("foo")
	// log.Info("bar")
	// log.Warnf("foo %v", "warn")
	// log.Errorf("Error %v", errors.New("test err"))
	// log.Print("print me foo")

	// var foo = log.With("foo", 13, "bar", 42)
	// foo.Debugf("bar ''%v''", 42)
	// foo.Info("motherfucker this should be info")

	// foo.Debug("foo")
	// foo.Info("bar")
	// foo.Warnf("foo %v", "warn")
	// foo.Errorf("Error %v", errors.New("test err"))
	// foo.Print("print me foo")

	log.Infof("logging initialized on '%v'; creating symlinks to latest", filepath.Base(allLevelsFile))
	// create symlinks in workingdir, pointing to files in logdir
	allSymlink := filepath.Join(workingdir, "gopod.all.latest.log")
	errSymlink := filepath.Join(workingdir, "gopod.error.latest.log")

	if err := podutils.CreateSymlink(allLevelsFile, allSymlink); err != nil {
		log.Warn("failed to create symlink file (all levels): ", err)

	} else if err2 := podutils.CreateSymlink(errorLevelsFile, errSymlink); err != nil {
		log.Warn("failed to create symlink file (error levels): ", err2)
	}

	return nil
}

func genCallFormatter(incFunc bool) charmlog.CallerFormatter {

	var fn = func(file string, line int, fn string) string {
		// function; strip qualified path; first dot after first slash
		var (
			ret string
		)
		// return relative path to file
		if relPath, err := filepath.Rel(relPath, file); err == nil {
			file = relPath
		}
		ret = fmt.Sprintf("%s:%d", file, line)
		if incFunc == false {
			return ret
		}

		// including function here
		if idx := strings.Index(fn, "/"); idx > -1 {
			fn = fn[idx+1:]
		}
		return fmt.Sprintf("%s (%s)", ret, fn)
	}

	return fn
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
