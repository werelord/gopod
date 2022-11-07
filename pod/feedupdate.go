package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
)

// --------------------------------------------------------------------------
func (f *Feed) Update() error {

	var (
		fileCollList map[string]*Item
		guidCollList map[string]*Item
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
			// associate items into map for update hashes.. assuming growing capacity by 10 or so..
			fileCollList = make(map[string]*Item, len(itemList)+10)
			guidCollList = make(map[string]*Item, len(itemList)+10)

			for _, item := range itemList {
				if _, exists := f.itemMap[item.Hash]; exists {
					f.log.Warn("Duplicate item found; wtf")
				}
				f.itemMap[item.Hash] = item

				// file name checking
				if _, exists := fileCollList[item.Filename]; exists {
					err := fmt.Errorf("duplicate filename '%v' found; need to run checkDownloads", item.Filename)
					f.log.Error(err)
					return err
				} else {
					fileCollList[item.Filename] = item
				}

				// guid checking
				if _, exists := guidCollList[item.Guid]; exists {
					err := fmt.Errorf("duplicate guid '%v' found; need to run checkDownloads", item.Guid)
					f.log.Error(err)
					return err
				} else {
					fileCollList[item.Guid] = item
				}
			}
		}
	}

	// collision function, for checking whether a generated filename collides with an existing filename
	// passed into item for filename generation
	var collFunc = func(file string) bool {
		_, exists := fileCollList[file]
		return exists
	}

	f.log.Debug("Feed loaded from db for update: ", f.Shortname)

	var (
		body         []byte
		err          error
		itemPairList []podutils.ItemPair
		newXmlData   *podutils.XChannelData
		newItems     []*Item
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
			// we don't necessarily want to create and replace; just update the new data in the existing entry

			// replace the existing xml data; make sure the previous is loaded for replacing the existing
			if err := itemEntry.loadItemXml(db); err != nil {
				f.log.Error("failed loading item xml: ", err)
				continue
			}
			if err := itemEntry.updateXmlData(hash, xmldata); err != nil {
				f.log.Error("failed updating xml data: ", err)
				continue
			}
			// don't need to add it to itemmap, as it already is set
			// same for guid (based on hash) and filename collision, since filename should remain the same
			if itemEntry.Downloaded == false {
				newItems = append(newItems, itemEntry)
			}

		} else if itemEntry, exists = guidCollList[xmldata.Guid]; exists {
			// guid collision, with no hash collision.. means the url has changed..
			f.log.WithFields(log.Fields{
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
				f.log.Error("failed loading item xml: ", err)
				continue
			} else if err := itemEntry.updateFromEntry(f.FeedToml, hash, xmldata, collFunc); err != nil {
				f.log.Error("failed updating existing item entry; skipping: ", err)
				continue
			} else {
				// add it to various lists
				f.itemMap[hash] = itemEntry
				fileCollList[itemEntry.Filename] = itemEntry
				guidCollList[itemEntry.Guid] = itemEntry

				newItems = append(newItems, itemEntry)
			}

		} else if itemEntry, err = createNewItemEntry(f.FeedToml, hash, xmldata, f.EpisodeCount+1, collFunc); err != nil {
			f.log.Error("failed creating new item entry; skipping: ", err)
			continue
		} else {
			// new item from create entry; need to increment episode count
			f.EpisodeCount++
			f.itemMap[hash] = itemEntry
			fileCollList[itemEntry.Filename] = itemEntry
			guidCollList[itemEntry.Guid] = itemEntry
			newItems = append(newItems, itemEntry)
		}

		f.log.Infof("item added: :%+v", itemEntry)
	}

	// todo: more error checking here

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

	for _, item := range newItems {
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
