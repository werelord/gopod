package pod

import (
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"gopod/podconfig"
	"gopod/poddb"
	"gopod/podutils"

	"github.com/ostafen/clover/v2"
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

	// channel entries from xml, exported
	XMLFeedData podutils.XChannelData
}

type feedInternal struct {
	// local items, not exported to database
	db *poddb.PodDB

	dbXmlId string // id referencing this feed's entry in the db
	xmlfile string
	mp3Path string

	numDups uint // number of dupiclates counted before skipping remaining items in xmlparse

	// itemlist is not explicitly exported, but converted to array to be exported
	// used mostly for checking update, as to when to quit
	itemlist map[string]*Item

	dbinitialized bool
}

type FeedXmlDBEntry struct {
	Hash        string
	XmlFeedData podutils.XChannelData
}

// todo: Feed data entry, if anything needs to be preserved

var (
	config *podconfig.Config
)

// func (f Feed) Format(fs fmt.State, c rune) {
// 	fs.Write([]byte("Name:" + f.Shortname + " url: " + f.Url))
// }

// --------------------------------------------------------------------------
func NewFeed(config *podconfig.Config, feedToml podconfig.FeedToml) (*Feed, error) {
	feed := Feed{FeedToml: feedToml}
	if err := feed.initFeed(config); err != nil {
		log.Error("Failed to init feed: ", err)
		return nil, err
	}
	return &feed, nil
}

// --------------------------------------------------------------------------
func (f *Feed) initFeed(cfg *podconfig.Config) error {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	config = cfg
	xmlFilePath := filepath.Join(config.WorkspaceDir, f.Shortname, ".xml")
	f.mp3Path = filepath.Join(config.WorkspaceDir, f.Shortname)

	// todo: error propegation

	// attempt create the dirs
	if err := podutils.MkdirAll(xmlFilePath); err != nil {
		log.Error("error making xml directory: ", err)
		return err
	}
	if err := podutils.MkdirAll(f.mp3Path); err != nil {
		log.Error("error making mp3 directory: ", err)
		return err
	}

	f.xmlfile = filepath.Join(xmlFilePath, f.Shortname+"."+config.TimestampStr+".xml")
	f.itemlist = make(map[string]*Item)

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) initDB() error {

	if f.dbinitialized == false {
		var (
			err error
			id  string
		)

		log.Infof("{%v} initializing feed db", f.Shortname)
		if f.db, err = poddb.NewDB(f.Shortname); err != nil {
			log.Error("failed creating db: ", err)
			return err
		}

		// load feed info
		feedXml := FeedXmlDBEntry{
			Hash: f.generateHash(),
		}

		if id, err = f.db.FeedCollection().FetchByEntry(&feedXml); err != nil {
			log.Error("failed fetching feed xml: ", err)
			return err
			// data integrity checks
		} else if id == "" {
			return errors.New("feed id is missing")
		} else if feedXml.XmlFeedData.Title == "" {
			return fmt.Errorf("feed title missing, id: '%v'", id)
		}

		f.dbXmlId = id
		f.XMLFeedData = feedXml.XmlFeedData

		log.Debugf("feed info fetched: %v (%v) ", f.XMLFeedData.Title, f.dbXmlId)

		if err = f.loadItemEntries(); err != nil {
			log.Error("failed loading item entries: ", err)
			return err
		}

		f.dbinitialized = true
	}

	return nil
}

// --------------------------------------------------------------------------
func (f Feed) generateHash() string {
	return podutils.GenerateHash(f.Shortname)
}

