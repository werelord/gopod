package main

//--------------------------------------------------------------------------
import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"gopod/commandline"
	"gopod/logger"
	"gopod/pod"
	"gopod/podconfig"

	log "github.com/sirupsen/logrus"
)

//--------------------------------------------------------------------------
var (
	runTimestamp time.Time
	cmdline      *commandline.CommandLine
)

//--------------------------------------------------------------------------
func init() {

	runTimestamp = time.Now()

}

//--------------------------------------------------------------------------
func main() {

	var (
		config         *podconfig.Config
		feedList       *[]podconfig.FeedToml
		feedMap        map[string]*pod.Feed
		err            error
		defaultworking = filepath.FromSlash("e:\\gopod\\")
	)

	const RunTest = false

	if RunTest {
		test(defaultworking)
		return
	}

	// todo: flag to check item entries that aren't downloaded
	if cmdline, err = commandline.InitCommandLine(filepath.Join(defaultworking, "master.toml")); err != nil {
		log.Error("failed to init commandline:", err)
		return
	}

	logger.InitLogging(filepath.Join(cmdline.WorkingDir, "gopod.log"), runTimestamp)

	if config, feedList, err = podconfig.LoadToml(cmdline.ConfigFile, runTimestamp); err != nil {
		log.Error("failed to read toml file; exiting!")
		return
	}

	//------------------------------------- DEBUG -------------------------------------
	config.SetDebug(cmdline.Debug)
	//------------------------------------- DEBUG -------------------------------------
	log.Infof("using config: %+v", config)

	// move feedlist into shortname map
	feedMap = make(map[string]*pod.Feed)
	for _, feedtoml := range *feedList {
		f := pod.NewFeed(config, feedtoml)
		feedMap[f.Shortname] = f
	}

	checkDebugProxy()

	var cmdFunc commandFunc
	if cmdFunc = parseCommand(); cmdFunc == nil {
		log.Error("command not recognized (this should not happen)")
		return
	}

	log.Debugf("running command: '%v'", cmdline.Command)

	if cmdline.FeedShortname != "" {
		if feed, exists := feedMap[cmdline.FeedShortname]; exists {
			cmdFunc(feed)
		} else {
			log.Error("cannot find shortname '%v'; not running command %v!", cmdline.FeedShortname, cmdline.Command)
			os.Exit(1)
		}
	} else {
		log.Infof("running '%v' on all feeds", cmdline.Command)
		for _, feed := range feedMap {
			cmdFunc(feed)

			// future: parallel via channels??
			// todo: flush all items on debug (schema change handling)
		}
	}
}

//--------------------------------------------------------------------------
func checkDebugProxy() {
	//------------------------------------- DEBUG -------------------------------------
	if cmdline.Debug && cmdline.UseProxy {
		var proxyUrl *url.URL
		// setting default transport proxy
		proxyUrl, _ = url.Parse("http://localhost:8888")
		if proxyUrl != nil {
			http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		}
	}
	//------------------------------------- DEBUG -------------------------------------
}

//--------------------------------------------------------------------------
func parseCommand() commandFunc {
	switch cmdline.Command {
	case commandline.Update:
		return runUpdate
	case commandline.CheckDownloaded:
		return runCheckDownloads
	default:
		return nil
	}
}

// command functions
//--------------------------------------------------------------------------
type commandFunc func(*pod.Feed)

func runUpdate(f *pod.Feed) {
	log.Infof("runing update on '%v'", f.Shortname)
	f.Update()
}

//--------------------------------------------------------------------------
func runCheckDownloads(f *pod.Feed) {
	// todo: this
	log.Debug("TODO")
}
