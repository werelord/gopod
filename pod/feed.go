package pod

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	log "gopod/multilogger"
	"gopod/podconfig"
	"gopod/podutils"
)

// --------------------------------------------------------------------------
type Feed struct {
	// toml information, extracted from config
	podconfig.FeedToml

	// internal, local to feed, not serialized (explicitly)
	feedInternal

	// all db entities, exported
	FeedDBEntry
}

type feedInternal struct {
	// local items, not exported to database

	xmlfile string
	mp3Path string

	log log.Logger
}

type FeedDBEntry struct {
	// anything that needs to be persisted between runs, go here
	PodDBModel
	Hash         string `gorm:"uniqueIndex"`
	DBShortname  string // just for db browsing
	EpisodeCount int
	XmlId        uint
	XmlFeedData  *FeedXmlDBEntry `gorm:"foreignKey:XmlId"`
	ItemList     []*ItemDBEntry  `gorm:"foreignKey:FeedId"`
	ImageId      uint
	ImageData    *ImageDBEntry   `gorm:"foreignKey:ImageId"`
	ImageList    []*ImageDBEntry `gorm:"foreignKey:FeedId"`
	imageMap     map[string]*ImageDBEntry
}

type FeedXmlDBEntry struct {
	PodDBModel
	podutils.XChannelData `gorm:"embedded"`
}

type ImageDBEntry struct {
	PodDBModel
	FeedId       uint
	Filename     string
	LastModified time.Time
	Url          string
	// Hash         string
}

var (
	config *podconfig.Config
	db     *PodDB
)

// func (f Feed) Format(fs fmt.State, c rune) {
// 	fs.Write([]byte("Name:" + f.Shortname + " url: " + f.Url))
// }

// init package global vars
func Init(cfg *podconfig.Config, pdb *PodDB) {
	// nil checking will happen in NewFeed init
	config = cfg
	db = pdb
}

// --------------------------------------------------------------------------
func NewFeed(feedToml podconfig.FeedToml) (*Feed, error) {
	var feed = Feed{FeedToml: feedToml}

	if err := feed.initFeed(); err != nil {
		return nil, fmt.Errorf("failed to init feed '%v': %w", feed.Name, err)
	}
	return &feed, nil
}

// --------------------------------------------------------------------------
func (f *Feed) initFeed() error {
	// make sure stuff is set
	if config == nil {
		return errors.New("config is nil; make sure set thru Init()")
	} else if db == nil {
		return errors.New("db is nil; make sure set thru Init()")
	}

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	f.log = log.With("feed", f.Shortname)

	// attempt create the dirs
	var xmlFilePath = filepath.Join(config.WorkspaceDir, f.Shortname, ".xml")
	if err := podutils.MkdirAll(xmlFilePath); err != nil {
		f.log.Errorf("error making xml directory: %v", err)
		return err
	}
	f.xmlfile = filepath.Join(xmlFilePath, f.Shortname+"."+config.TimestampStr+".xml")

	f.mp3Path = filepath.Join(config.WorkspaceDir, f.Shortname)
	if err := podutils.MkdirAll(f.mp3Path); err != nil {
		f.log.Errorf("error making mp3 directory: %v", err)
		return err
	}

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) LoadDBFeed(opt loadOptions) error {

	if db == nil {
		return errors.New("db is nil")
	} else if f.ID > 0 {
		// feed already initialized; run load XML directly
		if opt.includeXml {
			return f.loadDBFeedXml()
		} else {
			// not loading xml, we're done
			return nil
		}
	}
	// make sure hash is prepopulated
	f.generateHash()

	if opt.includeDeleted == false {
		// check for feed deleted before loading
		if _, err := db.isFeedDeleted(f.Hash); err != nil {
			//f.log.Error("error in checking deleted: ", err)
			err := fmt.Errorf("cannot load feed; %w", err)
			f.log.Error(err)
			return err
		}
	}

	if err := db.loadFeed(&f.FeedDBEntry, opt); err != nil {
		f.log.Errorf("failed loading feed: %v", err)
		return err
	}

	// xml is loaded (if applicable) from above query, no reason to call explicitly
	{
		lg := f.log.With("id", f.ID)
		if opt.includeXml && f.XmlFeedData != nil {
			lg = lg.With("xml id", f.XmlFeedData.ID)
		}
		lg.Info("feed loaded")
	}

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) loadDBFeedXml() error {

	if db == nil {
		return errors.New("db is nil")
	} else if f.ID == 0 {
		return fmt.Errorf("cannot load xml; feed '%v' itself not loaded", f.Shortname)
	} else if f.XmlId == 0 {
		return fmt.Errorf("cannot load xml; xml id in '%v' is zero", f.Shortname)
	} else if f.XmlFeedData != nil && f.XmlFeedData.ID > 0 {
		// already loaded
		return nil
	}

	if xml, err := db.loadDBFeedXml(f.XmlId); err != nil {
		f.log.Errorf("failed loading feed xml: %v", err)
		return err
	} else {
		f.XmlFeedData = xml
	}

	f.log.With("id", f.ID, "xmlId", f.XmlFeedData.ID).Info("feed xml loaded")

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) generateHash() {
	if f.Hash == "" {
		f.Hash = podutils.GenerateHash(f.Shortname)
	}
}

