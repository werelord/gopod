package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
)

type feedUpdate struct {
	feed       *Feed
	newItems   []*Item
	newXmlData *podutils.XChannelData
	numDups    uint // number of dupiclates counted before skipping remaining items in xmlparse

	hashCollList  map[string]*Item
	fileCollList  map[string]*Item
	guidCollList  map[string]*Item
	collisionFunc func(string) bool
}

// --------------------------------------------------------------------------
func (f *Feed) Update() error {

	var (
		fUpdate = feedUpdate{
			feed: f,
		}
	)

	// load feed and items
	if itemlist, err := fUpdate.loadDB(); err != nil {
		reterr := fmt.Errorf("failed loading db: %w", err)
		f.log.Error(reterr)
		return reterr

	} else if err := fUpdate.processDBItems(itemlist); err != nil {
		reterr := fmt.Errorf("failed to populate item lists: %w", err)
		f.log.Error(reterr)
		return reterr
	}

	f.log.Debug("Feed loaded from db for update: ", f.Shortname)

	// download/load feed xml
	if err := fUpdate.loadNewFeed(); err != nil {

		if errors.Is(err, podutils.ParseCanceledError{}) {
			f.log.Info("parse cancelled: ", err)
			return nil // this is not an error, just a shortcut to stop processing
		} else {
			reterr := fmt.Errorf("failed to process feed: %w", err)
			f.log.Error(reterr)
			return err
		}
	}

	// process new entries
	if errlist := fUpdate.downloadNewItems(); len(errlist) > 0 {
		f.log.Error("errors in processing new items:\n")
		for _, err := range errlist {
			f.log.Errorf("\t%v", err)
		}
		return errlist[0]
	}

	// save everything here, as processing is done.. any errors should have exited out at some point
	if err := f.saveDBFeed(fUpdate.newXmlData, fUpdate.newItems); err != nil {
		f.log.Error("saving db failed: ", err)
		return err
	}

	f.log.Debugf("done processing feed")
	return nil
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) loadDB() ([]*Item, error) {
	var (
		f   = fup.feed
		log = fup.feed.log
	)

	// make sure db is loaded
	if err := f.LoadDBFeed(true); err != nil {
		reterr := fmt.Errorf("failed to load feed data from db: %w", err)
		log.Error(reterr)
		return nil, reterr

		// because we're doing filename collisions and guid collisions, grab all items
	} else if itemList, err := f.loadDBFeedItems(-1, false, cDESC); err != nil {
		reterr := fmt.Errorf("failed to load item entries: %w", err)
		log.Error(reterr)
		return itemList, reterr
	} else {
		return itemList, nil
	}
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) processDBItems(itemlist []*Item) error {

	var (
		f   = fup.feed
		log = fup.feed.log
	)

	fup.newItems = make([]*Item, 0)
	// associate items into map for update hashes.. assuming growing capacity by 10 or so..
	fup.hashCollList = make(map[string]*Item, len(itemlist)+10)
	fup.fileCollList = make(map[string]*Item, len(itemlist)+10)
	fup.guidCollList = make(map[string]*Item, len(itemlist)+10)

	for _, item := range itemlist {
		if _, exists := fup.hashCollList[item.Hash]; exists {
			log.Warn("Duplicate item found; this shouldn not happen (wtf)")
		}
		fup.hashCollList[item.Hash] = item

		// file name checking
		if _, exists := fup.fileCollList[item.Filename]; exists {
			err := fmt.Errorf("duplicate filename '%v' found; need to run checkDownloads", item.Filename)
			f.log.Error(err)
			return err
		} else {
			fup.fileCollList[item.Filename] = item
		}

		// guid checking
		if _, exists := fup.guidCollList[item.Guid]; exists {
			err := fmt.Errorf("duplicate guid '%v' found; need to run checkDownloads", item.Guid)
			f.log.Error(err)
			return err
		} else {
			fup.guidCollList[item.Guid] = item
		}
	}

	// collision function, for checking whether a generated filename collides with an existing filename
	// passed into item for filename generation
	fup.collisionFunc = func(file string) bool {
		_, exists := fup.fileCollList[file]
		return exists
	}

	return nil
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) loadNewFeed() error {

	var (
		body []byte
		err  error

		f   = fup.feed
		log = fup.feed.log

		itemPairList []podutils.ItemPair
	)

	if body, err = fup.loadFeedXml(); err != nil {
		log.Error("error in loading xml: ", err)
		return err
	} else if fup.newXmlData, itemPairList, err = podutils.ParseXml(body, fup); err != nil {

		if errors.Is(err, podutils.ParseCanceledError{}) == false {
			// save the file (don't rotate) for future examination
			fup.saveAndRotateXml(body, false)
		}
		return err
	}

	// if we're at this point, save the new channel data (buildDate or PubDate has changed)
	// don't rotate xml or save feed xml on using most recent
	if config.UseMostRecentXml == false {
		fup.saveAndRotateXml(body, true)
	}

	// check url vs atom link & new feed url
	if f.XmlFeedData.AtomLinkSelf.Href != "" && f.Url != f.XmlFeedData.AtomLinkSelf.Href {
		f.log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XmlFeedData.AtomLinkSelf.Href)
		f.log.Warn("(change url in config.toml to reflect this change)")
	} else if f.XmlFeedData.NewFeedUrl != "" && f.Url != f.XmlFeedData.NewFeedUrl {
		f.log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XmlFeedData.NewFeedUrl)
		f.log.Warn("(change url in config.toml to reflect this change)")
	}

	if len(itemPairList) == 0 {
		f.log.Info("no items found to process")
		return nil
	}

	// list comes out newest (top of xml feed) to oldest.. reverse that,
	// go oldest to newest, to maintain item count
	for i := len(itemPairList) - 1; i >= 0; i-- {

		var (
			hash    = itemPairList[i].Hash
			xmldata = itemPairList[i].ItemData
		)

		// errors on these do not cancel processing; the item is just not added to new item list for
		// download.. continue isn't needed here, but I'd rather be explicit on whats happening
		// in case anything is added in the future

		if handled, err := fup.checkExistingHash(hash, xmldata); (handled == true) || (err != nil) {
			if err != nil {
				f.log.Error(err)
			}
			continue

		} else if handled, err := fup.checkExistingGuid(hash, xmldata); (handled == true) || (err != nil) {
			if err != nil {
				f.log.Error(err)
			}
			continue

		} else if handled, err = fup.createNewEntry(hash, xmldata); (handled == true) || (err != nil) {
			if err != nil {
				f.log.Error(err)
			}
			continue
		}
	}

	return nil
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) checkExistingHash(hash string, xmldata *podutils.XItemData) (bool, error) {

	var (
		handled = false
		log     = fup.feed.log
	)

	if itemEntry, exists := fup.hashCollList[hash]; exists {
		handled = true

		if config.ForceUpdate == false {
			// this should only happen when force == true; log warning if this is not the case
			log.Warn("hash for new item already exists and --force is not set; something is seriously wrong")
		}
		// we don't necessarily want to create and replace; just update the new data in the existing entry

		// replace the existing xml data; make sure the previous is loaded for replacing the existing
		if err := itemEntry.loadItemXml(db); err != nil {
			log.Error("failed loading item xml: ", err)
			return true, err
		} else if err := itemEntry.updateXmlData(hash, xmldata); err != nil {
			log.Error("failed updating xml data: ", err)
			return true, err
		}
		// don't need to add it to itemmap, as it already is set
		// same for guid (based on hash) and filename collision, since filename should remain the same
		if itemEntry.Downloaded == false {
			fup.newItems = append(fup.newItems, itemEntry)
			fup.feed.log.Infof("checkHash: item modified: %+v", itemEntry)
		}
	}

	return handled, nil
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) checkExistingGuid(hash string, xmldata *podutils.XItemData) (bool, error) {

	var (
		handled = false
		log     = fup.feed.log
	)

	if itemEntry, exists := fup.guidCollList[xmldata.Guid]; exists {
		handled = true
		// guid collision, with no hash collision.. means the url has changed..
		log.WithFields(logrus.Fields{
			"previousguid": itemEntry.Guid,
			"newguid":      xmldata.Guid,
			"oldhash":      itemEntry.Hash,
			"newhash":      hash,
		}).Infof("guid collision detected with no hash collision; likely new url for same item")

		// hash will change.. filename might change if url is in filenameparse
		// filename might change based on filenameparse.. xml definitely changed (diff url)
		// pubtimestamp maybe change.. episode count should not change

		// make sure the previous is loaded for replacing the existing
		if err := itemEntry.loadItemXml(db); err != nil {
			log.Error("failed loading item xml: ", err)
			return true, err
		} else if err := itemEntry.updateFromEntry(fup.feed.FeedToml, hash, xmldata, fup.collisionFunc); err != nil {
			log.Error("failed updating existing item entry; skipping: ", err)
			return true, err
		} else {
			// add it to various lists; may do a replacement
			fup.hashCollList[hash] = itemEntry
			fup.fileCollList[itemEntry.Filename] = itemEntry
			fup.guidCollList[itemEntry.Guid] = itemEntry

			fup.newItems = append(fup.newItems, itemEntry)
			fup.feed.log.Infof("checkGuid: item modified: %+v", itemEntry)
		}
	}

	return handled, nil
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) createNewEntry(hash string, xmldata *podutils.XItemData) (bool, error) {

	var (
		handled = false
		f       = fup.feed
		log     = fup.feed.log
	)

	if itemEntry, err := createNewItemEntry(f.FeedToml, hash, xmldata, f.EpisodeCount+1, fup.collisionFunc); err != nil {
		log.Error("failed creating new item entry; skipping: ", err)
		return true, err
	} else {
		handled = true
		// new item from create entry; need to increment episode count and add to all the lists
		f.EpisodeCount++
		fup.hashCollList[hash] = itemEntry
		fup.fileCollList[itemEntry.Filename] = itemEntry
		fup.guidCollList[itemEntry.Guid] = itemEntry
		fup.newItems = append(fup.newItems, itemEntry)

		f.log.Infof("createNew: item added: %+v", itemEntry)
	}

	return handled, nil
}

