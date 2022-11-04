package pod

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"gopod/podconfig"
	"gopod/podutils"

	log "github.com/sirupsen/logrus"
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

	numDups uint // number of dupiclates counted before skipping remaining items in xmlparse

	// itemMap is not explicitly exported, but converted to array to be exported
	// used mostly for checking update, as to when to quit
	itemMap map[string]*Item

	log *log.Entry
}

type FeedDBEntry struct {
	// anything that needs to be persisted between runs, go here
	PodDBModel
	Hash         string `gorm:"uniqueIndex"`
	DBShortname  string // just for db browsing
	EpisodeCount int
	XmlFeedData  FeedXmlDBEntry `gorm:"foreignKey:FeedId"`
	ItemList     []*ItemDBEntry `gorm:"foreignKey:FeedId"`
}

type FeedXmlDBEntry struct {
	PodDBModel
	FeedId                uint
	podutils.XChannelData `gorm:"embedded"`
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
	// todo: more error propegation

	f.log = log.WithField("feed", f.Shortname)

	// attempt create the dirs
	var xmlFilePath = filepath.Join(config.WorkspaceDir, f.Shortname, ".xml")
	if err := podutils.MkdirAll(xmlFilePath); err != nil {
		f.log.Error("error making xml directory: ", err)
		return err
	}
	f.xmlfile = filepath.Join(xmlFilePath, f.Shortname+"."+config.TimestampStr+".xml")

	f.mp3Path = filepath.Join(config.WorkspaceDir, f.Shortname)
	if err := podutils.MkdirAll(f.mp3Path); err != nil {
		f.log.Error("error making mp3 directory: ", err)
		return err
	}

	f.itemMap = make(map[string]*Item)

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) LoadDBFeed(includeXml bool) error {

	if db == nil {
		return errors.New("db is nil")
	} else if f.ID > 0 {
		// feed already initialized; run load XML directly
		if includeXml {
			return f.loadDBFeedXml()
		} else {
			// not loading xml, we're done
			return nil
		}
	}
	// make sure hash is prepopulated
	f.Hash = f.generateHash()
	if err := db.loadDBFeed(&f.FeedDBEntry, includeXml); err != nil {
		f.log.Error("failed loading feed: ", err)
		return err
	}
	// xml is loaded (if applicable) from above query, no reason to call explicitly
	f.log.Infof("{%v} feed loaded, id: %v, xml id: %v", f.Shortname, f.ID, f.XmlFeedData.ID)

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) loadDBFeedXml() error {

	if db == nil {
		return errors.New("db is nil")
	} else if f.ID == 0 {
		return fmt.Errorf("cannot load xml; feed '%v' itself not loaded", f.Shortname)
	} else if f.XmlFeedData.ID > 0 {
		// already loaded
		return nil
	}

	f.XmlFeedData.FeedId = f.ID
	if err := db.loadDBFeedXml(&f.XmlFeedData); err != nil {
		f.log.Error("failed loading feed xml: ", err)
		return err
	}

	f.log.Infof("{%v} feed xml loaded, id: %v, xml id: %v", f.Shortname, f.ID, f.XmlFeedData.ID)

	return nil
}

// --------------------------------------------------------------------------
func (f Feed) generateHash() string {
	return podutils.GenerateHash(f.Shortname)
}

// --------------------------------------------------------------------------
func (f *Feed) loadDBFeedItems(numItems int, includeXml bool, dtn direction) ([]*Item, error) {
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
	entryList, err = db.loadFeedItems(f.ID, numItems, includeXml, dtn)
	if err != nil {
		f.log.Error("Failed to get item data from db: ", err)
		return nil, err
	} else if len(entryList) == 0 {
		f.log.Warn("unable to get db entries; list is empty (new feed?)")
	}

	var itemList = make([]*Item, 0, len(entryList))

	for _, entry := range entryList {

		var item *Item
		if item, err = loadFromDBEntry(f.FeedToml, entry); err != nil {
			f.log.Error("failed to load item data: ", err)
			// if this fails, something is wrong
			return itemList, err
		}
		itemList = append(itemList, item)
		// f.log.Tracef("item:'%v':'%v'", item.PubTimeStamp.Format(podutils.TimeFormatStr), item.Filename)
	}

	return itemList, nil
}

