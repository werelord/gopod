package pod

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "gopod/multilogger"
	"gopod/podconfig"
	"gopod/podutils"

	"github.com/araddon/dateparse"
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
	imgPath string

	lastModCache map[string]LastMod
	log          log.Logger
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

	ImageList []*ImageDBEntry `gorm:"foreignKey:FeedId"`
	imageMap  map[string]*ImageDBEntry
	etagMap   map[string]*ImageDBEntry
	ImageKey  string
}

type FeedXmlDBEntry struct {
	PodDBModel
	podutils.XChannelData `gorm:"embedded"`
}

type LastMod struct {
	Timestamp time.Time
	ETag      string
}

type ImageDBEntry struct {
	PodDBModel
	FeedId       uint
	Filename     string
	LastModified LastMod `gorm:"embedded;embeddedPrefix:LastModified_"`
	Url          string
	// Hash         string
}

const (
	lastModStr = "#lastmodified#"
	extStr     = "#ext#"
)

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

	f.imgPath = filepath.Join(config.WorkspaceDir, f.Shortname, ".img")
	if err := podutils.MkdirAll(f.imgPath); err != nil {
		f.log.Errorf("error making image directory: %v", err)
		return err
	}

	// make sure last modifed cache is created
	f.lastModCache = make(map[string]LastMod, 0)

	// even tho these are created in the db call, if its a new feed they won't exists yet
	f.imageMap = make(map[string]*ImageDBEntry, 0)
	f.etagMap = make(map[string]*ImageDBEntry, 0)

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
	{
		var title = ""
		if newxml != nil {
			title = newxml.Title
		}
		f.log.Info("Saving db", "xml", title, "itemCount", len(newitems))
	}

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

// --------------------------------------------------------------------------
func (f Feed) genImageFilename() string {
	return fmt.Sprintf("%v__%v%v", f.Shortname, lastModStr, extStr)
}

// --------------------------------------------------------------------------
func (f *Feed) setFeedImage(imgData *ImageDBEntry) error {
	if imgData == nil {
		return errors.New("image cannot be nil")
	} else if imgData.FeedId > 0 && imgData.FeedId != f.ID {
		f.log.Error("image feed id != feed's id", "imgFeedId", imgData.FeedId, "feedId", f.ID)
		return errors.New("image feed id != feed's id")
	}

	// set the current, and add it to the map
	f.ImageKey = imgData.Url
	return f.addImage(imgData)
}

// --------------------------------------------------------------------------
func (f *Feed) addImage(imgData *ImageDBEntry) error {
	if imgData == nil {
		return errors.New("image cannot be nil")
	} else if imgData.FeedId > 0 && imgData.FeedId != f.ID {
		f.log.Error("image feed id != feed's id", "imgFeedId", imgData.FeedId, "feedId", f.ID)
		return errors.New("image feed id != feed's id")
	}

	f.imageMap[imgData.Url] = imgData
	if imgData.LastModified.ETag != "" {
		f.etagMap[imgData.LastModified.ETag] = imgData
	}

	return nil

}

// --------------------------------------------------------------------------
// compares the latest image from the url to the current indicated in the feed
// if different, downloads that image and returns a ImageDBEntry pointer to that new image
// if same just returns the current entry (likely with db id and stuff)
func (f *Feed) getImage(urlStr string, imgFilename string) (*ImageDBEntry, error) {
	var (
		log    = f.log
		imgUrl string
	)

	// parse the url, remove querystring and fragments
	if u, err := url.ParseRequestURI(urlStr); err != nil {
		return nil, err
	} else {
		u.RawQuery = ""
		u.Fragment = ""
		imgUrl = u.String()
	}

	// check url against current list
	if img, exists := f.imageMap[imgUrl]; exists == false {

		// still check head request, in case different url but same etag
		if headEtag, err := f.getLastModified(imgUrl); err != nil {
			return nil, err
		} else if img, exists := f.etagMap[headEtag.ETag]; (headEtag.ETag == "") || (exists == false) {
			log.Debug("new url found, etag doesn't exist or is blank")
			return f.downloadImage(imgUrl, imgFilename)
		} else {
			log.Debug("new url found, but etag is the same; returning match")
			return img, nil
		}

	} else {
		if strings.EqualFold(f.ImageCompare, "filename") { // compare via filename only
			log.Debug("image compare set to filename, and urls match.. returning given image")
			return img, nil

		} else if headLastMod, err := f.getLastModified(imgUrl); err != nil {
			return nil, err

		} else if headLastMod.ETag != "" { // solely compare on etag
			if headLastMod.ETag == img.LastModified.ETag {
				log.Debug("etags match, returning current")
				return img, nil
			} else {
				log.Debug("etags do not match, downloading new image",
					"head etag", headLastMod.ETag,
					"previous etag", img.LastModified.ETag)
				return f.downloadImage(imgUrl, imgFilename)
			}

		} else { // compare on lastmodified dates
			if img.LastModified.Timestamp.IsZero() || headLastMod.Timestamp.IsZero() {
				log.Warn("last modified timestamp for current and latest is zero; downloading (etag is also empty)")
				return f.downloadImage(imgUrl, imgFilename)
			} else if headLastMod.Timestamp.After(img.LastModified.Timestamp) {
				log.Debug("head request after current, downloading new image",
					"head LM", headLastMod.Timestamp.Format(podutils.TimeFormatStr),
					"previous LM", img.LastModified.Timestamp.Format(podutils.TimeFormatStr))
				return f.downloadImage(imgUrl, imgFilename)
			} else if headLastMod.Timestamp.Equal(img.LastModified.Timestamp) {
				log.Debug("head request equals current, returning current")
				return img, nil
			} else { // before
				log.Warn("head request after current, downloading image",
					"head LM", headLastMod.Timestamp.Format(podutils.TimeFormatStr),
					"previous LM", img.LastModified.Timestamp.Format(podutils.TimeFormatStr))
				return f.downloadImage(imgUrl, imgFilename)
			}
		}
	}
}