// --------------------------------------------------------------------------
func (fup feedUpdate) loadFeedXml() ([]byte, error) {
	var (
		log  = fup.feed.log
		body []byte
		err  error
	)

	if config.UseMostRecentXml {
		// find the most recent xml based on the glob pattern
		var filename string
		if filename, err = podutils.FindMostRecent(filepath.Dir(fup.feed.xmlfile), fmt.Sprintf("%v.*.xml", fup.feed.Shortname)); err != nil {
			log.Error("error finding most recent xml: ", err)
			return nil, err
		}

		log.Debug("loading xml file: ", filename)
		if body, err = podutils.LoadFile(filename); err != nil {
			log.Error("error loading xml file: ", err)
			return nil, err
		}

	} else {
		// download from url, unbuffered
		if body, err = podutils.Download(fup.feed.Url); err != nil {
			log.Error("failed to download: ", err)
			return nil, err
		}
	}

	if len(body) == 0 {
		err = fmt.Errorf("body length is zero")
		log.Error(err)
		return nil, err
	}
	return body, nil
}

// --------------------------------------------------------------------------
func (fup feedUpdate) saveAndRotateXml(body []byte, shouldRotate bool) {
	// for external reference
	if err := podutils.SaveToFile(body, fup.feed.xmlfile); err != nil {
		fup.feed.log.Error("failed saving xml file: ", err)
		// not exiting; not a fatal error as the parsing happens on the byte string
	} else if shouldRotate && config.XmlFilesRetained > 0 {
		fup.feed.log.Debug("rotating xml files..")
		podutils.RotateFiles(filepath.Dir(fup.feed.xmlfile),
			fmt.Sprintf("%v.*.xml", fup.feed.Shortname),
			uint(config.XmlFilesRetained))
	}
}