// --------------------------------------------------------------------------
func (f Feed) saveDBFeed(newxml *podutils.XChannelData, newitems []*Item) error {

	// make sure we have an ID.. in loading, if this is a new feed, we're creating via FirstOrCreate
	if f.ID == 0 {
		return errors.New("unalbe to save to db; id is zero")
	}
	f.log.Infof("Saving db, xml:%v, itemCount:%v", newxml, len(newitems))

	if config.Simulate {
		f.log.Info("skipping saving database due to sim flag")
		return nil
	}

	// make sure hash and shortname is set
	f.Hash = f.generateHash()
	f.DBShortname = f.Shortname
	if newxml != nil {
		f.XmlFeedData.FeedId = f.ID
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

	f.log.Tracef("{%v} feed saved, id: %v, xml id: %v", f.Shortname, f.ID, f.XmlFeedData.ID)
	for _, i := range f.ItemList {
		f.log.Tracef("{%v} item saved, id: %v, xmlId: %v", i.Filename, i.ID, i.XmlData.ID)
	}

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) saveDBFeedItems(itemlist []*Item) error {
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
	} else if err := db.saveItems(commitList); err != nil {
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

	return db.deleteFeedItems(dbEntryList)
}

// --------------------------------------------------------------------------
func (f *Feed) Update() error {

	var (
		filelist map[string]*Item
		guidlist map[string]*Item
	)

	// make sure db is loaded
	if err := f.LoadDBFeed(true); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		return err
	} else {

		// because we're doing filename collisions and guid collisions, grab all items
		if itemList, err := f.loadDBFeedItems(-1, false, cDESC); err != nil {
			f.log.Error("failed to load item entries: ", err)
			return err
		} else {
			// associate items into map for update hashes
			filelist = make(map[string]*Item, len(itemList))
			guidlist = make(map[string]*Item, len(itemList))

			for _, item := range itemList {
				if _, exists := f.itemMap[item.Hash]; exists {
					f.log.Warn("Duplicate item found; wtf")
				}
				f.itemMap[item.Hash] = item

				// file name checking
				if _, exists := filelist[item.Filename]; exists {
					err := fmt.Errorf("duplicate filename '%v' found; need to run checkDownloads", item.Filename)
					f.log.Error(err)
					return err
				} else {
					filelist[item.Filename] = item
				}

				// guid checking
				if _, exists := guidlist[item.Guid]; exists {
					err := fmt.Errorf("duplicate guid '%v' found; need to run checkDownloads", item.Guid)
					f.log.Error(err)
					return err
				} else {
					filelist[item.Guid] = item
				}

			}
		}
	}

	f.log.Debug("Feed loaded from db for update: ", f.Shortname)

	var (
		body       []byte
		err        error
		newXmlData *podutils.XChannelData
		newItems   []*Item
	)

	if config.UseMostRecentXml {
		body, err = f.loadMostRecentXml()
	} else {
		body, err = f.downloadFeedXml()
	}
	if err != nil {
		f.log.Error("error in download: ", err)
		return err
	} else if len(body) == 0 {
		err = fmt.Errorf("body length is zero")
		f.log.Error(err)
		return err
	}

	// todo: break this up some more

	var itemPairList []podutils.ItemPair
	newXmlData, itemPairList, err = podutils.ParseXml(body, f)
	if err != nil {
		if errors.Is(err, podutils.ParseCanceledError{}) {
			f.log.Info("parse cancelled: ", err)
			return nil // this is not an error, just a shortcut to stop processing
		} else {
			// not canceled; some other error.. exit
			f.log.Error("failed to parse xml: ", err)
			// save the file (don't rotate) for future examination
			f.saveAndRotateXml(body, false)
			return err
		}
	}

	// if we're at this point, save the new channel data (buildDate or PubDate has changed)
	// don't rotate xml or save feed xml on using most recent
	if config.UseMostRecentXml == false {
		f.saveAndRotateXml(body, true)
	}

	// check url vs atom link & new feed url
	// TODO: handle this
	if f.XmlFeedData.AtomLinkSelf.Href != "" && f.Url != f.XmlFeedData.AtomLinkSelf.Href {
		f.log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XmlFeedData.AtomLinkSelf.Href)
	} else if f.XmlFeedData.NewFeedUrl != "" && f.Url != f.XmlFeedData.NewFeedUrl {
		f.log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XmlFeedData.NewFeedUrl)
	}

	if len(itemPairList) == 0 {
		f.log.Info("no items found to process")
		return nil
	}

	// list comes out newest (top of xml feed) to oldest.. reverse that,
	// go oldest to newest, to maintain item count
	for i := len(itemPairList) - 1; i >= 0; i-- {

		var (
			hash      = itemPairList[i].Hash
			xmldata   = itemPairList[i].ItemData
			itemEntry *Item
			exists    bool
		)

		if itemEntry, exists = f.itemMap[hash]; exists {
			// this should only happen when force == true; log warning if this is not the case
			if config.ForceUpdate == false {
				f.log.Warn("hash for new item already exists and --force is not set; something is seriously wrong")
			}
			// we don't necessarily want to create and replace;
			// just update the new data in the existing entry

			// replace the existing xml data
			// todo: deep copy comparison
			itemEntry.updateXmlData(hash, xmldata)

		} else if itemEntry, err = createNewItemEntry(f.FeedToml, hash, xmldata, f.EpisodeCount+1); err != nil {
			f.log.Error("failed creating new item entry; skipping: ", err)
			continue
		} else {
			// new item added; save the episode count
			f.EpisodeCount++
		}

		f.log.Infof("item added: :%+v", itemEntry)

		// add it to the entry list
		f.itemMap[hash] = itemEntry

		// add it to the new items needing processing
		newItems = append([]*Item{itemEntry}, newItems...)
	}

	// todo: more error checking here
	// todo: need to check filename collissions
	// todo: check guid collisions, since fuckers just replace the url.. we can't trust these damn generators..

	// process new entries
	// todo: move this outside update (likely on goroutine implementation)

	if errlist := f.processNew(newItems); len(errlist) > 0 {
		f.log.Error("errors in processing new items:\n")
		for _, err := range errlist {
			f.log.Errorf("%v", err)
		}
		return errlist[0]
	}

	// save everything here, as processing is done.. any errors should have exited out at some point
	// inserting everything into feed db; by assoc should save everything
	if err = f.saveDBFeed(newXmlData, newItems); err != nil {
		f.log.Error("saving db failed: ", err)
		return err
	}

	f.log.Debugf("{%v} done processing feed", f.Shortname)
	return nil
}

