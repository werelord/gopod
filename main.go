package main

//--------------------------------------------------------------------------
import (
	"gopod/podutils"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/sirupsen/logrus"
)

//--------------------------------------------------------------------------
var (
	runTimestamp time.Time
	log          *logrus.Logger
	cmdline      CommandLine
	config       *Config
)

// todo: changable

const Defaultworking = "e:\\gopod\\"

//--------------------------------------------------------------------------
func init() {

	runTimestamp = time.Now()

	// todo: rotate log files with timestamp
	log = podutils.InitLogging(path.Join(Defaultworking, "gopod.log"))
	if log == nil {
		panic("logfile failed; wtf")
	}

}

//--------------------------------------------------------------------------
func main() {

	const RunTest = false

	if RunTest {
		test(Defaultworking)
		return
	}

	var (
		feedList map[string]*Feed
		err      error
	)

	// todo: flag to check item entries that aren't downloaded
	if cmdline.initCommandLine(path.Join(Defaultworking, "master.toml")) == false {
		log.Error("failed to init commandline")
		return
	}

	if config, feedList, err = loadToml(cmdline.configFile, runTimestamp); err != nil {
		log.Error("failed to read toml file; exiting!")
		return
	}

	log.Infof("using config: %+v", config)

	checkDebugProxy()

	var cmdFunc commandFunc
	if cmdFunc = parseCommand(); cmdFunc == nil {
		log.Error("command not recognized (this should not happen)")
		return
	}

	log.Debugf("running command: '%v'", cmdline.command)

	if cmdline.feedShortname != "" {
		if feed, exists := feedList[cmdline.feedShortname]; exists {
			cmdFunc(feed)
		} else {
			log.Error("cannot find shortname '%v'", cmdline.feedShortname)
		}
	} else {
		log.Infof("running '%v' on all feeds", cmdline.command)
		for _, feed := range feedList {
			cmdFunc(feed)

			// future: parallel via channels??
			// todo: flush all items on debug (schema change handling)
		}
	}

}

//--------------------------------------------------------------------------
func checkDebugProxy() {
	//------------------------------------- DEBUG -------------------------------------
	if cmdline.Debug && cmdline.useProxy {
		var proxyUrl *url.URL
		// setting default transport proxy
		proxyUrl, _ = url.Parse("http://localhost:8888")
		if proxyUrl != nil {
			http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}
		}
	}
	//------------------------------------- DEBUG -------------------------------------
}

func parseCommand() commandFunc {
	switch cmdline.command {
	case update:
		return runUpdate
	case checkDownloaded:
		return runCheckDownloads
	default:
		return nil
	}
}

// command functions
//--------------------------------------------------------------------------
type commandFunc func(*Feed)

func runUpdate(f *Feed) {
	log.Info("runing update on ", f.Shortname)
	f.Update()
}

//--------------------------------------------------------------------------
func runCheckDownloads(f *Feed) {
	// todo: this
	log.Debug("TODO")
}
