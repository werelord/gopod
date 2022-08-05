package commandline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DavidGamba/go-getoptions"
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
	SkipDownload  bool
}

//--------------------------------------------------------------------------
func InitCommandLine(defaultConfig string) (*CommandLine, error) {

	var c CommandLine

	opt := getoptions.New()
	opt.SetUnknownMode(getoptions.Pass)

	opt.StringVar(&c.ConfigFile, "config", defaultConfig,
		opt.Description("TOML config to use; full path to file, or file in "+filepath.Dir(defaultConfig)),
		opt.Alias("c"),
		opt.ArgName("config.toml"))
	opt.StringVar(&c.FeedShortname, "feed", "",
		opt.Description("feed shortname to compile on"),
		opt.Alias("f"), opt.ArgName("shortname"))
	opt.BoolVar(&c.UseProxy, "use-proxy", false,
		opt.Description("use preconfigured proxy"),
		opt.Alias("p", "useproxy"))
	opt.BoolVar(&c.Debug, "debug", false,
		opt.Description("Debug"),
		opt.Alias("dbg"))

	updateCommand := opt.NewCommand("update", "update feeds")
	updateCommand.BoolVar(&c.SkipDownload, "skipdownload", false, opt.Alias("s"),
		opt.Description("skip download of found files"))
	updateCommand.SetCommandFn(c.generateCmdFunc(Update))

	checkcommand := opt.NewCommand("checkdownloads", "check downloads of files")
	checkcommand.SetCommandFn(c.generateCmdFunc(CheckDownloaded))

	opt.HelpCommand("help", opt.Alias("h", "?"))

	//fmt.Printf("%+v\n", os.Args[1:])
	remaining, err := opt.Parse(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %s\n\n", err)
		fmt.Fprint(os.Stderr, opt.Help(getoptions.HelpSynopsis))
		return nil, err
	}
	//fmt.Printf("remaining: %+v\nupdateCalled:%v", remaining, opt.Called("update"))

	ctx, cancel, done := getoptions.InterruptContext()
	defer func() { cancel(); <-done }()

	if err := opt.Dispatch(ctx, remaining); err != nil {
		// if ErrorHelpCalled, caller will handle it
		return nil, err
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

	return &c, nil
}

//--------------------------------------------------------------------------
func (c *CommandLine) generateCmdFunc(t commandType) getoptions.CommandFn {
	fn := func(context.Context, *getoptions.GetOpt, []string) error {
		//fmt.Printf("setting command to %v\n", t)
		c.Command = t
		return nil
	}

	return fn
}
