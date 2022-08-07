package pod

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopod/podconfig"
	"gopod/podutils"

	scribble "github.com/nanobox-io/golang-scribble"
	log "github.com/sirupsen/logrus"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

//--------------------------------------------------------------------------
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
	// local items
	db      *scribble.Driver
	xmlfile string
	mp3Path string
	dbPath  string
	// itemlist is not explicitly exported, but converted to array to be exported
	itemlist *orderedmap.OrderedMap[string, *ItemData]

	dbinitialized bool
}

// exported fields for database serialization
type FeedDBExport struct {
	Feed *Feed
	// using slice for json db output
	ItemEntryList []*FeedItemEntry
}

var (
	config  *podconfig.Config
	numDups uint // number of dupiclates counted before skipping remaining items in xmlparse
)

// func (f Feed) Format(fs fmt.State, c rune) {
// 	fs.Write([]byte("Name:" + f.Shortname + " url: " + f.Url))
// }

//--------------------------------------------------------------------------
func NewFeed(config *podconfig.Config, feedToml podconfig.FeedToml) *Feed {
	feed := Feed{FeedToml: feedToml}
	feed.InitFeed(config)
	return &feed
}

//--------------------------------------------------------------------------
func (f *Feed) InitFeed(cfg *podconfig.Config) {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	config = cfg

	f.dbPath = filepath.Join(config.Workspace, f.Shortname, "db")
	f.xmlfile = filepath.Join(f.dbPath, f.Shortname+"."+config.TimestampStr+".xml")
	f.mp3Path = filepath.Join(config.Workspace, f.Shortname)

	f.itemlist = orderedmap.New[string, *ItemData]()
}

//--------------------------------------------------------------------------
func (f *Feed) initDB() {
	// future: modify scribble to use 7zip archives?
	if f.dbinitialized == false {
		log.Infof("{%v} initializing feed db, path: %v", f.Shortname, f.dbPath)
		var e error
		f.db, e = scribble.New(f.dbPath, nil)
		if e != nil {
			log.Error("Error init db: ", e)
			return
		}

		feedImport := FeedDBExport{Feed: f}

		// load feed information
		if e := f.db.Read("./", "feed", &feedImport); e != nil {
			if errors.Is(e, fs.ErrNotExist) {
				log.Info("file doesn't exist; likely new feed")
			} else {
				log.Warn("error reading feed info:", e)
				return
			}
		} else {
			// populate ordered map
			for _, item := range feedImport.ItemEntryList {
				itemdata := ItemData{
					parent:        f,
					FeedItemEntry: *item,
				}
				f.itemlist.Set(item.Hash, &itemdata)
			}
		}

		f.dbinitialized = true
	}
}

//--------------------------------------------------------------------------
func (f Feed) saveDB() (err error) {

	log.Info("Saving db for ", f.Shortname)

	if config.Simulate {
		log.Info("skipping saving database due to sim flag")
		return
	}

	// make sure database is initialized
	f.initDB()
	var feedExport FeedDBExport

	feedExport.Feed = &f
	for pair := f.itemlist.Oldest(); pair != nil; pair = pair.Next() {
		feedExport.ItemEntryList = append(feedExport.ItemEntryList, &pair.Value.FeedItemEntry)
	}

	if e := f.db.Write("./", "feed", feedExport); e != nil {
		log.Error("failed to write database file: ", e)
		return e
	}

	return nil
}

//--------------------------------------------------------------------------
func (f *Feed) Update() {

	// make sure db is initialized
	f.initDB()

	var (
		body       []byte
		err        error
		newXmlData *podutils.XChannelData
		newItems   []*ItemData
	)
	// download file
	// todo: check header for last modified; we could possibly skip parsing
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
			itemdata  *ItemData
			itemerror error
		)

		
		if _, exists := f.itemlist.Get(hash); exists {
			// if the old item exists, it will get replaced on setting this new item
			// this should only happen when force == true; log warning if this is not the case
			if config.ForceUpdate == false {
				log.Warn("hash for new item already exists and --force is not set; something is seriously wrong")
			}
		}

		if itemdata, itemerror = CreateNewItemEntry(f, hash, &xmldata); itemerror != nil {
			// new entry, create it
			log.Error("failed creating new item entry; skipping: ", itemerror)
			continue
		}

		log.Infof("adding new item: :%+v", itemdata)

		f.itemlist.Set(hash, itemdata)
		if e := f.itemlist.MoveToFront(hash); err != nil {
			log.Error("failed to move to front: ", e)
		}

		// saving item xml; this
		itemdata.saveItemXml()

		// download these in order newest to last, to hijack initial population of downloaded items
		newItems = append([]*ItemData{itemdata}, newItems...)

	}

	f.saveDB()

	// process new entries
	f.processNew(newItems)

}

//--------------------------------------------------------------------------
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

//--------------------------------------------------------------------------
func (f Feed) SkipParsingItem(hash string) (skip bool, cancelRemaining bool) {

	if config.ForceUpdate {
		return false, false
	}

	_, skip = f.itemlist.Get(hash)

	if (config.MaxDupChecks >= 0) && (skip == true) {
		numDups++
		cancelRemaining = (numDups >= uint(config.MaxDupChecks))
	}
	return
}

//--------------------------------------------------------------------------
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

//--------------------------------------------------------------------------
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

//--------------------------------------------------------------------------
func (f *Feed) processNew(newItems []*ItemData) {

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
