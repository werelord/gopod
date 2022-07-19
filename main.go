package main

//--------------------------------------------------------------------------
import (
	"os"
	"path"
)

//--------------------------------------------------------------------------
var (
	cmdline CommandLine
)

//--------------------------------------------------------------------------
func init() {
	// todo: rotate log files with timestamp
	var defaultworking = "e:/gopod/"

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

	//todo: command-line vars
	programName := os.Args[0]
	log.Debug(programName)

	config, feedTomlList = loadToml(cmdline.Filename)

	log.Debug("config:", config)
	//log.Debug(feedlist)

	for _, feedtoml := range feedTomlList[:1] {
		//log.Debug(feed.Name)
		//log.Debug(feed.Url)

		var f Feed
		f.FeedToml = feedtoml

		f.initFeed(&config)

		// todo: parallel via channels??
		f.update(config)
	}
}