//--------------------------------------------------------------------------
// feedProcess implementation
//--------------------------------------------------------------------------

// --------------------------------------------------------------------------
func (fup *feedUpdate) SkipParsingItem(hash string) (skip bool, cancelRemaining bool) {

	if config.ForceUpdate {
		return false, false
	}

	// assume itemlist has been populated with enough entries (if not all)
	_, skip = fup.hashCollList[hash]

	if (config.MaxDupChecks >= 0) && (skip == true) {
		fup.numDups++
		cancelRemaining = (fup.numDups >= uint(config.MaxDupChecks))
	}
	return
}

// --------------------------------------------------------------------------
// returns true if parsing should halt on pub date; parse returns ParseCanceledError on true
func (fup feedUpdate) CancelOnPubDate(xmlPubDate time.Time) (cont bool) {

	if config.ForceUpdate {
		return false
	}

	//f.log.Tracef("Checking build date; \nFeed.Pubdate:'%v' \nxmlBuildDate:'%v'", f.XMLFeedData.PubDate.Unix(), xmlPubDate.Unix())
	if fup.feed.XmlFeedData.PubDate.IsZero() == false {
		if xmlPubDate.After(fup.feed.XmlFeedData.PubDate) == false {
			fup.feed.log.Info("new pub date not after previous; cancelling parse")
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
// returns true if parsing should halt on build date; parse returns ParseCanceledError on true
func (fup feedUpdate) CancelOnBuildDate(xmlBuildDate time.Time) (cont bool) {

	if config.ForceUpdate {
		return false
	}

	//f.log.Tracef("Checking build date; Feed.LastBuildDate:'%v', xmlBuildDate:'%v'", f.XMLFeedData.LastBuildDate, xmlBuildDate)
	if fup.feed.XmlFeedData.LastBuildDate.IsZero() == false {
		if xmlBuildDate.After(fup.feed.XmlFeedData.LastBuildDate) == false {
			fup.feed.log.Info("new build date not after previous; cancelling parse")
			return true
		}
	}
	return false
}

// --------------------------------------------------------------------------
func (fup feedUpdate) CalcItemHash(guid string, url string) (string, error) {
	// within item
	return calcHash(guid, url, fup.feed.UrlParse)
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) downloadNewItems() []error {

	var (
		f       = fup.feed
		errList = make([]error, 0)
	)

	if len(fup.newItems) == 0 {
		f.log.Info("no items to process; item list is empty")
		return nil
	}

	for _, item := range fup.newItems {
		f.log.Debugf("processing new item: {%v %v}", item.Filename, item.Hash)

		podfile := filepath.Join(f.mp3Path, item.Filename)
		var fileExists bool

		fileExists, err := podutils.FileExists(podfile)
		if err != nil {
			f.log.Error("error in FileExists; not downloading: ", err)
			errList = append(errList, err)
			continue
		}

		if item.Downloaded == true {
			f.log.Warnf("item downloaded '%v', archived: '%v', fileExists: '%v'", item.Downloaded, item.Archived, fileExists)
			if fileExists == false {
				if item.Archived == true {
					f.log.Info("skipping download due to archived flag")
					continue
				} else {
					f.log.Warn("downloading item; archive flag not set")
				}
			} else {
				f.log.Warn("skipping download; file already downloaded.. ")
				continue
			}
		} else if fileExists == true {
			f.log.Warnf("item downloaded '%v', archived: '%v', fileExists: '%v'", item.Downloaded, item.Archived, fileExists)
			f.log.Warn("file already exists.. possible filename collision? skipping download")
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