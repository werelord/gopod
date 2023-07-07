package podconfig

import (
	"errors"
	"fmt"
	"gopod/commandline"
	"gopod/podutils"
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
type FeedToml struct {
	Name              string  `toml:"name"`
	Shortname         string  `toml:"shortname"`
	Url               string  `toml:"url"`
	FilenameParse     string  `toml:"filenameParse,omitempty"`
	Regex             string  `toml:"regex,omitempty"`
	UrlParse          string  `toml:"urlParse,omitempty"`
	CleanRep          *string `toml:"cleanReplacement,omitempty"`
	EpisodePad        int     `toml:"episodePad,omitempty"`
	CountStart        int     `toml:"countStart,omitempty"`
	DupFilenameBypass string  `toml:"dupFilenameBypass,omitempty"`
}

// --------------------------------------------------------------------------
func LoadToml(filename string, timestamp time.Time) (*Config, []FeedToml, error) {

	var tomldoc = struct {
		Config   Config     `toml:"config"`
		Feedlist []FeedToml `toml:"feed"`
	}{}

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

	var decoder = toml.NewDecoder(file).DisallowUnknownFields()
	if err := decoder.Decode(&tomldoc); err != nil {
		// if err := toml.Unmarshal(buf, &tomldoc); err != nil {
		var details *toml.StrictMissingError

		if errors.As(err, &details) {
			return nil, nil, fmt.Errorf("toml.Decode failed: %w\ndetails: %v", details, details.String())
		}
		return nil, nil, fmt.Errorf("toml.Decode failed: %w", err)
	}

	return &tomldoc.Config, tomldoc.Feedlist, nil

}

func ExportToml(feed FeedToml, file string) error {

	// match import config, without the config section
	type exportDoc struct {
		Feedlist []FeedToml `toml:"feed"`
	}

	var exportData = exportDoc{Feedlist: make([]FeedToml, 0)}

	exportData.Feedlist = append(exportData.Feedlist, feed)

	out, err := os.Create(file)
	if err != nil {
		return err
	}

	enc := toml.NewEncoder(out)
	if err := enc.Encode(exportData); err != nil {
		return err
	}

	return nil
}