// --------------------------------------------------------------------------
func (f *Feed) loadDBFeedItems(numItems int, opt loadOptions) ([]*Item, error) {
	var (
		err       error
		entryList []*ItemDBEntry
	)
	// lets not load feed here; return error if feed is not loaded
	if f.ID == 0 {
		return nil, errors.New("feed id is zero")
	}

	// load itemlist.. if numitems is negative, load everything..
	// otherwise limit to numLatest
	entryList, err = db.loadFeedItems(f.ID, numItems, opt)
	if err != nil {
		f.log.Errorf("Failed to get item data from db: %v", err)
		return nil, err
	} else if len(entryList) == 0 {
		f.log.Warn("unable to get db entries; list is empty (new feed?)")
	}

	var itemList = make([]*Item, 0, len(entryList))

	for _, entry := range entryList {

		var item *Item
		if item, err = loadFromDBEntry(f.FeedToml, entry); err != nil {
			f.log.Errorf("failed to load item data: %v", err)
			// if this fails, something is wrong
			return itemList, err
		}
		itemList = append(itemList, item)
		// f.log.Tracef("item:'%v':'%v'", item.PubTimeStamp.Format(podutils.TimeFormatStr), item.Filename)
	}

	return itemList, nil
}

// --------------------------------------------------------------------------
func (f *Feed) saveDBFeed(newxml *podutils.XChannelData, newitems []*Item) error {

	// make sure we have an ID.. in loading, if this is a new feed, we're creating via FirstOrCreate
	if f.ID == 0 {
		return errors.New("unable to save to db; id is zero")
	}
	f.log.Infof("Saving db, xml:%v, itemCount:%v", newxml.Title, len(newitems))

	if config.Simulate {
		f.log.Info("skipping saving database due to sim flag")
		return nil
	}
	// inserting everything into feed db; by full assoc should save everything

	// make sure hash and shortname is set
	f.generateHash()
	f.DBShortname = f.Shortname
	if newxml != nil {
		if (f.XmlId != 0) && (f.XmlFeedData == nil) {
			return errors.New("feed xml is not loaded; make sure it is loaded before saving new xml")
		} else if (f.XmlId == 0) && (f.XmlFeedData == nil) {
			f.log.Debug("xml id is 0, and feed data is nil; new feed xml detected")
			f.XmlFeedData = &FeedXmlDBEntry{}
		}
		// straight replace, keeping same id if not new
		f.XmlFeedData.XChannelData = *newxml
	}

	if len(newitems) > 0 {
		if entryList, err := f.genItemDBEntryList(newitems); err != nil {
			return err
		} else {
			f.ItemList = entryList
		}
	}

	if err := db.saveFeed(&f.FeedDBEntry); err != nil {
		f.log.Errorf("error saving feed db: %v", err)
		return err
	}

	fl := f.log.With("id", f.ID)
	if f.XmlFeedData != nil {
		fl = fl.With("xmlid", f.XmlFeedData.ID)
	}
	fl.Debug("feed saved")

	for _, i := range f.ItemList {

		il := f.log.With("itemFilename", i.Filename, "itemId", i.ID)
		if i.XmlData != nil {
			il = il.With("itemXmlId", i.XmlData.ID)
		}
		il.Debug("item saved")
	}

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) saveDBFeedItems(itemlist ...*Item) error {
	// make sure we have an ID.. in loading, if this is a new feed, we're creating via FirstOrCreate
	if f.ID == 0 {
		return errors.New("unable to save to db; feed id is zero")
	} else if len(itemlist) == 0 {
		f.log.Warn("not saving items; length is zero")
		return nil
	}
	f.log.Infof("Saving db items, itemCount:%v", len(itemlist))

	if config.Simulate {
		f.log.Info("skipping saving database due to sim flag")
		return nil
	}

	if commitList, err := f.genItemDBEntryList(itemlist); err != nil {
		return err
	} else if err := db.saveItems(commitList...); err != nil {
		return err
	}
	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) genItemDBEntryList(itemlist []*Item) ([]*ItemDBEntry, error) {

	if f.ID == 0 {
		return nil, errors.New("unable to save to db; feed id is zero")
	} else if len(itemlist) == 0 {
		return nil, errors.New("empty list")
	}
	var ret = make([]*ItemDBEntry, 0, len(itemlist))

	for _, item := range itemlist {
		if item.Hash == "" {
			return nil, fmt.Errorf("hash is empty for item: %v", item.Filename)
		}
		if item.FeedId > 0 && item.FeedId != f.ID {
			return nil, fmt.Errorf("item feed id is set(%v), and does not match feed id (%v): %v", item.FeedId, f.ID, item.Filename)
		} else /*if item.FeedId == 0 */ {
			item.FeedId = f.ID
		}
		ret = append(ret, &item.ItemDBEntry)
	}

	return ret, nil
}

// --------------------------------------------------------------------------
func (f Feed) delete() error {
	if db == nil {
		return errors.New("db is nil")
	}
	f.log.Debug("deleting feed (soft delete)")

	// will do a recursive delete of entry, xml, items and itemxml
	return db.deleteFeed(&f.FeedDBEntry)
}

// --------------------------------------------------------------------------
func (f Feed) deleteFeedItems(list []*Item) error {
	if db == nil {
		return errors.New("db is nil")
	}
	f.log.Debugf("deleting items; len: %v", len(list))
	// anything else needed here??
	var dbEntryList = make([]*ItemDBEntry, 0, len(list))
	for _, item := range list {
		dbEntryList = append(dbEntryList, &item.ItemDBEntry)
	}

	return db.deleteItems(dbEntryList)
}