// --------------------------------------------------------------------------
func (f *Feed) loadItemEntries() error {

	var (
		err       error
		dbEntries []poddb.DBEntry
	)

	// load itemlist.. if force is enabled, load everything..
	// otherwise limit to numdupcheck * relative amount
	if config.ForceUpdate {
		// the list is not sorted, so don't worry about that
		log.Debug("loading all items (force == true)")
		dbEntries, err = f.db.ItemDataCollection().FetchAll(createItemDataDBEntry)
	} else {
		var limit = config.MaxDupChecks * 2
		var opt = clover.SortOption{Field: "ItemData.PubTimeStamp", Direction: -1}
		q := f.db.ItemDataCollection().NewQuery().Sort(opt).Limit(limit)
		log.Debugf("loading %v items sorted by pubdated", limit)
		dbEntries, err = f.db.ItemDataCollection().FetchAllWithQuery(createItemDataDBEntry, q)
	}

	if err != nil {
		log.Error("Failed to get item data from db: ", err)
		return err
	} else if len(dbEntries) == 0 {
		log.Warn("unable to get db entries; list is empty (new feed?)")
	}

	for _, entry := range dbEntries {
		var item *Item
		if item, err = loadFromDBEntry(f.FeedToml, f.db, entry); err != nil {
			log.Error("failed to load item data: ", err)
			// if this fails, something is wrong
			return err
		}
		if _, exists := f.itemlist[item.Hash]; exists == true {
			log.Warn("Duplicate item found; wtf")
		}
		f.itemlist[item.Hash] = item
	}

	return nil

}

// for db conversion only
// func (f *Feed) CreateExport() *FeedDBEntry {

// 	f.initDB()

// 	feedStruct := FeedDBEntry{
// 		Hash:        podutils.GenerateHash(f.Shortname),
// 		XmlFeedData: f.XMLFeedData,
// 	}

// 	return &feedStruct
// }

// func (f *Feed) CreateItemDataExport() []*ItemDataDBEntry {

// 	f.initDB()

// 	var list = make([]*ItemDataDBEntry, 0, f.itemlist.Len())

// 	for pair := f.itemlist.Oldest(); pair != nil; pair = pair.Next() {
// 		entry := ItemDataDBEntry{
// 			Hash:     pair.Value.Hash,
// 			ItemData: pair.Value.ItemData,
// 		}
// 		list = append(list, &entry)
// 	}

// 	return list
// }

// for db conversion only
// func (f *Feed) CreateItemXmlExport() []*ItemXmlDBEntry {

// 	f.initDB()

// 	records, err := f.db.ReadAll("items")
// 	if err != nil {
// 		log.Error("error: ", err)
// 	}

// 	var list = make([]*ItemXmlDBEntry, 0, len(records))

// 	var scribbleEntryMap = make(map[string]string)

// 	// put these in reverse order.. fuck it
// 	for i := len(records) - 1; i >= 0; i-- {
// 		var item = records[i]
// 		var scribbleEntry = ItemExportScribble{}

// 		if err := json.Unmarshal([]byte(item), &scribbleEntry); err != nil {
// 			log.Error("unmarshal error: ", err)
// 		}
// 		list = append(list, &ItemXmlDBEntry{
// 			Hash:    scribbleEntry.Hash,
// 			ItemXml: scribbleEntry.ItemXmlData,
// 		})

// 		// we're running a check on shit here as well.. make sure this record exists in itemlist
// 		if _, exists := f.itemlist.Get(scribbleEntry.Hash); exists == false {
// 			log.Warnf("(%v) Itemlist missing scribble xml hash %v (item:%v)",
// 				f.Shortname, scribbleEntry.Hash, scribbleEntry.ItemXmlData.Enclosure.Url)
// 		}
// 		// set a map to check items the opposite way; just need the hash
// 		scribbleEntryMap[scribbleEntry.Hash] = scribbleEntry.Hash
// 	}

// 	// check the other way around
// 	for pair := f.itemlist.Oldest(); pair != nil; pair = pair.Next() {
// 		if _, exists := scribbleEntryMap[pair.Value.Hash]; exists == false {
// 			log.Warnf("(%v) scribble entries missing itemlist hash %v (item:%v)",
// 				f.Shortname, pair.Value.Hash, pair.Value.Filename)
// 		}
// 	}

// 	return list
// }

