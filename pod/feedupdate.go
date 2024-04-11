package pod

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"time"

	log "gopod/multilogger"
	"gopod/podutils"

	"github.com/araddon/dateparse"
)

// common to all
type DownloadResults struct {
	currentLogger        log.Logger
	Results              map[string][]string
	TotalDownloaded      uint
	TotalDownloadedBytes uint64
	Errors               []error
}

func (dr *DownloadResults) addError(errs ...error) {
	for _, err := range errs {
		if dr.currentLogger != nil {
			// log.Error(err)
		} else {
			dr.currentLogger.Error(err)
		}
	}
	dr.Errors = append(dr.Errors, errs...)
}

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

func UpdateFeeds(feeds ...*Feed) DownloadResults {

	var dlRes = DownloadResults{
		Results:              make(map[string][]string, len(feeds)),
		TotalDownloaded:      0,
		TotalDownloadedBytes: 0,
		Errors:               make([]error, 0),
	}

	for _, feed := range feeds {
		feed.log.Info("running update")
		feed.update(&dlRes)
	}

	return dlRes
}

// --------------------------------------------------------------------------
func (f *Feed) update(results *DownloadResults) {

	var (
		fUpdate = feedUpdate{
			feed: f,
		}
	)
	results.currentLogger = f.log

	// load feed and items
	if itemlist, err := fUpdate.loadDB(); err != nil {
		results.addError(fmt.Errorf("failed loading db: %w", err))
		return

	} else if err := fUpdate.loadDBItems(itemlist); err != nil {
		results.addError(fmt.Errorf("failed to populate item lists: %w", err))
		return
	}

	f.log.Debugf("Feed loaded from db for update: %v", f.Shortname)

	// download/load feed xml
	if err := fUpdate.loadNewFeed(); err != nil {

		if errors.Is(err, podutils.ParseCanceledError{}) {
			f.log.Infof("parse cancelled: %v", err)
			return // this is not an error, just a shortcut to stop processing
		} else {
			results.addError(fmt.Errorf("failed to process feed: %w", err))
			return
		}
	}

	// process feed image changes
	if err := fUpdate.processFeedImage(); err != nil {
		// don't fail becaudse image fail..
		f.log.Warnf("error processing image, continuing with feed processing: '%v'", err)
	}

	// before download save feed & items.. downloads will update saved feeds
	if err := f.saveDBFeed(fUpdate.newXmlData, fUpdate.newItems); err != nil {
		results.addError(fmt.Errorf("saving db failed: %v", err))
		return
	}

	// process new entries
	if success := fUpdate.downloadNewItems(results); success == false {
		f.log.Error("download errors encountered")
	}

	f.log.Debugf("done processing feed")
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) loadDB() ([]*Item, error) {
	var (
		f   = fup.feed
		log = fup.feed.log
	)

	// make sure db is loaded
	if err := f.LoadDBFeed(loadOptions{includeXml: true}); err != nil {
		reterr := fmt.Errorf("failed to load feed data from db: %w", err)
		log.Error(reterr)
		return nil, reterr

		// because we're doing filename collisions and guid collisions, grab all items
	} else if itemList, err := f.loadDBFeedItems(AllItems, loadOptions{includeXml: false, direction: cDESC}); err != nil {
		reterr := fmt.Errorf("failed to load item entries: %w", err)
		log.Error(reterr)
		return itemList, reterr
	} else {

		// check episode count start; if a new feed (count == 0) double check config for countStart
		if (f.EpisodeCount == 0) && (f.CountStart != 0) {
			log.Debugf("new feed (?); episode count == 0 and countStart == %v; setting episodeCount to countStart", f.CountStart)
			f.EpisodeCount = f.CountStart
		}

		return itemList, nil
	}
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) loadDBItems(itemlist []*Item) error {

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

	if body, err = fup.loadNewXml(); err != nil {
		log.Errorf("error in loading xml: %v", err)
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
	if fup.newXmlData.AtomLinkSelf.Href != "" && f.Url != fup.newXmlData.AtomLinkSelf.Href {
		f.log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, fup.newXmlData.AtomLinkSelf.Href)
		f.log.Warn("(change url in config.toml to reflect this change)")
	} else if fup.newXmlData.NewFeedUrl != "" && f.Url != fup.newXmlData.NewFeedUrl {
		f.log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, fup.newXmlData.NewFeedUrl)
		f.log.Warn("(change url in config.toml to reflect this change)")
	}

	if err := fup.processNewItems(itemPairList); err != nil {
		return err
	}

	return nil
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) processNewItems(newItems []podutils.ItemPair) error {

	if len(newItems) == 0 {
		fup.feed.log.Info("no items found to process")
		return nil
	}

	// if this is std chrono, reverse it to match typical reverse chrono to keep item count correct
	if fup.feed.StdChrono {
		slices.Reverse(newItems)
	}

	var retErr error = nil

	// list comes out newest (top of xml feed) to oldest.. reverse that,
	// go oldest to newest, to maintain item count
	for i := len(newItems) - 1; i >= 0; i-- {

		var (
			hash    = newItems[i].Hash
			xmldata = newItems[i].ItemData
		)

		// errors on these do not cancel processing; the item is just not added to new item list for
		// download.. continue isn't needed here, but I'd rather be explicit on whats happening
		// in case anything is added in the future

		if handled, err := fup.checkExistingHash(hash, xmldata); (handled == true) || (err != nil) {
			if err != nil {
				fup.feed.log.Error(err)
				errors.Join(retErr, err)
			}
			continue

		} else if handled, err := fup.checkExistingGuid(hash, xmldata); (handled == true) || (err != nil) {
			if err != nil {
				fup.feed.log.Error(err)
				errors.Join(retErr, err)
			}
			continue

		} else if handled, err = fup.createNewEntry(hash, xmldata); (handled == true) || (err != nil) {
			if err != nil {
				fup.feed.log.Error(err)
				errors.Join(retErr, err)
			}
			continue
		}
	}
	return retErr
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
			log.Errorf("failed loading item xml: %v", err)
			return true, err
		} else if err := itemEntry.updateXmlData(hash, xmldata); err != nil {
			log.Errorf("failed updating xml data: %v", err)
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
		flog    = fup.feed.log
	)

	if itemEntry, exists := fup.guidCollList[xmldata.Guid]; exists {
		handled = true
		// guid collision, with no hash collision.. means the url has changed..
		flog.With(
			"previousguid", itemEntry.Guid,
			"newguid", xmldata.Guid,
			"oldhash", itemEntry.Hash,
			"newhash", hash,
			"oldUrl", itemEntry.Url,
			"newUrl", xmldata.Enclosure.Url,
		).Infof("guid collision detected with no hash collision; likely new url for same item")

		// hash will change.. filename might change if url is in filenameparse
		// filename might change based on filenameparse.. xml definitely changed (diff url)
		// pubtimestamp maybe change.. episode count should not change

		// make sure the previous is loaded for replacing the existing
		if err := itemEntry.loadItemXml(db); err != nil {
			flog.Errorf("failed loading item xml: %v", err)
			return true, err
		} else if err := itemEntry.updateFromEntry(fup.feed.FeedToml, hash, xmldata, fup.collisionFunc); err != nil {
			flog.Errorf("failed updating existing item entry; skipping: %v", err)
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
		log.Errorf("failed creating new item entry; skipping: %v", err)
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
func (fup feedUpdate) loadNewXml() ([]byte, error) {
	var (
		log        = fup.feed.log
		body       []byte
		err        error
		recentfile string
	)

	if config.UseMostRecentXml {
		// find the most recent xml based on the glob pattern

		if recentfile, err = podutils.FindMostRecent(filepath.Dir(fup.feed.xmlfile), fmt.Sprintf("%v.*.xml", fup.feed.Shortname)); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Warn("most recent file not found, doing a download")
			} else {
				log.Errorf("error finding most recent xml: %v", err)
				return nil, err
			}
		}
	}

	if recentfile != "" {
		log.Debugf("loading xml file: %v", recentfile)
		if body, err = podutils.LoadFile(recentfile); err != nil {
			log.Errorf("error loading xml file: %v", err)
			return nil, err
		}

	} else {
		// download from url, unbuffered
		if body, err = podutils.Download(fup.feed.Url); err != nil {
			log.Errorf("failed to download: %v", err)
			return nil, err
		}
	}

	if len(body) == 0 {
		err = errors.New("body length is zero")
		log.Error(err)
		return nil, err
	}
	return body, nil
}

