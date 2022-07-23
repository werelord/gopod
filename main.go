package main

//--------------------------------------------------------------------------
import (
	"path"
	"time"
)

//--------------------------------------------------------------------------
var (
	cmdline      CommandLine
	runTimestamp time.Time
)

// todo: changable
const defaultworking = "e:\\gopod\\"

//--------------------------------------------------------------------------
func init() {

	runTimestamp = time.Now()

	// todo: rotate log files with timestamp
	initLogging(path.Join(defaultworking, "gopod.log"))

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
	)

	config, feedTomlList = loadToml(cmdline.Filename, runTimestamp)

	log.Infof("using config: %+v", config)

	// if config.Debug {
	// 	var proxyUrl *url.URL
	// 	// setting default transport proxy
	// 	proxyUrl, _ = url.Parse("http://localhost:8888")
	// 	if proxyUrl != nil {
	// 		http.DefaultTransport = &http.Transport{Proxy: http.ProxyURL(proxyUrl)}

	// 	}
	// }

	for _, feedtoml := range feedTomlList[1:2] {

		f := Feed{FeedToml: feedtoml}

		f.initFeed(&config)

		// todo: parallel via channels??
		f.update()

		// todo: flush all items on debug (schema change handling)

	}
}
