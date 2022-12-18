package main

//--------------------------------------------------------------------------
import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopod/commandline"
	"gopod/logger"
	"gopod/pod"
	"gopod/podconfig"

	"github.com/DavidGamba/go-getoptions"
	log "github.com/sirupsen/logrus"
)

// --------------------------------------------------------------------------
var (
	runTimestamp time.Time
)

// --------------------------------------------------------------------------
func init() {

	runTimestamp = time.Now()

}

// --------------------------------------------------------------------------
func main() {

	var (
		cmdline  *commandline.CommandLine
		poddb    *pod.PodDB
		config   *podconfig.Config
		feedList *[]podconfig.FeedToml
		feedMap  map[string]*pod.Feed

		err error
	)

	if cmdline, err = commandline.InitCommandLine(os.Args[1:]); err != nil {
		// if help called, no errors to output
		if errors.Is(err, getoptions.ErrorHelpCalled) == false {
			fmt.Println("failed to init commandline:", err)
		}
		return
	}

	if err := logger.InitLogging(filepath.Dir(cmdline.ConfigFile), "gopod", runTimestamp); err != nil {
		fmt.Println("failed to initialize logging: ", err)
		return
	}

	// logging initialized, lets output commandline struct
	log.Debugf("cmdline: %+v", cmdline)

	if config, feedList, err = podconfig.LoadToml(cmdline.ConfigFile, runTimestamp); err != nil {
		log.Errorf("failed to read toml file: %v", err)
		return
	}

	// settings passed from commandline
	config.CommandLineOptions = cmdline.CommandLineOptions

	log.Infof("using config: %+v", config)

	// todo: official poddb migration methods
	if poddb, err = SetupDB(*config); err != nil {
		log.Error("Failed setting up db: ", err)
		return
	}
	pod.Init(config, poddb)

	// move feedlist into shortname map
	feedMap = make(map[string]*pod.Feed)
	for _, feedtoml := range *feedList {
		f, err := pod.NewFeed(feedtoml)
		if err != nil {
			log.Error("failed to create new feed: ", err)
			continue
		}
		feedMap[f.Shortname] = f
	}

	//------------------------------------- DEBUG -------------------------------------
	if config.Debug && RunTest(*config, feedMap, poddb) {
		// runtest was run, exit
		return
	}
	//------------------------------------- DEBUG -------------------------------------

	if len(cmdline.Proxy) > 0 {
		setProxy(cmdline.Proxy)
	}

	var cmdFunc commandFunc
	if cmdFunc = parseCommand(cmdline.Command); cmdFunc == nil {
		log.Error("command not recognized (this should not happen)")
		return
	}

	log.Debugf("running command: '%v'", cmdline.Command)

	var cmdLog string
	if cmdline.FeedShortname != "" {
		if feed, exists := feedMap[cmdline.FeedShortname]; exists {
			cmdLog = cmdFunc(feed)
		} else {
			log.Errorf("cannot find shortname '%v'; not running command %v!", cmdline.FeedShortname, cmdline.Command)
			return
		}
	} else {
		log.Infof("running '%v' on all feeds", cmdline.Command)
		for _, feed := range feedMap {
			cmdLog += cmdFunc(feed)

			// future: parallel via channels??
		}
	}

	if cmdLog != "" {
		// just for console output, not logging
		fmt.Printf("finished %v:\n%v", cmdline.Command, cmdLog)
	}

	// rotate the log files
	if config.LogFilesRetained > 0 {
		logger.RotateLogFiles(config.LogFilesRetained)
	}
}

// --------------------------------------------------------------------------
func RunTest(config podconfig.Config, feedMap map[string]*pod.Feed, db *pod.PodDB) (exit bool) {
	if config.Debug && false {

		//doMigrate(feedMap, db)
		return true
	}
	return false
}

// --------------------------------------------------------------------------
func SetupDB(cfg podconfig.Config) (*pod.PodDB, error) {
	dbpath := filepath.Join(cfg.WorkspaceDir, ".db", "gopod_test.db")
	// dbpath := filepath.Join(cfg.WorkspaceDir, ".db", "gopod.db")

	if db, err := pod.NewDB(dbpath); err != nil {
		return nil, err
	} else {
		return db, nil
	}
}

// --------------------------------------------------------------------------
func setProxy(urlstr string) {
	if len(urlstr) > 0 {
		// setting default transport proxy.. don't care about the error on parse,
		if proxyUrl, err := url.ParseRequestURI(urlstr); err != nil {
			log.Error("Failed to parse proxy url: ", err)
		} else if proxyUrl != nil {
			http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		}
	}
}

// --------------------------------------------------------------------------
func parseCommand(cmd commandline.CommandType) commandFunc {
	switch cmd {
	case commandline.Update:
		return runUpdate
	case commandline.CheckDownloaded:
		return runCheckDownloads
	case commandline.Delete:
		return runDelete
	case commandline.Preview:
		return runPreview
	default:
		return nil
	}
}

// command functions
// --------------------------------------------------------------------------
type commandFunc func(*pod.Feed) string

func runUpdate(f *pod.Feed) string {
	log.Infof("runing update on '%v'", f.Shortname)
	if downloaded, err := f.Update(); err != nil {
		log.Errorf("Error in updating feed '%v': %v", f.Shortname, err)
		return ""
	} else {
		return downloaded
	}
}

// --------------------------------------------------------------------------
func runCheckDownloads(f *pod.Feed) string {
	log.Infof("running check downloads on '%v'", f.Shortname)
	if err := f.CheckDownloads(); err != nil {
		log.Errorf("Error in checking downloads for feed '%v': %v", f.Shortname, err)
	}
	return ""
}

// --------------------------------------------------------------------------
func runDelete(f *pod.Feed) string {
	// todo: logging of what's deleted
	log.WithField("feed", f.Shortname).Infof("running delete")
	if err := f.RunDelete(); err != nil {
		log.WithFields(log.Fields{
			"feed":  f.Shortname,
			"error": err,
		}).Error("failed running delete")
	}
	return ""
}

// --------------------------------------------------------------------------
func runPreview(f *pod.Feed) string {
	log.WithField("feed", f.Shortname).Info("running preview")
	if err := f.Preview(); err != nil {
		log.WithFields(log.Fields{
			"feed":  f.Shortname,
			"error": err,
		}).Error("failed running preview")
	}
	return ""
}
