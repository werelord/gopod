package podconfig

import (
	"fmt"
	"gopod/commandline"
	"gopod/podutils"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// --------------------------------------------------------------------------
type Config struct {
	LogFilesRetained int `toml:"logfilesretained"`
	MaxDupChecks     int `toml:"dupcheckmax"`
	XmlFilesRetained int `toml:"xmlfilesretained"`
	WorkspaceDir     string
	Timestamp        time.Time
	TimestampStr     string
	// add in commandline options explicitly
	commandline.CommandLineOptions
}

// --------------------------------------------------------------------------
type tomldocImport struct {
	Config   Config     `toml:"config"`
	Feedlist []FeedToml `toml:"feed"`
}

type FeedToml struct {
	Name          string `toml:"name"`
	Shortname     string `toml:"shortname"`
	Url           string `toml:"url"`
	FilenameParse string `toml:"filenameParse"`
	Regex         string `toml:"regex"`
	UrlParse      string `toml:"urlParse"`
	SkipFileTrim  bool   `toml:"skipFileTrim"`
	EpisodePad    int    `toml:"episodePad"`
}

// --------------------------------------------------------------------------
func LoadToml(filename string, timestamp time.Time) (*Config, *[]FeedToml, error) {

	// todo: better handling of these objects (pointer?)
	tomldoc := tomldocImport{}
	tomldoc.Config.Timestamp = timestamp
	tomldoc.Config.TimestampStr = timestamp.Format(podutils.TimeFormatStr)
	tomldoc.Config.WorkspaceDir = filepath.Dir(filename)

	// defaults, if not defined in config
	tomldoc.Config.MaxDupChecks = 3
	tomldoc.Config.XmlFilesRetained = 4

	file, err := os.Open(filename)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open %v: %v ", filename, err)
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("readall '%v' failed: %v", filename, err)
	}

	if err := toml.Unmarshal(buf, &tomldoc); err != nil {
		return nil, nil, fmt.Errorf("toml.unmarshal failed: %v ", err)
	}

	return &tomldoc.Config, &tomldoc.Feedlist, nil

}