// --------------------------------------------------------------------------
// gets the last modifed timestamp / ETag of uri
// uses cached value held in feed if found
// saves result in lastModCache if head is requested
func (f *Feed) getLastModified(url string) (LastMod, error) {
	if lastmodcache, exists := f.lastModCache[url]; exists {
		// lastmodified comes from previous
		return lastmodcache, nil
	} else {
		// peek new location to get last modified
		if lastmod, etag, err := podutils.GetLastModified(url); err != nil {
			f.log.Warnf("head request returned error: %v", err)
			return LastMod{time.Time{}, ""}, err
		} else {
			var lastmod = LastMod{lastmod, etag}
			f.lastModCache[url] = lastmod
			return lastmod, nil
		}
	}
}

// --------------------------------------------------------------------------
// downloads buffered, copies result to imgFilename
// saves last modified to lastModCache
func (f *Feed) downloadImage(imgUrl string, imgFilename string) (*ImageDBEntry, error) {
	var (
		log    = f.log
		newImg = ImageDBEntry{
			Url:          imgUrl,
			LastModified: LastMod{time.Time{}, ""},
		}
		ext        = path.Ext(imgUrl)
		tempImg    = fmt.Sprintf("imgTemp*%v", ext)
		lastmodstr = ""
		etag       = ""

		// generic function for getting last modified on request
		onResp = func(resp *http.Response) {
			// either lastmodified or ETag should be set
			lastmodstr = resp.Header.Get("last-modified")
			etag = resp.Header.Get("ETag")
		}
	)

	file, err := podutils.CreateTemp(f.imgPath, tempImg)
	if err != nil {
		log.Errorf("Failed creating temp file: %v", err)
		return nil, err
	}
	defer file.Close()

	if _, err := podutils.DownloadBuffered(imgUrl, file, onResp); err != nil {
		log.Error("failed downloading image", "err", err, "url", imgUrl)
		// delete the temp file
		file.Close()
		if e := os.Remove(file.Name()); e != nil {
			errors.Join(err, e)
		}
		return nil, err
		// } else {
		// 	log.Debug("file written", "filename", file.Name(), "bytes", podutils.FormatBytes(uint64(bw)))
	}
	file.Close() // explicit close

	// parse last modified / etag
	if lastmodstr != "" {
		if lm, err := dateparse.ParseAny(lastmodstr); err != nil {
			log.Errorf("parsing last modified timstamp err: %v", err, "timestamp", lastmodstr)
		} else {
			newImg.LastModified.Timestamp = lm
		}
	}
	if etag != "" {
		if etag[0] == '"' {
			etag = etag[1:]
		}
		if etag[len(etag)-1] == '"' {
			etag = etag[:len(etag)-1]
		}
		newImg.LastModified.ETag = etag
	}

	// generate filename (also sanity check); default to timestamp if exists
	var lms string
	if newImg.LastModified.Timestamp.IsZero() == false {
		lms = newImg.LastModified.Timestamp.Format(podutils.TimeFormatStr)
	} else if newImg.LastModified.ETag != "" {
		// limit filename length
		lms = newImg.LastModified.ETag[0:8]
	} else {
		log.Warn("lastmodified timestamp and ETag is empty")
		log.Warn("setting filename to zero timestamp")
		lms = time.Time{}.Format(podutils.TimeFormatStr)
	}
	newImg.Filename = strings.Replace(imgFilename, lastModStr, lms, 1)
	newImg.Filename = strings.Replace(newImg.Filename, extStr, ext, 1)
	newImg.Filename = podutils.CleanFilename(newImg.Filename, "_")

	// move the file
	if err := podutils.Rename(file.Name(), filepath.Join(f.imgPath, newImg.Filename)); err != nil {
		log.Error("error renaming file", "err", err)
		return nil, err
	} else {
		var ts = time.Now()
		if newImg.LastModified.Timestamp.IsZero() == false {
			ts = newImg.LastModified.Timestamp
		}

		if err := podutils.Chtimes(filepath.Join(f.imgPath, newImg.Filename), ts, ts); err != nil {
			log.Error("error changing last modified", "err", err)
			// return nil, err
		}
	}
	log.Debug("image file downloaded successfully", "file", newImg.Filename)

	// save the last modified to the cache, just to make sure
	f.lastModCache[newImg.Url] = newImg.LastModified

	return &newImg, nil

}
