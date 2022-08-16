package pod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopod/podconfig"
	"gopod/poddb"
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
func NewFeed(config *podconfig.Config, feedToml podconfig.FeedToml) *Feed {
	feed := Feed{FeedToml: feedToml}
	feed.InitFeed(config)
	return &feed
}

// --------------------------------------------------------------------------
func (f *Feed) InitFeed(cfg *podconfig.Config) {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	config = cfg
	xmlFilePath := filepath.Join(config.Workspace, ".xml")
	f.mp3Path = filepath.Join(config.Workspace, f.Shortname)

	// todo: error propegation

	// attempt create the dirs
	if err := podutils.MkDirAll(xmlFilePath); err != nil {
		log.Error("error making xml directory: ", err)
		return
	}
	if err := podutils.MkDirAll(f.mp3Path); err != nil {
		log.Error("error making mp3 directory: ", err)
		return
	}

	f.xmlfile = filepath.Join(xmlFilePath, f.Shortname+"."+config.TimestampStr+".xml")
	f.itemlist = make(map[string]*Item)

	// todo: error check on this
	f.initDB()
}

// --------------------------------------------------------------------------
func (f *Feed) initDB() error {

	/*
		// todo: this

		if f.dbinitialized == false {
			log.Infof("{%v} initializing feed db, path: %v", f.Shortname, f.dbPath)
			var e error
			f.db, e = scribble.New(f.dbPath, nil)
			if e != nil {
				log.Error("Error init db: ", e)
				return
			}

			feedImport := FeedDBExportScribble{Feed: f}

			// load feed information
			if e := f.db.Read("./", "feed", &feedImport); e != nil {
				if errors.Is(e, fs.ErrNotExist) {
					log.Info("file doesn't exist; likely new feed")
				} else {
					log.Warn("error reading feed info:", e)
					return
				}
			}

			f.itemlist = toOrderedMap(f, feedImport.ItemEntryList)

			f.dbinitialized = true
		}

	*/
	return errors.New("not Implemented")

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
func (f Feed) saveDB() (err error) {

	log.Info("Saving db for ", f.Shortname)

	// todo: this

	if config.Simulate {
		log.Info("skipping saving database due to sim flag")
		return
	}

	// make sure database is initialized
	// f.initDB()
	// var feedExport_scribble FeedDBExportScribble

	// feedExport_scribble.Feed = &f
	// for pair := f.itemlist.Oldest(); pair != nil; pair = pair.Next() {

	// 	// convert to scribble format
	// 	entry := ItemDataOld{
	// 		Hash:     pair.Value.Hash,
	// 		ItemData: pair.Value.ItemData,
	// 	}

	// 	feedExport_scribble.ItemEntryList = append(feedExport_scribble.ItemEntryList, &entry)
	// }

	// if e := f.db.Write("./", "feed", feedExport_scribble); e != nil {
	// 	log.Error("failed to write database file: ", e)
	// 	return e
	// }

	return nil
}

// --------------------------------------------------------------------------
func (f *Feed) Update() {

	// make sure db is initialized
	// todo: error check on this
	f.initDB()

	var (
		body       []byte
		err        error
		newXmlData *podutils.XChannelData
		newItems   []*Item
	)
	// download file
	if body, err = podutils.Download(f.Url); err != nil {
		log.Error("failed to download: ", err)
		return
	}

	var itemList *orderedmap.OrderedMap[string, podutils.XItemData]
	newXmlData, itemList, err = podutils.ParseXml(body, f)

	if err != nil {
		if errors.Is(err, podutils.ParseCanceledError{}) {
			log.Info("parse cancelled: ", err)
			return
		} else {
			// not canceled; some other error.. exit
			log.Error("failed to parse xml: ", err)
			// save the file (don't rotate) for future examination
			f.saveAndRotateXml(body, false)
			return
		}
	}

	// if we're at this point, save the new channel data (buildDate or PubDate has changed)
	f.saveAndRotateXml(body, true)
	// future: comparison operations for feedData instead of direct insertion?
	f.XMLFeedData = *newXmlData

	// check url vs atom link & new feed url
	// TODO: handle this
	if f.XMLFeedData.AtomLinkSelf.Href != "" && f.Url != f.XMLFeedData.AtomLinkSelf.Href {
		log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XMLFeedData.AtomLinkSelf.Href)
	} else if f.XMLFeedData.NewFeedUrl != "" && f.Url != f.XMLFeedData.NewFeedUrl {
		log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XMLFeedData.NewFeedUrl)
	}

	// maintain order on pairs; go from oldest to newest (each item moved to front)
	for pair := itemList.Newest(); pair != nil; pair = pair.Prev() {

		var (
			hash      = pair.Key
			xmldata   = pair.Value
			itemdata  *Item
			itemerror error
		)

		if _, exists := f.itemlist[hash]; exists {
			// if the old item exists, it will get replaced on setting this new item
			// this should only happen when force == true; log warning if this is not the case
			if config.ForceUpdate == false {
				log.Warn("hash for new item already exists and --force is not set; something is seriously wrong")
			}
		}

		if itemdata, itemerror = CreateNewItemEntry(f.FeedToml, f.db, hash, &xmldata); itemerror != nil {
			// new entry, create it
			log.Error("failed creating new item entry; skipping: ", itemerror)
			continue
		}

		log.Infof("adding new item: :%+v", itemdata)

		// saving item xml
		if err = itemdata.saveItemXml(); err != nil {
			log.Error("saving xml daeta failed; skipping entry: ", err)
			continue
		}

		f.itemlist[hash] = itemdata

		newItems = append(newItems, itemdata)

	}

	f.saveDB()

	// todo: need to check filename collissions

	// process new entries
	// todo: move this outside update (likely on goroutine implementation)
	f.processNew(newItems)

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
			fmt.Sprintf("%v.([0-9]{8}_[0-9]{6})|(DEBUG).xml", f.Shortname),
			uint(config.XmlFilesRetained))
	}
}

//--------------------------------------------------------------------------
// feedProcess implementation
//--------------------------------------------------------------------------

// --------------------------------------------------------------------------
func (f Feed) SkipParsingItem(hash string) (skip bool, cancelRemaining bool) {

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

	//------------------------------------- DEBUG -------------------------------------
	var skipRemaining = false
	//------------------------------------- DEBUG -------------------------------------

	for _, item := range newItems {
		log.Debugf("processing new item: {%v %v}", item.Filename, item.Hash)

		podfile := filepath.Join(f.mp3Path, item.Filename)
		downloadTimestamp := time.Now()
		var fileExists bool

		if _, err := os.Stat(podfile); err == nil {
			fileExists = true
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
		if cd, err := podutils.DownloadBuffered(item.Url, podfile); err != nil {
			log.Error("Failed downloading pod:", err)
			continue
		} else if strings.Contains(cd, "filename") {
			// content disposition header, for the hell of it
			if r, err := regexp.Compile("filename=\"(.*)\""); err == nil {
				if matches := r.FindStringSubmatch(cd); len(matches) == 2 {
					item.CDFilename = matches[1]
				}
			} else {
				log.Warn("parsing content disposition had regex error: ", err)
			}

		}

		if err := os.Chtimes(podfile, downloadTimestamp, item.PubTimeStamp); err != nil {
			log.Error("failed to change modified time: ", err)
			// don't skip due to timestamp issue
		}

		log.Debug("finished downloading file: ", podfile)
		item.Downloaded = true
	}

	log.Info("all new downloads completed, saving db")
	f.saveDB()

}
