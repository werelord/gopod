package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"time"

	log "gopod/multilogger"
)

type archType struct {
	feed *Feed
	log  log.Logger

	year   int
	path   string
	items  []*Item
	images map[string]*ImageDBEntry
}

// --------------------------------------------------------------------------
func Archive(feeds ...*Feed) error {

	var err error

	for _, feed := range feeds {
		feed.log.Info("running archive")
		if e := archive(feed); e != nil {
			feed.log.Error("error archiving feed", "err", e)
			errors.Join(err, e)
		}
	}

	return err
}

// --------------------------------------------------------------------------
func archive(f *Feed) error {
	var (
		log     = f.log
		options = loadOptions{
			dontCreate:     true,
			includeXml:     true,
			includeDeleted: false,
			direction:      cASC,
		}
		itemlist   []*Item
		archiveMap = make(map[int]*archType, 0)
		reterr     error

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
			arc, exists := archiveMap[itemYear]
			if exists == false {
				arc = &archType{
					feed:   f,
					log:    f.log.With("year", itemYear),
					year:   itemYear,
					path:   filepath.Join(f.archivePath, fmt.Sprintf("%s.%d", f.Shortname, itemYear)),
					items:  make([]*Item, 0),
					images: make(map[string]*ImageDBEntry, 0),
				}
				archiveMap[itemYear] = arc
			}
			arc.items = append(arc.items, item)
			if item.ImageKey != "" && item.ImageKey != f.ImageKey {	// don't back up feed's image
				if img, exists := f.imageMap[item.ImageKey]; exists == false {
					arc.log.Error("image with key doesn't exist in db", "key", item.ImageKey)
					// skip any image processing
				} else {
					arc.images[item.ImageKey] = img
				}
			}
		}
	}

	if len(archiveMap) <= 0 {
		f.log.Info("nothing found to archive")
		return nil
	}

	for _, arc := range archiveMap {
		log.Debug("archive step", "path", arc.path, "items", len(arc.items), "images", len(arc.images))
		if err := arc.archiveYear(); err != nil {
			arc.log.Error(err)
			reterr = errors.Join(reterr, err)
		} else {
			// saving archive status in base database
			arc.log.Debug("saving items")
			if err := f.saveDBFeedItems(arc.items...); err != nil {
				arc.log.Error("error saving items: %w", err)
				reterr = errors.Join(reterr, err)
			}
			if len(arc.images) > 0 {
				arc.log.Debug("saving images")
				if err := f.saveDBFeedImages(arc.images); err != nil {
					arc.log.Error(err)
					reterr = errors.Join(reterr, err)
				}
			}
		}
	}

	return reterr
}

func (arc archType) archiveYear() error {

	var (
		reterr  error
		imgPath = filepath.Join(arc.path, ".img")
	)

	// solely for making directories; full simulation checks done on each individual item/image
	if config.Simulate == false {
		if err := podutils.MkdirAll(arc.path); err != nil {
			arc.log.Error("error creating archive path", err)
			return err
		}

		if len(arc.images) > 0 {
			if err := podutils.MkdirAll(imgPath); err != nil {
				arc.log.Error("error creating archive image path", err)
				return err
			}
		}
	}

	for _, item := range arc.items {
		if err := arc.archiveItem(item); err != nil {
			arc.log.Error(err)
			reterr = errors.Join(reterr, err)
		}
	}

	for _, img := range arc.images {
		if err := arc.archiveImage(img); err != nil {
			arc.log.Error(err)
			reterr = errors.Join(reterr, err)
		}
	}

	if reterr != nil {
		arc.log.Warn("not saving archive db due to errors")
	} else if err := arc.createArchiveDb(); err != nil {
		arc.log.Error(err)
		return err
	}
	return reterr
}

// --------------------------------------------------------------------------
func (arc archType) archiveItem(item *Item) error {
	// move items; mark as archived
	var (
		log = arc.log
		src = filepath.Join(arc.feed.mp3Path, item.Filename)
		dst = filepath.Join(arc.path, item.Filename)
	)

	if config.Simulate {
		log.Infof("not archiving due to simulate flag '%v'", item.Filename)
		return nil
	} else {
		log.Infof("archiving '%v'", item.Filename)
	}

	if exists, err := podutils.FileExists(src); err != nil {
		log.Error("error checking file exists", err)
		return err
	} else if exists == false {
		err = fmt.Errorf("item to be archived but file does not exists: %v", item.Filename)
		return err
	} else if err := podutils.Rename(src, dst); err != nil {
		log.Error("error moving file", err)
		return err
	} else {
		// item is archived; mark it as such
		item.Archived = true
		return nil
	}
}

func (arc archType) archiveImage(img *ImageDBEntry) error {
	var (
		log = arc.log
		src = filepath.Join(arc.feed.imgPath, img.Filename)
		dst = filepath.Join(arc.path, ".img", img.Filename)
	)

	if config.Simulate {
		log.Infof("not archiving due to simulate flag '%v'", img.Filename)
		return nil
	} else {
		log.Infof("archiving '%v'", img.Filename)
	}

	// if the image is already archived in another item, don't do processing anymore
	// due to using pointers everywhere, this should hold true if it exists in another year's archive
	if img.Archived {
		log.Debug("image already archived", "imgfilename", img.Filename)
		return nil
	}
	if exists, err := podutils.FileExists(src); err != nil {
		log.Error("error checking file exists", err)
		return err
	} else if exists == false {
		// with images, the file may have been archived in a previous year; just warn on this
		log.Warn("image to be archived but file does not exists", "imgFilename", img.Filename)
		return nil
	} else if err := podutils.Rename(src, dst); err != nil {
		log.Error("error moving file", err)
		return err
	} else {
		// item is archived; mark it as such
		img.Archived = true
		return nil
	}
}

// --------------------------------------------------------------------------
func (arc archType) createArchiveDb() error {
	// save archive db with feed, archived items, archived images

	var (
		log    = arc.log
		dbfile = filepath.Join(arc.path, ".db", fmt.Sprintf("%s.db", arc.feed.Shortname))
		// create a separate copy of the feed
		arcFeed = *arc.feed
	)

	if config.Simulate {
		log.Info("not saving db due to simulate flag")
		return nil
	} else {
		log.Info("saving archive db")
	}

	if dbarc, err := NewDB(dbfile); err != nil {
		log.Error(err)
		return err
	} else if entrylist, err := arc.feed.genItemDBEntryList(arc.items); err != nil { // set the new feed items
		log.Error(err)
		return err
	} else {
		arcFeed.ItemList = entrylist
		arcFeed.imageMap = arc.images
		if err := dbarc.saveFeed(&arcFeed.FeedDBEntry); err != nil {
			log.Errorf("error saving feed db: %v", err)
			return err
		}
	}

	return nil
}
