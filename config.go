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
	FilenameRegex string `toml:"filenameRegex"`
	UrlParse      string `toml:"urlParse"`
}

//--------------------------------------------------------------------------
func loadToml(filename string, timestamp time.Time) (Config, []FeedToml) {

	var ()
	tomldoc := tomldoc{}
	tomldoc.Config.timestamp = timestamp
	tomldoc.Config.timestampStr = timestamp.Format("20060102_150405")
	tomldoc.Config.workspace = path.Dir(filename)

	file, err := os.Open(filename)
	if err != nil {
		log.Error("failed to open "+filename+": ", err)
		return tomldoc.Config, tomldoc.Feedlist
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		log.Error("failed to open "+filename+": ", err)
		return tomldoc.Config, tomldoc.Feedlist
	}

	err1 := toml.Unmarshal(buf, &tomldoc)
	if err1 != nil {
		log.Error("failed to open "+filename+": ", err)
		return tomldoc.Config, tomldoc.Feedlist
	}

	// todo: move this??
	//------------------------------------- DEBUG -------------------------------------
	if tomldoc.Config.Debug {
		tomldoc.Config.timestampStr = "DEBUG"
	}
	//------------------------------------- DEBUG -------------------------------------

	//log.Debug(tomldoc)

	return tomldoc.Config, tomldoc.Feedlist

}
