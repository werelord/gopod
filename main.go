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
	cmdline      CommandLine
	runTimestamp time.Time
	log          *logrus.Logger
)

// todo: changable
const defaultworking = "e:\\gopod\\"

//--------------------------------------------------------------------------
func init() {

	runTimestamp = time.Now()

	// todo: rotate log files with timestamp
	log = podutils.InitLogging(path.Join(defaultworking, "gopod.log"))
	if log == nil {
		panic("logfile failed; wtf")
	}

	// todo: flag to check item entries that aren't downloaded
	cmdline.initCommandLine(path.Join(defaultworking, "master.toml"))
}

//--------------------------------------------------------------------------
func main() {

	const RunTest = false

	if RunTest {
		test(defaultworking)
		return
	}

	var (
		config       Config
		feedTomlList []FeedToml
		err          error
	)

	if config, feedTomlList, err = loadToml(cmdline.Filename, runTimestamp); err != nil {
		log.Error("failed to read toml file; exiting!")
		return
	}

	log.Infof("using config: %+v", config)

	//------------------------------------- DEBUG -------------------------------------
	const UseProxy = false
	if config.Debug && UseProxy {
		var proxyUrl *url.URL
		// setting default transport proxy
		proxyUrl, _ = url.Parse("http://localhost:8888")
		if proxyUrl != nil {
			http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}

		}
	}
	//------------------------------------- DEBUG -------------------------------------

	for _, feedtoml := range feedTomlList {

		f := Feed{FeedToml: feedtoml}

		f.initFeed(&config)

		// todo: parallel via channels??
		f.update()

		// todo: flush all items on debug (schema change handling)

	}
}
