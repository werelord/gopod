package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
)

// --------------------------------------------------------------------------
func Archive(feeds ...*Feed) error {

	var err error

	for _, feed := range feeds {
		feed.log.Info("running archive")
		if e := feed.archive(); e != nil {
			feed.log.Error("error archiving feed", "err", e)
			errors.Join(err, e)
		}
	}

	return err
}

// --------------------------------------------------------------------------
func (f *Feed) archive() error {
	var (
		log     = f.log
		options = loadOptions{
			dontCreate:     true,
			includeXml:     true,
			includeDeleted: false,
			direction:      cASC,
		}
		itemlist     []*Item
		archivedList = make([]*Item, 0)
		reterr       error

		// by default, archive every item not in current year
		currentYear, _, _ = time.Now().Date()
	)

	if err := f.LoadDBFeed(options); err != nil {
		log.Error("error loading feed from db", "err", err)
		return err
		// todo: restrict to archive == false
	} else if il, err := f.loadDBFeedItems(AllItems, options); err != nil {
		log.Error("error loading items from db", "err", err)
		return err
	} else {
		itemlist = il
	}

	for _, item := range itemlist {
		// skip if already arhived
		if item.Archived {
			continue
		}

		var itemYear, _, _ = item.PubTimeStamp.Date()
		if itemYear < currentYear {
			// mark for archive
			if err := f.archiveItem(item, itemYear); err != nil {
				log.Errorf("error archiving item: %w", err)
				reterr = errors.Join(reterr, err)
			} else {
				archivedList = append(archivedList, item)
			}
		}
	}

	// any errors won't be incuded in archived list, so just dump the entire list
	if len(archivedList) > 0 {
		if err := f.saveDBFeedItems(archivedList...); err != nil {
			log.Error("error saving items: %w", err)
			reterr = errors.Join(err)
		} else {
			log.Info("archive competed")
		}
	} else {
		log.Info("nothing to archive")
	}

	return reterr
}

// --------------------------------------------------------------------------
func (f *Feed) archiveItem(item *Item, itemYear int) error {

	if config.Simulate {
		log.Infof("not archiving due to simulate flag '%v' (%v)", item.Filename, itemYear)
		return nil
	} else {
		log.Infof("archiving '%v' (%v)", item.Filename, itemYear)

	}

	var archivePath = filepath.Join(f.archivePath, fmt.Sprintf("%s.%d", f.Shortname, itemYear))

	if err := podutils.MkdirAll(archivePath); err != nil {
		log.Error("error creating archive path", err)
		return err
	} else if exists, err := podutils.FileExists(filepath.Join(f.mp3Path, item.Filename)); err != nil {
		log.Error("error checking file exists", err)
		return err
	} else if exists == false {
		err = fmt.Errorf("item to be archived but file does not exists: %v", item.Filename)
		return err
	} else if err := podutils.Rename(filepath.Join(f.mp3Path, item.Filename), filepath.Join(archivePath, item.Filename)); err != nil {
		log.Error("error moving file", err)
		return err
	} else {
		// item is archived; mark it as such
		item.Archived = true
		return nil
	}
}