// --------------------------------------------------------------------------
func (f Feed) saveFeedDb() (err error) {

	log.Info("Saving db for ", f.Shortname)

	// make sure we have an ID
	if f.dbXmlId == "" {
		log.Warn("saving feed xml; but id is empty; this shouldn't happen")
	}

	if config.Simulate {
		log.Info("skipping saving database due to sim flag")
		return
	}

	entry := FeedXmlDBEntry{
		Hash:        f.generateHash(),
		XmlFeedData: f.XMLFeedData,
	}

	id, err := f.db.FeedCollection().InsertyById(f.dbXmlId, entry)
	if err != nil {
		return err
	} else if id != f.dbXmlId {
		err := errors.New("id returned from the db doesn't match previously stored")
		log.Error("%v\nfid:'%v' != dbid:'%v'", err, f.dbXmlId, id)
		return err
	}
	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) Update() error {

	// make sure db is loaded
	if err := f.initDB(); err != nil {
		log.Error("failed to init db: ", err)
		return err
	}

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
		log.Error("error in download: ", err)
		return err
	} else if len(body) == 0 {
		err = fmt.Errorf("body length is zero")
		log.Error(err)
		return err
	}

	// todo: break this up some more

	var itemList *orderedmap.OrderedMap[string, podutils.XItemData]
	newXmlData, itemList, err = podutils.ParseXml(body, f)
	if err != nil {
		if errors.Is(err, podutils.ParseCanceledError{}) {
			log.Info("parse cancelled: ", err)
			return nil // this is not an error, just a shortcut
		} else {
			// not canceled; some other error.. exit
			log.Error("failed to parse xml: ", err)
			// save the file (don't rotate) for future examination
			f.saveAndRotateXml(body, false)
			return err
		}
	}

	// if we're at this point, save the new channel data (buildDate or PubDate has changed)
	// don't rotate on using most recent
	if config.UseMostRecentXml == false {
		f.saveAndRotateXml(body, true)
	}

	// future: comparison operations for feedData instead of direct insertion?
	f.XMLFeedData = *newXmlData
	if err = f.saveFeedDb(); err != nil {
		log.Error("saving db failed: ", err)
		// continue on..
	}

	// check url vs atom link & new feed url
	// TODO: handle this
	if f.XMLFeedData.AtomLinkSelf.Href != "" && f.Url != f.XMLFeedData.AtomLinkSelf.Href {
		log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XMLFeedData.AtomLinkSelf.Href)
	} else if f.XMLFeedData.NewFeedUrl != "" && f.Url != f.XMLFeedData.NewFeedUrl {
		log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XMLFeedData.NewFeedUrl)
	}

	if itemList.Len() == 0 {
		log.Info("no items found to process")
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

		if itemEntry, exists = f.itemlist[hash]; exists {
			// this should only happen when force == true; log warning if this is not the case
			if config.ForceUpdate == false {
				log.Warn("hash for new item already exists and --force is not set; something is seriously wrong")
			}
			// we don't necessarily want to create and replace;
			// just update the new data in the existing entry

			// replace the existing xml data
			// todo: deep copy comparison
			itemEntry.updateXmlData(hash, &xmldata)

		} else if itemEntry, err = createNewItemEntry(f.FeedToml, f.db, hash, &xmldata); err != nil {
			// new entry, create it
			log.Error("failed creating new item entry; skipping: ", err)
			continue
		}

		log.Infof("item added: :%+v", itemEntry)

		// add it to the entry list
		f.itemlist[hash] = itemEntry

		// add it to the new items needing processing
		// warning; still add newest to oldest, due to skip remaining stuff..
		// at least until archive flag is set
		newItems = append([]*Item{itemEntry}, newItems...)
	}

	// regardless of new or existing
	// todo: need to have dirty flag handling
	if err = f.batchSaveItemData(newItems); err != nil {
		log.Error("failed to insert xml db entries: ", err)
		return err
	}

	if err = f.batchSaveItemXml(newItems); err != nil {
		log.Error("failed to insert xml db entries: ", err)
		return err
	}

	// todo: more error checking here
	// todo: need to check filename collissions

	// process new entries
	// todo: move this outside update (likely on goroutine implementation)

	f.processNew(newItems)

	return nil
}

