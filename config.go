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
	// nothing yet
	Foo   bool `toml:"foo"`
	Debug bool `toml:"debug"`
	MaxDupChecks int `toml:"maxdupchecks"`
	// todo: change default
	workspace    string
	timestamp    time.Time
	timestampStr string
}

//--------------------------------------------------------------------------
type tomldoc struct {
	Config   Config `toml:"config"`
	Feedlist []Feed `toml:"feed"`
}

//--------------------------------------------------------------------------
func loadToml(filename string) (Config, []Feed) {

	var ()
	tomldoc := tomldoc{}
	tomldoc.Config.timestamp = time.Now()
	tomldoc.Config.timestampStr = tomldoc.Config.timestamp.Format("20060102_150405")
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
	if tomldoc.Config.Debug {
		tomldoc.Config.timestampStr = "DEBUG"
	}

	//log.Debug(tomldoc)

	return tomldoc.Config, tomldoc.Feedlist

}
