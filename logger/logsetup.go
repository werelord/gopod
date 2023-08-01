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
	logdir string
)

// --------------------------------------------------------------------------
func InitLogging(workingdir string, timestamp time.Time, level string) error {
	// todo: somehow differentiate between debug/release programmatically

	var (
		relpath  = getRelPathCaller()
		loglevel = mapLogLevel(level)

		reportCaller = (loglevel == charmlog.DebugLevel)	// only show path if we're debug

		allOpt = charmlog.Options{
			Level:           loglevel,
			ReportTimestamp: true,
			ReportCaller:    reportCaller,
			CallerFormatter: genCallFormatter(relpath, false),
			TimeFormat:      "2006-01-02 15:04:05.000",
		}
		errOpt = charmlog.Options{
			Level:           charmlog.WarnLevel, // always warn or above
			ReportTimestamp: true,
			ReportCaller:    reportCaller,
			CallerFormatter: genCallFormatter(relpath, true),
			TimeFormat:      "2006-01-02 15:04:05.000",
		}
	)

	// global styles for console
	charmlog.TimestampStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#1a75ff")).Italic(true)
	charmlog.CallerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080"))
	charmlog.KeyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#66ffff")).Italic(true)
	charmlog.ValueStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#66ff66")).Italic(true)

	log.SetConsoleWithOptions(os.Stderr, allOpt)

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
		log.AddWithOptions(allFile, allOpt)
		log.AddWithOptions(errFile, errOpt)
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

	// log.Print("done")

	log.Infof("logging initialized on '%v'; creating symlinks to latest", filepath.Base(allLevelsFile))
	// create symlinks in workingdir, pointing to files in logdir
	allSymlink := filepath.Join(workingdir, "gopod.all.latest.log")
	errSymlink := filepath.Join(workingdir, "gopod.error.latest.log")

	if err := podutils.CreateSymlink(allLevelsFile, allSymlink); err != nil {
		log.Warnf("failed to create symlink file (all levels): %v", err)

	} else if err2 := podutils.CreateSymlink(errorLevelsFile, errSymlink); err != nil {
		log.Warnf("failed to create symlink file (error levels): %v", err2)
	}

	return nil
}

func genCallFormatter(relpath string, incFunc bool) charmlog.CallerFormatter {

	var fn = func(file string, line int, fn string) string {
		// function; strip qualified path; first dot after first slash
		var (
			ret string
		)
		// return relative path to file
		if relPath, err := filepath.Rel(relpath, file); err == nil {
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
		log.Warnf("failed to rotate logs: %v", err)
	}
	if err := podutils.RotateFiles(logdir, "gopod.error.*.log", uint(numKeep)); err != nil {
		log.Warnf("failed to rotate logs: %v", err)
	}

	return nil
}

func mapLogLevel(lev string) charmlog.Level {
	switch {
	case strings.EqualFold(lev, "debug"):
		return charmlog.DebugLevel
	case strings.EqualFold(lev, "info"):
		return charmlog.InfoLevel
	case strings.EqualFold(lev, "warn"):
		return charmlog.WarnLevel
	case strings.EqualFold(lev, "error"):
		return charmlog.ErrorLevel
	default:
		return charmlog.InfoLevel
	}

}
