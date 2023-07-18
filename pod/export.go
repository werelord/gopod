package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopod/commandline"
	"gopod/podconfig"
	"gopod/podutils"

	log "github.com/sirupsen/logrus"
)

func Export(feedlist []*Feed) error {

	var reterr error

	if config == nil {
		return errors.New("cannot export feeds; config is nil")
	} else if db == nil {
		return errors.New("cannot export feeds; db is nil")
	}

	for _, feed := range feedlist {
		var expPath = config.ExportPath
		if expPath == "" {
			expPath = filepath.Join(config.WorkspaceDir, feed.Shortname)
		}
		if err := feed.export(expPath); err != nil {
			reterr = errors.Join(reterr, err)
		}
	}

	return reterr
}

func (f *Feed) export(path string) error {

	f.log.Debug("running export")

	var reterr error

	if config.ExportPath != "" {
		podutils.MkdirAll(config.ExportPath)
	}

	if err := f.exportConfig(path); err != nil {
		reterr = errors.Join(reterr, err)
	} else if err := f.exportData(path); err != nil {
		reterr = errors.Join(reterr, err)
	}

	return reterr
}

func (f Feed) exportConfig(path string) error {
	var tomlFile = filepath.Join(path, fmt.Sprintf("%v.toml", f.Shortname))

	if err := podconfig.ExportToml(f.FeedToml, tomlFile); err != nil {
		return err
	} else {
		f.log.Infof("config exported to %v", tomlFile)
		return nil
	}
}

func (f Feed) exportData(path string) error {

	var (
		opt = loadOptions{
			dontCreate:     true,
			includeXml:     true,
			direction:      cASC,
			includeDeleted: config.IncludeDeleted,
		}
	)
	if err := f.LoadDBFeed(opt); err != nil {
		return err
	} else if itemlist, err := f.loadDBFeedItems(AllItems, opt); err != nil {
		return err
	} else if f.ItemList, err = f.genItemDBEntryList(itemlist); err != nil {
		return err
	}

	if config.ExportFormat == commandline.ExportJson {
		var filename = filepath.Join(path, fmt.Sprintf("%v.json", f.Shortname))
		return exportToJson(&f.FeedDBEntry, filename)
	} else if config.ExportFormat == commandline.ExportDB {
		var filename = filepath.Join(path, fmt.Sprintf("%v.db", f.Shortname))
		return exportToDb(&f.FeedDBEntry, filename)
	}

	return fmt.Errorf("export format not handled: %v", config.ExportFormat)
}

func exportToJson(feed *FeedDBEntry, file string) error {

	var lg = log.WithField("feed", feed.DBShortname)

	lg.Debugf("exporting feed to %v", file)

	out, err := os.Create(file)
	if err != nil {
		return err
	}
	defer out.Close()

	enc := json.NewEncoder(out)
	enc.SetIndent("", "    ")
	enc.SetEscapeHTML(true)

	if err := enc.Encode(feed); err != nil {
		return err
	}

	lg.Infof("feed exported to '%v'", file)
	return nil

}

func exportToDb(feed *FeedDBEntry, file string) error {

	var lg = log.WithField("feed", feed.DBShortname)

	lg.Debugf("exporting feed to %v", file)

	if export, err := NewDB(file); err != nil {
		return err
	} else {
		// export with the already given IDs from the master db
		if err := export.saveFeed(feed); err != nil {
			return err
		}
	}

	lg.Infof("feed exported to '%v'", file)
	return nil
}
