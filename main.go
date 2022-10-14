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
	runTimestamp   time.Time
	defaultworking = filepath.FromSlash("e:\\gopod\\")
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

	// todo: flag to check item entries that aren't downloaded
	if cmdline, err = commandline.InitCommandLine(filepath.Join(defaultworking, "master.toml"), os.Args[1:]); err != nil {
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
		log.Error("failed to read toml file; exiting!")
		return
	}

	// settings passed from commandline
	config.CommandLineOptions = cmdline.CommandLineOptions

	log.Infof("using config: %+v", config)

	// todo: lock down pointer receivers where necessary

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
	//	const RunTest = true
	if config.Debug && RunTest(*config, feedMap) {
		// runtest run, exit
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

	// todo: separate updating xml feed with downloading files (move to higher abstraction)

	if cmdline.FeedShortname != "" {
		if feed, exists := feedMap[cmdline.FeedShortname]; exists {
			cmdFunc(feed)
		} else {
			log.Errorf("cannot find shortname '%v'; not running command %v!", cmdline.FeedShortname, cmdline.Command)
			return
		}
	} else {
		log.Infof("running '%v' on all feeds", cmdline.Command)
		for _, feed := range feedMap {
			cmdFunc(feed)

			// future: parallel via channels??
		}
	}

	// todo: db export to json

	// rotate the log files
	logger.RotateLogFiles()
}

// --------------------------------------------------------------------------
func RunTest(config podconfig.Config, feedMap map[string]*pod.Feed) (exit bool) {
	if config.Debug && false {
		//convertdb(feedMap)
		//checkdb(feedMap)
		//checkHashes(feedMap)
		testConflict(feedMap)
		return true
	}
	return false
}

// --------------------------------------------------------------------------
func SetupDB(cfg podconfig.Config) (*pod.PodDB, error) {
	var dbpath = filepath.Join(defaultworking, "gopod_TEST.db")

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
	default:
		return nil
	}
}

// command functions
// --------------------------------------------------------------------------
type commandFunc func(*pod.Feed)

func runUpdate(f *pod.Feed) {
	log.Infof("runing update on '%v'", f.Shortname)
	f.Update()
}

// --------------------------------------------------------------------------
func runCheckDownloads(f *pod.Feed) {
	// todo: this
	log.Debug("TODO")
}
