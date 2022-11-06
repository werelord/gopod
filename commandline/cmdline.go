package commandline

import (
	"context"
	"errors"
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"

	"github.com/DavidGamba/go-getoptions"
)

type CommandType int

const ( // commands
	Unknown CommandType = iota
	Update
	CheckDownloaded
)

// for testing purposes
var fileExists = podutils.FileExists

var cmdMap = map[CommandType]string{
	Unknown:         "unknown command",
	Update:          "update",
	CheckDownloaded: "checkDownloaded",
}

func (cmd CommandType) Format(fs fmt.State, c rune) {
	fs.Write([]byte(cmdMap[cmd]))
}

// --------------------------------------------------------------------------
type CommandLine struct {
	ConfigFile    string
	Command       CommandType
	FeedShortname string
	Proxy         string

	CommandLineOptions
}

type CommandLineOptions struct {
	// global
	Debug bool

	UpdateOpt
	CheckDownloadOpt
}

// update specific
type UpdateOpt struct {
	Simulate         bool
	ForceUpdate      bool
	UseMostRecentXml bool
}

// check downloads specific
type CheckDownloadOpt struct {
	DoArchive     bool
	DoRename      bool
	SaveCollision bool
	DoCollision   bool
}

// --------------------------------------------------------------------------
func InitCommandLine(args []string) (*CommandLine, error) {

	var c CommandLine

	opt := c.buildOptions()

	//fmt.Printf("%+v\n", os.Args[1:])
	remaining, err := opt.Parse(args)
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
	} else if c.Command == Unknown {
		// getopts should already have outputted the help text
		return nil, errors.New("command not recognized")
	}

	if c.ConfigFile == "" {
		return nil, errors.New("config cannot be blank")
	} else {
		// check for config file
		var workfile = c.ConfigFile
		if exists, err := podutils.FileExists(workfile); err != nil {
			return nil, fmt.Errorf("error checking config file exists: %w", err)

		} else if exists == false {
			// check relative to currently running executable
			if ex, err := os.Executable(); err != nil {
				return nil, fmt.Errorf("error checking config file exists: %w", err)

			} else {
				workfile = filepath.Join(filepath.Dir(ex), c.ConfigFile)
				if exists, err := fileExists(workfile); err != nil {
					return nil, fmt.Errorf("error checking config file exists: %w", err)
				} else if exists == false {
					return nil, fmt.Errorf("cannot find config file '%v'; file does not exist", c.ConfigFile)
				}
			}
		}
		// we should have a valid config here..
		c.ConfigFile = workfile
	}

	return &c, nil
}

// --------------------------------------------------------------------------
func (c *CommandLine) buildOptions() *getoptions.GetOpt {
	opt := getoptions.New()
	opt.SetUnknownMode(getoptions.Pass)

	opt.StringVar(&c.ConfigFile, "config", "",
		opt.Required("config required"),
		opt.Description("TOML config to use"),
		opt.Alias("c"),
		opt.ArgName("config.toml"))
	opt.StringVar(&c.FeedShortname, "feed", "",
		opt.Description("feed shortname to compile on"),
		opt.Alias("f"), opt.ArgName("shortname"))
	opt.StringVar(&c.Proxy, "proxy", "",
		opt.Description("use proxy url"),
		opt.Alias("p", " proxy"))
	opt.BoolVar(&c.Debug, "debug", false,
		opt.Description("Debug"),
		opt.Alias("dbg"))

	updateCommand := opt.NewCommand("update", "update feeds")
	updateCommand.BoolVar(&c.Simulate, "simulate", false, opt.Alias("sim"),
		opt.Description("Simulate; will not download items or save database"))
	updateCommand.BoolVar(&c.ForceUpdate, "force", false,
		opt.Description("force update on xml and items (will process everything in feed"))
	updateCommand.BoolVar(&c.UseMostRecentXml, "use-recent", false, opt.Alias("use-recent-xml", "userecent"),
		opt.Description("Use the most recent feed xml file fetched rather than checking for new; if recent doesn't exist, will still download.  Note: if there are no errors on previous run, will likely do nothing unless --force is specified"))
	updateCommand.SetCommandFn(c.generateCmdFunc(Update))

	checkcommand := opt.NewCommand("checkdownloads", "check downloads of files")
	checkcommand.BoolVar(&c.DoArchive, "archive", false, opt.Alias("arc"),
		opt.Description("set missing downloads to archived"))
	checkcommand.BoolVar(&c.DoRename, "rename", false,
		opt.Description("perform rename on files dependant on Filename parse (useful when parse value changes"))
	checkcommand.BoolVar(&c.DoCollision, "collision", false, opt.Alias("coll"),
		opt.Description("Collision handling; will promt for which item to keep"))
	checkcommand.BoolVar(&c.SaveCollision, "savecollision", false, opt.Alias("savecoll"),
		opt.Description("Save collision differences to <workingdir>\\.collisions\\"))
	checkcommand.SetCommandFn(c.generateCmdFunc(CheckDownloaded))

	opt.HelpCommand("help", opt.Alias("h", "?"))
	return opt
}

// --------------------------------------------------------------------------
func (c *CommandLine) generateCmdFunc(t CommandType) getoptions.CommandFn {
	fn := func(context.Context, *getoptions.GetOpt, []string) error {
		//fmt.Printf("setting command to %v\n", t)
		c.Command = t
		return nil
	}

	return fn
}
