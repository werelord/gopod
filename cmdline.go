package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/exp/slices"
)

type commandType int

const ( // commands
	update commandType = iota
	checkDownloaded
)

func (cmd commandType) Format(fs fmt.State, c rune) {
	m := map[commandType]string{update: "update", checkDownloaded: "checkDownloaded"}
	fs.Write([]byte(m[cmd]))
}

//--------------------------------------------------------------------------
type CommandLine struct {
	configFile    string
	command       commandType
	feedShortname string
	useProxy      bool
	Debug         bool
}

//--------------------------------------------------------------------------
func (c *CommandLine) initCommandLine(defaultConfig string) bool {

	flag.StringVar(&c.configFile, "config", defaultConfig, "what config toml to use")
	flag.StringVar(&c.feedShortname, "feed", "", "specify a specific feed (shortname) to work on")
	flag.BoolVar(&c.useProxy, "use-proxy", false, "use the preconfigured proxy")
	flag.BoolVar(&c.Debug, "debug", false, "debugging")

	flag.Parse()

	if c.configFile != "" {
		if _, err := os.Stat(c.configFile); err != nil {
			log.Error("missing config file: ", err)
			return false
		}
	} else {
		log.Error("config cannot be blank!")
		return false
	}

	switch {
	case slices.Contains(flag.Args(), "update"):
		c.command = update
	case slices.Contains(flag.Args(), "checkDownloaded"):
		c.command = checkDownloaded
	default:
		c.command = update
	}

	return true
}
