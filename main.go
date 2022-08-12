package main

//--------------------------------------------------------------------------
import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"time"

	"gopod/commandline"
	"gopod/logger"
	"gopod/pod"
	"gopod/podconfig"
	"gopod/poddb"

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
		cmdline        *commandline.CommandLine
		config         *podconfig.Config
		feedList       *[]podconfig.FeedToml
		feedMap        map[string]*pod.Feed
		err            error
		defaultworking = filepath.FromSlash("e:\\gopod\\")
	)

	// todo: flag to check item entries that aren't downloaded
	if cmdline, err = commandline.InitCommandLine(filepath.Join(defaultworking, "master.toml")); err != nil {
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

	poddb.SetDBPath(filepath.Join(filepath.Dir(cmdline.ConfigFile), ".db"))

	// logging initialized, lets output commandline struct
	log.Debugf("cmdline: %+v", cmdline)

	if config, feedList, err = podconfig.LoadToml(cmdline.ConfigFile, runTimestamp); err != nil {
		log.Error("failed to read toml file; exiting!")
		return
	}

	// settings passed from commandline
	config.CommandLineOptions = cmdline.CommandLineOptions

	log.Infof("using config: %+v", config)

	// move feedlist into shortname map
	feedMap = make(map[string]*pod.Feed)
	for _, feedtoml := range *feedList {
		f := pod.NewFeed(config, feedtoml)
		feedMap[f.Shortname] = f
	}

	//	const RunTest = true
	if true {
		test(feedMap)
		return
	}

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
