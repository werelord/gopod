package podconfig

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pelletier/go-toml/v2"
)

//--------------------------------------------------------------------------
type Config struct {
	//Foo          bool `toml:"foo"`
	MaxDupChecks uint `toml:"maxdupchecks"`
	// todo: change default
	Workspace    string
	Timestamp    time.Time
	TimestampStr string
	Debug        bool
}

//--------------------------------------------------------------------------
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

//--------------------------------------------------------------------------
func LoadToml(filename string, timestamp time.Time) (*Config, *[]FeedToml, error) {

	// todo: better handling of these objects (pointer?)
	tomldoc := tomldocImport{}
	tomldoc.Config.Timestamp = timestamp
	tomldoc.Config.TimestampStr = timestamp.Format("20060102_150405")
	tomldoc.Config.Workspace = filepath.Dir(filename)

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

//--------------------------------------------------------------------------
func (c *Config) SetDebug(dbg bool) {
	c.Debug = dbg
	//------------------------------------- DEBUG -------------------------------------
	if dbg {
		c.TimestampStr = "DEBUG"
	}
	//------------------------------------- DEBUG -------------------------------------
}