// --------------------------------------------------------------------------
func (f Feed) downloadFeedXml() (body []byte, err error) {
	// download file
	if body, err = podutils.Download(f.Url); err != nil {
		f.log.Error("failed to download: ", err)
	}
	return
}

// --------------------------------------------------------------------------
func (f Feed) loadMostRecentXml() (body []byte, err error) {
	// find the most recent xml based on the glob pattern
	filename, err := podutils.FindMostRecent(filepath.Dir(f.xmlfile), fmt.Sprintf("%v.*.xml", f.Shortname))
	if err != nil {
		return nil, err
	}
	f.log.Debug("loading xml file: ", filename)

	return podutils.LoadFile(filename)
}

// --------------------------------------------------------------------------
func (f Feed) saveAndRotateXml(body []byte, shouldRotate bool) {
	// for external reference
	if err := podutils.SaveToFile(body, f.xmlfile); err != nil {
		f.log.Error("failed saving xml file: ", err)
		// not exiting; not a fatal error as the parsing happens on the byte string
	} else if shouldRotate && config.XmlFilesRetained > 0 {
		f.log.Debug("rotating xml files..")
		podutils.RotateFiles(filepath.Dir(f.xmlfile),
			fmt.Sprintf("%v.*.xml", f.Shortname),
			uint(config.XmlFilesRetained))
	}
}

