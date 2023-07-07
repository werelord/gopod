package pod

import (
	"errors"
	"fmt"
	"gopod/commandline"
	"gopod/podconfig"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

// todo: here

func Export(feedlist []*Feed) error {

	var reterr error

	// log.Debug("running export")

	// todo: export config
	// todo: create export dir

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

	if err := f.exportConfig(path); err != nil {
		reterr = errors.Join(reterr, err)
	} else if err := f.exportData(path); err != nil {
		reterr = errors.Join(reterr, err)
	}

	return reterr
}

func (f Feed) exportConfig(path string) error {
	var tomlFile = filepath.Join(path, fmt.Sprintf("%v.toml", f.Shortname))
	return podconfig.ExportToml(f.FeedToml, tomlFile)

}

func (f Feed) exportData(path string) error {

	var (
		opt = loadOptions{
			dontCreate:     true,
			includeXml:     true,
			direction:      cASC,
			includeDeleted: config.IncludeDeleted,
		}
		itemlist []*Item
	)
	if err := f.LoadDBFeed(opt); err != nil {
		return err
	} else if itemlist, err = f.loadDBFeedItems(AllItems, opt); err != nil {
		return err
	}

	log.Debug(itemlist)

	if config.ExportFormat == commandline.ExportJson {
		var filename = filepath.Join(path, fmt.Sprintf("%v.json", f.Shortname))
		return f.exportToJson(itemlist, filename)
	} else if config.ExportFormat == commandline.ExportDB {
		var filename = filepath.Join(path, fmt.Sprintf("%v.db", f.Shortname))
		return f.exportToDb(itemlist, filename)
	}

	return fmt.Errorf("export format not handled: %v", config.ExportFormat)
}

func (f Feed) exportToJson(items []*Item, file string) error {

	return errors.New("not yet implemented")
}

func (f Feed) exportToDb(items []*Item, file string) error {
	return errors.New("not yet implemented")
}
