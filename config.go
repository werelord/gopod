package main

import (
	"io"
	"os"
	"path"
	"time"

	"github.com/pelletier/go-toml/v2"
)

//--------------------------------------------------------------------------
type Config struct {
	//Foo          bool `toml:"foo"`
	Debug        bool `toml:"debug"`
	MaxDupChecks uint `toml:"maxdupchecks"`
	// todo: change default
	workspace    string
	timestamp    time.Time
	timestampStr string
}

//--------------------------------------------------------------------------
type tomldoc struct {
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
}

//--------------------------------------------------------------------------
func loadToml(filename string, timestamp time.Time) (Config, []FeedToml, error) {

	// todo: better handling of these objects (pointer?)
	var ()
	tomldoc := tomldoc{}
	tomldoc.Config.timestamp = timestamp
	tomldoc.Config.timestampStr = timestamp.Format("20060102_150405")
	tomldoc.Config.workspace = path.Dir(filename)

	file, err := os.Open(filename)
	if err != nil {
		log.Error("failed to open "+filename+": ", err)
		return tomldoc.Config, tomldoc.Feedlist, err
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		log.Error("readall '%v' failed: ", filename, err)
		return tomldoc.Config, tomldoc.Feedlist, err
	}

	if err := toml.Unmarshal(buf, &tomldoc); err != nil {
		log.Error("toml.unmarshal failed: ", err)
		return tomldoc.Config, tomldoc.Feedlist, err
	}

	// todo: move this??
	//------------------------------------- DEBUG -------------------------------------
	if tomldoc.Config.Debug {
		tomldoc.Config.timestampStr = "DEBUG"
	}
	//------------------------------------- DEBUG -------------------------------------

	//log.Debug(tomldoc)

	return tomldoc.Config, tomldoc.Feedlist, nil

}