//--------------------------------------------------------------------------
// feedProcess implementation
//--------------------------------------------------------------------------

// --------------------------------------------------------------------------
func (f *Feed) SkipParsingItem(hash string) (skip bool, cancelRemaining bool) {

	if config.ForceUpdate {
		return false, false
	}

	// assume itemlist has been populated with enough entries (if not all)
	// todo: is this safe to assume?? any way we can check??
	_, skip = f.itemMap[hash]

	if (config.MaxDupChecks >= 0) && (skip == true) {
		f.numDups++
		cancelRemaining = (f.numDups >= uint(config.MaxDupChecks))
	}
	return
}

// --------------------------------------------------------------------------
// returns true if parsing should halt on pub date; parse returns ParseCanceledError on true
func (f Feed) CancelOnPubDate(xmlPubDate time.Time) (cont bool) {

	if config.ForceUpdate {
		return false
	}

	//f.log.Tracef("Checking build date; \nFeed.Pubdate:'%v' \nxmlBuildDate:'%v'", f.XMLFeedData.PubDate.Unix(), xmlPubDate.Unix())
	if f.XmlFeedData.PubDate.IsZero() == false {
		if xmlPubDate.After(f.XmlFeedData.PubDate) == false {
			f.log.Info("new pub date not after previous; cancelling parse")
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
// returns true if parsing should halt on build date; parse returns ParseCanceledError on true
func (f Feed) CancelOnBuildDate(xmlBuildDate time.Time) (cont bool) {

	if config.ForceUpdate {
		return false
	}

	//f.log.Tracef("Checking build date; Feed.LastBuildDate:'%v', xmlBuildDate:'%v'", f.XMLFeedData.LastBuildDate, xmlBuildDate)
	if f.XmlFeedData.LastBuildDate.IsZero() == false {
		if xmlBuildDate.After(f.XmlFeedData.LastBuildDate) == false {
			f.log.Info("new build date not after previous; cancelling parse")
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
func (f Feed) CalcItemHash(guid string, url string) (string, error) {
	// within item
	return calcHash(guid, url, f.UrlParse)
}

// --------------------------------------------------------------------------
func (f *Feed) processNew(newItems []*Item) []error {

	var errList = make([]error, 0)

	if len(newItems) == 0 {
		f.log.Info("no items to process; item list is empty")
		return nil
	}

	// todo: move download handling within item
	for _, item := range newItems {
		f.log.Debugf("processing new item: {%v %v}", item.Filename, item.Hash)

		podfile := filepath.Join(f.mp3Path, item.Filename)
		var fileExists bool

		fileExists, err := podutils.FileExists(podfile)
		if err != nil {
			f.log.Warn("error in FileExists: ", err)
			errList = append(errList, err)
		}

		// todo: check downloaded & archive flag

		if item.Downloaded == true {
			f.log.Warn("skipping entry; file already downloaded.. possible filename collision?")
			continue
		} else if fileExists == true {
			f.log.Warn("downloaded == false, but file already exists.. possible filename collision?")
			//item.Downloaded = true
			continue
		}
		if config.Simulate {
			f.log.Info("skipping downloading file due to sim flag")
			continue
		}
		if err = item.Download(f.mp3Path); err != nil {
			f.log.Error("Error downloading file: ", err)
			errList = append(errList, err)
		}
		f.log.Info("finished processing file: ", podfile)
	}

	f.log.Info("all new downloads completed")
	return errList
}
