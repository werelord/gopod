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

//--------------------------------------------------------------------------
func init() {
	// todo: rotate log files with timestamp
	var defaultworking = "e:\\gopod\\"
	runTimestamp = time.Now()

	initLogging(path.Join(defaultworking, "gopod.log"))

	cmdline.initCommandLine(path.Join(defaultworking, "master.toml"))
}

//--------------------------------------------------------------------------
func main() {
	//test()

	var (
		config       Config
		feedTomlList []FeedToml
	)

	config, feedTomlList = loadToml(cmdline.Filename, runTimestamp)

	log.Info("using config:", config)

	for _, feedtoml := range feedTomlList[:1] {

		f := Feed{FeedToml: feedtoml}

		f.initFeed(&config)

		// todo: parallel via channels??
		f.update()

		// todo: flush all items on debug (schema change handling)

	}
}