// --------------------------------------------------------------------------
func (f Feed) downloadFeedXml() (body []byte, err error) {
	// download file
	if body, err = podutils.Download(f.Url); err != nil {
		log.Error("failed to download: ", err)
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
	log.Debug("loading xml file: ", filename)

	return podutils.LoadFile(filename)
}

// --------------------------------------------------------------------------
func (f Feed) saveAndRotateXml(body []byte, shouldRotate bool) {
	// for external reference
	if err := podutils.SaveToFile(body, f.xmlfile); err != nil {
		log.Error("failed saving xml file: ", err)
		// not exiting; not a fatal error as the parsing happens on the byte string
	} else if shouldRotate && config.XmlFilesRetained > 0 {
		log.Debug("rotating xml files..")
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
	_, skip = f.itemlist[hash]

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

	//log.Tracef("Checking build date; \nFeed.Pubdate:'%v' \nxmlBuildDate:'%v'", f.XMLFeedData.PubDate.Unix(), xmlPubDate.Unix())
	if f.XMLFeedData.PubDate.IsZero() == false {
		if xmlPubDate.After(f.XMLFeedData.PubDate) == false {
			log.Info("new pub date not after previous; cancelling parse")
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

	//log.Tracef("Checking build date; Feed.LastBuildDate:'%v', xmlBuildDate:'%v'", f.XMLFeedData.LastBuildDate, xmlBuildDate)
	if f.XMLFeedData.LastBuildDate.IsZero() == false {
		if xmlBuildDate.After(f.XMLFeedData.LastBuildDate) == false {
			log.Info("new build date not after previous; cancelling parse")
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
func (f *Feed) processNew(newItems []*Item) {

	if len(newItems) == 0 {
		log.Info("no items to process; item list is empty")
		return
	}

	//------------------------------------- DEBUG -------------------------------------
	// todo: skipRemaining can be removed when archived flag is set (downloaded == true && fileExists == false)
	var skipRemaining = false
	//------------------------------------- DEBUG -------------------------------------
	// todo: move download handling within item
	for _, item := range newItems {
		log.Debugf("processing new item: {%v %v}", item.Filename, item.Hash)

		podfile := filepath.Join(f.mp3Path, item.Filename)
		var fileExists bool

		fileExists, err := podutils.FileExists(podfile)
		if err != nil {
			log.Warn("error in FileExists: ", err)
		}

		//------------------------------------- DEBUG -------------------------------------
		if config.Debug && skipRemaining {
			log.Debug("skip remaining set; previously downloaded items.. making sure downloaded == true")
			item.Downloaded = true
			continue
		}
		//------------------------------------- DEBUG -------------------------------------

		if item.Downloaded == true {
			log.Debug("skipping entry; file already downloaded")
			continue
		} else if fileExists == true {
			log.Debug("downloaded == false, but file already exists.. updating itemdata")
			item.Downloaded = true

			//------------------------------------- DEBUG -------------------------------------
			if config.Debug {
				log.Debug("debug setup, setting skip remaining to true")
				skipRemaining = true
			}
			//------------------------------------- DEBUG -------------------------------------

			continue
		}

		if config.Simulate {
			log.Info("skipping downloading file due to sim flag")
			continue
		}
		if err = item.Download(f.mp3Path); err != nil {
			log.Error("Error downloading file: ", err)
		}
		log.Info("finished processing file: ", podfile)
	}

	// regardless of whether skipremaining was set, make sure we save all the new items
	// can't do this in the above loop, as continue catching might skip (maybe)

	if err := f.batchSaveItemData(newItems); err != nil {
		log.Error("failed to save item data: ", err)
	}

	log.Info("all new downloads completed")
}

// --------------------------------------------------------------------------
func (f *Feed) batchSaveItemXml(itemList []*Item) error {
	// loop thru new items, saving xml
	if len(itemList) == 0 {
		log.Info("nothing to insert into db; item list is empty")
		return nil
	}

	dbEntries := make([]*poddb.DBEntry, 0, len(itemList))
	for _, item := range itemList {
		dbEntries = append(dbEntries, item.getItemXmlDBEntry())

	}
	if err := f.db.ItemXmlCollection().InsertAll(dbEntries); err != nil {
		return err
		// } else if config.Debug {
		// 	// make sure xml id is set by ref
		// 	for _, item := range newItems {
		// 		log.Debugf("xml id: '%v'", item.dbXmlId)
		// 	}
	}
	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) batchSaveItemData(itemList []*Item) error {

	if len(itemList) == 0 {
		log.Info("nothing to insert into db; item list is empty")
		return nil
	}

	dbUpdates := make([]*poddb.DBEntry, 0, len(itemList))
	for _, item := range itemList {
		dbUpdates = append(dbUpdates, item.getItemDataDBEntry())
	}
	if err := f.db.ItemDataCollection().InsertAll(dbUpdates); err != nil {
		return err
	}
	return nil
}
