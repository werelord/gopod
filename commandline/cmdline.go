package commandline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DavidGamba/go-getoptions"
	"golang.org/x/exp/slices"
)

type commandType int

const ( // commands
	Update commandType = iota
	CheckDownloaded
)

var cmdMap = map[commandType]string{
	Update:          "update",
	CheckDownloaded: "checkDownloaded",
}

func (cmd commandType) Format(fs fmt.State, c rune) {
	fs.Write([]byte(cmdMap[cmd]))
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

	opt := getoptions.New()
	opt.SetUnknownMode(getoptions.Pass)

	opt.Bool("help", false, opt.Alias("h", "?"))
	opt.StringVar(&c.ConfigFile, "config", defaultConfig,
		opt.Description("TOML config to use. Either a full path, or file in "+filepath.Dir(defaultConfig)),
		opt.Alias("c"))
	opt.StringVar(&c.FeedShortname, "feed", "",
		opt.Description("feed shortname to compile on"),
		opt.Alias("f"))
	opt.BoolVar(&c.UseProxy, "use-proxy", false,
		opt.Description("use preconfigured proxy"),
		opt.Alias("prox"))
	opt.BoolVar(&c.Debug, "debug", false,
		opt.Description("Debug"),
		opt.Alias("dbg"))

	// i'm not convinced command functions the way I intend.. maybe??
	// for now, just parse remaining for the commands
	// future: figure this out
	//opt.NewCommand("update", "updates feeds")

	//fmt.Printf("%+v\n", os.Args[1:])
	remaining, err := opt.Parse(os.Args[1:])

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n\n", err)
		fmt.Fprint(os.Stderr, opt.Help(getoptions.HelpSynopsis))
		os.Exit(1)
	}
	//fmt.Printf("remaining: %+v\nupdateCalled:%v", remaining, opt.Called("update"))

	if opt.Called("help") {
		fmt.Fprint(os.Stderr, opt.Help())
		os.Exit(1)
	}

	if c.ConfigFile != "" {
		if _, err := os.Stat(c.ConfigFile); err != nil {
			// check relative to default
			work := filepath.Join(filepath.Dir(defaultConfig), c.ConfigFile)
			if _, err := os.Stat(work); err != nil {
				return nil, fmt.Errorf("cannot find config file: %v", c.ConfigFile)
			}
		}

	} else {
		return nil, errors.New("config cannot be blank")
	}

	if slices.Contains(remaining, "update") {
		c.Command = Update
	} else if slices.Contains(remaining, "checkDownloaded") {
		c.Command = CheckDownloaded
	} else {
		// default command
		c.Command = Update
	}

	return &c, nil
}