// --------------------------------------------------------------------------
func (fup feedUpdate) saveAndRotateXml(body []byte, shouldRotate bool) {
	// for external reference
	if err := podutils.SaveToFile(body, fup.feed.xmlfile); err != nil {
		fup.feed.log.Errorf("failed saving xml file: %v", err)
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

	// usually feeds are reverse chronological, we can skip if we start seeing dupes
	// in the case where the feed is standard chrono, we can't do that
	if (fup.feed.StdChrono == false) && (config.MaxDupChecks >= 0) && (skip == true) {
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
	} else if fup.feed.XmlFeedData == nil {
		// likely new feed; not previously set
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
	} else if fup.feed.XmlFeedData == nil {
		// likely new feed, not prevously set.. no cancel
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
func (fup *feedUpdate) processFeedImage() error {

	// need to determine which url to use; /image/url or /itunes:image/href

	if fup.newXmlData == nil {
		return errors.New("xml data is nil")
	}
	var (
		imgUrl    = fup.newXmlData.Image.Url
		itunesUrl = fup.newXmlData.ItunesImageUrl
		log       = fup.feed.log
	)

	// slight warning if image.url != itunes:image href
	if imgUrl != "" && itunesUrl != "" && (imgUrl != itunesUrl) {
		log.With("imgUrl", imgUrl, "itunes:imgUrl", itunesUrl).Warn("image values are not equal.. figure out what to do")
	}

	if imgUrl == "" {
		imgUrl = fup.newXmlData.ItunesImageUrl
	}

	if imgUrl == "" {
		return errors.New("image url is blank")
	} else if imgEntry, err := fup.feed.getImage(imgUrl, fup.feed.genImageFilename()); err != nil {
		return err
	} else if err := fup.feed.setFeedImage(imgEntry); err != nil {
		return err
	} else {
		fup.feed.log.Debug("image set", "url", imgEntry.Url)
		return nil
	}
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) processItemImage(item *Item) error {

	var (
		f   = fup.feed
		log = fup.feed.log
	)
	if item.XmlData.Imageurl == "" {
		log.Debug("item image url is blank, nothing to download")
		return nil
	} else if imgEntry, err := f.getImage(item.XmlData.Imageurl, item.genImageFilename()); err != nil {
		return err
	} else if err := fup.feed.addImage(imgEntry); err != nil {
		return err
	} else {
		item.ImageKey = imgEntry.Url
		fup.feed.log.Debug("image set", "url", imgEntry.Url)

		return nil
	}
}

// --------------------------------------------------------------------------
func (fup *feedUpdate) downloadNewItems(results *DownloadResults) bool {

	var (
		f   = fup.feed
		log = f.log
		// by default; any errors will set this to false
		success       = true
		downloadAfter time.Time
		completed     = make([]*Item, 0, len(fup.newItems))
	)

	if config.DownloadAfter != "" {
		if date, err := dateparse.ParseAny(config.DownloadAfter); err != nil {
			werr := fmt.Errorf("downloadAfter not recognized: %w", err)
			log.With("downloadAfter", config.DownloadAfter).Error(werr)
			results.addError(werr)
			return false

		} else if date.IsZero() {
			log.Warn("download after date is zero")
		} else {
			downloadAfter = date
		}
	}

	for _, item := range fup.newItems {
		log.Debugf("processing new item: {%v : %v : %v}", item.Filename, item.Hash, path.Base(item.Url))

		podfile := filepath.Join(f.mp3Path, item.Filename)
		var fileExists bool

		// check download after flag; if set, only download items after given date..
		// anything before given date just mark as downloaded and archived
		if (downloadAfter.IsZero() == false) && (item.PubTimeStamp.Before(downloadAfter)) {
			log.Debugf("pubtimestamp before downloadAfter; skipping and marking as downloaded")
			item.Downloaded = true
			item.Archived = true
			completed = append(completed, item)
			continue
		}

		fileExists, err := podutils.FileExists(podfile)
		if err != nil {
			results.addError(fmt.Errorf("error in FileExists; not downloading: %v", err))
			success = false
			continue
		}

		if item.Downloaded == true {
			log.Debugf("item downloaded '%v', archived: '%v', fileExists: '%v'", item.Downloaded, item.Archived, fileExists)
			if fileExists == false {
				if item.Archived == true {
					log.Info("skipping download due to archived flag")
					continue
				} else {
					log.Warn("downloading item; archive flag not set")
				}
			} else {
				log.Debug("skipping download; file already downloaded.. ")
				continue
			}
		} else if fileExists == true {
			if config.MarkDownloaded {
				log.Info("file exists, and set downloaded flag set.. marking as downloaded")

				item.Downloaded = true
				completed = append(completed, item)

			} else {
				log.Warnf("item downloaded '%v', archived: '%v', fileExists: '%v'", item.Downloaded, item.Archived, fileExists)
				log.Warn("file already exists.. possible filename collision? skipping download")
			}
			continue
		}

		var bytes uint64

		if config.Simulate {
			log.Info("skipping downloading file due to sim flag")
			// fake the bytes downloaded
			if item.XmlData.Enclosure.Length == 0 {
				log.Warn("simulate flag, and download length in xml is 0")
			} else {
				bytes = uint64(item.XmlData.Enclosure.Length)
			}

		} else {
			if b, err := item.Download(f.mp3Path); err != nil {
				results.addError(fmt.Errorf("Error downloading file: %v", err))
				success = false
				continue
			} else {
				// get and save the pod image
				if err := fup.processItemImage(item); err != nil {
					log.Warn("error processing item image", "itemfilename", item.Filename, "image url", item.XmlData.Imageurl)
					// continuing; not erroring on image download
				}

				completed = append(completed, item)
				bytes = uint64(b)
			}
		}

		// add the success to the results
		results.TotalDownloaded++
		results.TotalDownloadedBytes += bytes
		results.Results[fup.feed.Shortname] = append(results.Results[fup.feed.Shortname], item.Filename)

		log.Infof("finished downloading file: %v", podfile)
	}

	// save items and images
	f.saveDBFeed(nil, completed)

	log.Info("all new downloads completed")
	return success
}
