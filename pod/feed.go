package pod

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"gopod/podconfig"
	"gopod/podutils"

	log "github.com/sirupsen/logrus"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// --------------------------------------------------------------------------
type Feed struct {
	// toml information, extracted from config
	podconfig.FeedToml

	// todo: archive flag

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
	Hash        string         `gorm:"uniqueIndex"`
	DBShortname string         // just for db browsing
	XmlFeedData FeedXmlDBEntry `gorm:"foreignKey:FeedId"`
	ItemList    []*ItemDBEntry `gorm:"foreignKey:FeedId"`
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

// eventually remove
// func GenerateHash(f Feed) string {
// 	return f.generateHash()
// }

// --------------------------------------------------------------------------
func (f Feed) generateHash() string {
	return podutils.GenerateHash(f.Shortname)
}

// --------------------------------------------------------------------------
func (f *Feed) loadDBFeedItems(numLatest int, includeXml bool) ([]*Item, error) {
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
	entryList, err = db.loadFeedItems(f.ID, numLatest, includeXml)
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

	// TODO: separate new item saving into separate function; allows saving items without "saving" feed
	if len(newitems) > 0 {
		f.ItemList = make([]*ItemDBEntry, 0, len(newitems))
		for _, item := range newitems {
			if item.Hash == "" {
				return fmt.Errorf("hash is empty for item: %v", item.Filename)
			}
			if item.FeedId > 0 && item.FeedId != f.ID {
				return fmt.Errorf("item feed id is set(%v), and does not match feed id (%v): %v", item.FeedId, f.ID, item.Filename)
			} else if item.FeedId == 0 {
				item.FeedId = f.ID
			}
			f.ItemList = append(f.ItemList, &item.ItemDBEntry)
		}
	}

	if err := db.saveFeed(&f.FeedDBEntry); err != nil {
		f.log.Errorf("error saving feed db: %v", err)
		return err
	}

	f.log.Infof("{%v} feed saved, id: %v, xml id: %v", f.Shortname, f.ID, f.XmlFeedData.ID)
	for _, i := range f.ItemList {
		f.log.Infof("{%v} item saved, id: %v, xmlId: %v", i.Filename, i.ID, i.XmlData.ID)
	}

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) Update() error {

	// make sure db is loaded
	if err := f.LoadDBFeed(true); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		return err
	} else {

		var itemCount = podutils.Tern(config.ForceUpdate, -1, config.MaxDupChecks*2)
		if itemList, err := f.loadDBFeedItems(itemCount, false); err != nil {
			f.log.Error("failed to load item entries: ", err)
			return err
		} else {
			// associate items into map for update hashes
			for _, item := range itemList {
				if _, exists := f.itemMap[item.Hash]; exists == true {
					f.log.Warn("Duplicate item found; wtf")
				}
				f.itemMap[item.Hash] = item
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

	var itemList *orderedmap.OrderedMap[string, podutils.XItemData]
	newXmlData, itemList, err = podutils.ParseXml(body, f)
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

	if itemList.Len() == 0 {
		f.log.Info("no items found to process")
		return nil
	}

	// maintain order on pairs; go from oldest to newest (each item moved to front)
	for pair := itemList.Newest(); pair != nil; pair = pair.Prev() {

		var (
			hash      = pair.Key
			xmldata   = pair.Value
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
			itemEntry.updateXmlData(hash, &xmldata)

		} else if itemEntry, err = createNewItemEntry(f.FeedToml, hash, &xmldata); err != nil {
			f.log.Error("failed creating new item entry; skipping: ", err)
			continue
		}

		f.log.Infof("item added: :%+v", itemEntry)

		// add it to the entry list
		f.itemMap[hash] = itemEntry

		// add it to the new items needing processing
		// warning; still add newest to oldest, due to skip remaining stuff..
		// at least until archive flag is set
		newItems = append([]*Item{itemEntry}, newItems...)
	}

	// todo: more error checking here
	// todo: need to check filename collissions

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
func (f *Feed) CheckDownloads() error {

	// make sure db is loaded; don't need xml for this
	var (
		itemList  []*Item
		dirtyList = make([]*Item, 0)
		err       error
	)

	if err = f.LoadDBFeed(false); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		return err
	} else {
		// load all items (will be sorted desc); we do want item xml
		if itemList, err = f.loadDBFeedItems(-1, true); err != nil {
			f.log.Error("failed to load item entries: ", err)
			return err
		}
	}
	f.log.Debug("Feed loaded from db for update: ", f.Shortname)

	// todo:
	// check download exist

	for _, item := range itemList {
		// check item hashes
		var (
			verifyHash  string
			filePathStr string
			fileExists  bool
			err         error
		)

		verifyHash, err = calcHash(item.XmlData.Guid, item.XmlData.Enclosure.Url, f.UrlParse)
		if err != nil {
			f.log.Errorf("error calculating hash: %v", err)
			return err
		}
		if verifyHash != item.Hash {
			f.log.Warnf("hash mismatch: calc:'%v', stored:'%v'", verifyHash, item.Hash)
		}

		// check download exists
		filePathStr = filepath.Join(f.mp3Path, item.Filename)
		fileExists, err = podutils.FileExists(filePathStr)
		if err != nil {
			f.log.Error("Error checking file exists: ", err)
		} else {

			if config.SetArchive && fileExists == false {
				f.log.Info("setting '%v' as archived", item.Filename)
				item.Archived = true
				dirtyList = append(dirtyList, item)
			}

			// logging mismatched file, in case mismatch still exists
			if fileExists == item.Archived {
				f.log.Warnf("%v, archive: %v, exists:%v", filePathStr, item.Archived, fileExists)
			}

			if item.Archived == false {
				if item.Downloaded == false {
					f.log.Warnf("File not downloaded: '%v'", item.Filename)
				} else if fileExists == false {
					f.log.Warnf("file downloaded, but not found: '%v'", item.Filename)
				}
			}
		}

		// check filename generation, in case shit changed..

	}

	if len(dirtyList) > 0 {
		f.saveDBFeed(nil, dirtyList)
	}

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

	//------------------------------------- DEBUG -------------------------------------
	// todo: skipRemaining can be removed when archived flag is set (downloaded == true && fileExists == false)
	var skipRemaining = false
	//------------------------------------- DEBUG -------------------------------------
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

		//------------------------------------- DEBUG -------------------------------------
		if config.Debug && skipRemaining {
			f.log.Debug("skip remaining set; previously downloaded items.. making sure downloaded == true")
			item.Downloaded = true
			continue
		}
		//------------------------------------- DEBUG -------------------------------------

		if item.Downloaded == true {
			f.log.Debug("skipping entry; file already downloaded")
			continue
		} else if fileExists == true {
			f.log.Debug("downloaded == false, but file already exists.. updating itemdata")
			item.Downloaded = true

			//------------------------------------- DEBUG -------------------------------------
			if config.Debug {
				f.log.Debug("debug setup, setting skip remaining to true")
				skipRemaining = true
			}
			//------------------------------------- DEBUG -------------------------------------

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

// --------------------------------------------------------------------------
// func (f *Feed) saveDBItems(itemList []*Item) error {

// 	// loop thru new items, saving xml
// 	if len(itemList) == 0 {
// 		f.log.Info("nothing to insert into db; item list is empty")
// 		return nil
// 	}

// 	var dbEntries = make([]*ItemDBEntry, 0, len(itemList))
// 	for _, item := range itemList {
// 		// if these are new items, make sure feed id is set
// 		item.FeedId = f.ID
// 		dbEntries = append(dbEntries, &item.ItemDBEntry)
// 	}

// 	// will save xml if set in the entry as well
// 	if err := db.saveItemEntries(dbEntries); err != nil {
// 		return err
// 	}
// 	return nil
// }
