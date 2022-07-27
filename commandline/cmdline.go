package commandline

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"golang.org/x/exp/slices"
)

type commandType int

const ( // commands
	Update commandType = iota
	CheckDownloaded
)

func (cmd commandType) Format(fs fmt.State, c rune) {
	m := map[commandType]string{
		Update:          "update",
		CheckDownloaded: "checkDownloaded",
	}
	fs.Write([]byte(m[cmd]))
}

//--------------------------------------------------------------------------
type CommandLine struct {
	ConfigFile    string
	Command       commandType
	FeedShortname string
	UseProxy      bool
	Debug         bool
}

//--------------------------------------------------------------------------
func InitCommandLine(defaultConfig string) (*CommandLine, error) {

	var c CommandLine

	// flag package is just shit; need to ignore non-flagged arguments (which it doesn't do)
	flag.StringVar(&c.ConfigFile, "config", defaultConfig, "what config toml to use")
	flag.StringVar(&c.FeedShortname, "feed", "", "specify a specific feed (shortname) to work on")
	flag.BoolVar(&c.UseProxy, "use-proxy", false, "use the preconfigured proxy")
	flag.BoolVar(&c.Debug, "debug", false, "debugging")

	flag.Parse()

	if c.ConfigFile != "" {
		if _, err := os.Stat(c.ConfigFile); err != nil {
			return nil, fmt.Errorf("missing config file: %v", err)
		}
	} else {
		return nil, errors.New("config cannot be blank")
	}

	switch {
	case slices.Contains(flag.Args(), "update"):
		c.Command = Update
	case slices.Contains(flag.Args(), "checkDownloaded"):
		c.Command = CheckDownloaded
	default:
		c.Command = Update
	}

	return &c, nil
}
