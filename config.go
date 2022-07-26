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
	MaxDupChecks uint `toml:"maxdupchecks"`
	// todo: change default
	Workspace    string
	Timestamp    time.Time
	TimestampStr string
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
func loadToml(filename string, timestamp time.Time) (*Config, map[string]*Feed, error) {

	// todo: better handling of these objects (pointer?)
	tomldoc := tomldocImport{}
	tomldoc.Config.Timestamp = timestamp
	tomldoc.Config.TimestampStr = timestamp.Format("20060102_150405")
	tomldoc.Config.Workspace = path.Dir(filename)

	file, err := os.Open(filename)
	if err != nil {
		log.Error("failed to open "+filename+": ", err)
		return nil, nil, err
	}
	defer file.Close()

	buf, err := io.ReadAll(file)
	if err != nil {
		log.Error("readall '%v' failed: ", filename, err)
		return nil, nil, err
	}

	if err := toml.Unmarshal(buf, &tomldoc); err != nil {
		log.Error("toml.unmarshal failed: ", err)
		return nil, nil, err
	}

	// todo: move this??
	//------------------------------------- DEBUG -------------------------------------
	if cmdline.Debug {
		tomldoc.Config.TimestampStr = "DEBUG"
	}
	//------------------------------------- DEBUG -------------------------------------

	// move feedlist into shortname map
	feedMap := make(map[string]*Feed)
	for _, feedtoml := range tomldoc.Feedlist {
		f := NewFeed(&tomldoc.Config, feedtoml)
		feedMap[f.Shortname] = f
	}

	return &tomldoc.Config, feedMap, nil

}
