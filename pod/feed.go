package pod

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopod/podconfig"
	"gopod/podutils"

	"github.com/google/uuid"
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
	ItemListExport []*ItemData
}

// exported fields for database in feed list
type ItemData struct {
	Hash         string
	Filename     string
	Url          string
	Downloaded   bool
	CDFilename   string // content-disposition filename
	PubTimeStamp time.Time
}

// exported fields for each item
type ItemExport struct {
	Hash        string
	ItemXmlData podutils.XItemData
}

//------------------------------------- DEBUG -------------------------------------
const (
	DOWNLOADFILE        = true
	SAVEDATABASE        = true
	numXmlFilesRetained = 5
)

var (
	config *podconfig.Config
)

//------------------------------------- DEBUG -------------------------------------

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
		// todo: error occurs on new feed, handle more gracefully
		if e := f.db.Read("./", "feed", &feedImport); e != nil {
			log.Warn("error reading feed info (new feed?):", e)
			// don't return, just log the error
		} else {
			// populate ordered map
			for _, item := range feedImport.ItemListExport {
				f.itemlist.Set(item.Hash, item)
			}
		}

		f.dbinitialized = true

	}
}

//--------------------------------------------------------------------------
func (f Feed) saveDB() (err error) {

	log.Info("Saving db for ", f.Shortname)

	//------------------------------------- DEBUG -------------------------------------
	if config.Debug && SAVEDATABASE == false {
		log.Debug("skipping saving database due to flag")
		return
	}
	//------------------------------------- DEBUG -------------------------------------

	// make sure database is initialized
	f.initDB()
	var feedExport FeedDBExport

	feedExport.Feed = &f
	for pair := f.itemlist.Oldest(); pair != nil; pair = pair.Next() {
		feedExport.ItemListExport = append(feedExport.ItemListExport, pair.Value)
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
		body     []byte
		err      error
		newItems []*ItemData
	)
	// check to see if xml exists
	//------------------------------------- DEBUG -------------------------------------
	// if config.Debug {
	// 	if _, err = os.Stat(f.xmlfile); err == nil {
	// 		body = loadXmlFile(f.xmlfile)

	// 	} else {
	// 		// download file
	// 		if body, err = podutils.Download(f.Url); err != nil {
	// 			log.Error(err)
	// 			return
	// 		}
	// 		saveXmlToFile(body, f.xmlfile)
	// 	}
	// 	//------------------------------------- DEBUG -------------------------------------
	// } else {
	//download file
	if body, err = podutils.Download(f.Url); err != nil {
		log.Error("failed to download: ", err)
		return
	}
	saveXmlToFile(body, f.xmlfile)

	// todo: this
	//RotateFiles(f.xmlfile)
	// }

	// future: comparison operations for feedData?
	var itemList *orderedmap.OrderedMap[string, podutils.XItemData]
	f.XMLFeedData, itemList, err = podutils.ParseXml(body, f)

	if err != nil {
		log.Error("failed to parse xml: ", err)
		return
	}

	// check url vs atom link & new feed url
	// TODO: handle this
	if f.XMLFeedData.AtomLinkSelf.Href != "" && f.Url != f.XMLFeedData.AtomLinkSelf.Href {
		log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XMLFeedData.AtomLinkSelf.Href)
	} else if f.XMLFeedData.NewFeedUrl != "" && f.Url != f.XMLFeedData.NewFeedUrl {
		log.Warnf("Feed url possibly changing: '%v':'%v'", f.Url, f.XMLFeedData.NewFeedUrl)
	}

	// maintain order on pairs; go from oldest to newest (each item moved to front)
	for pair := itemList.Newest(); pair != nil; pair = pair.Prev() {
		// all these should be new entries..
		var (
			hash        = pair.Key
			xmldata     = pair.Value
			urlfilename string
			genfilename string
			parsedUrl   string
			err         error
		)

		if urlfilename, parsedUrl, err = f.parseUrl(xmldata.Enclosure.Url); err != nil {
			log.Error("Failed parsing url, skipping entry: ", err)
			continue
		}

		if genfilename, err = f.generateFilename(xmldata, urlfilename); err != nil {
			log.Error("failed to generate filename:", err)
			// to make sure we can continue, shortname.uuid.mp3
			genfilename = f.Shortname + "." + strings.ReplaceAll(uuid.NewString(), "-", "") + ".mp3"
		}

		var itemdata = ItemData{
			Hash:         hash,
			Filename:     genfilename,
			Url:          parsedUrl,
			Downloaded:   false,
			PubTimeStamp: xmldata.Pubdate}

		log.Infof("adding new item: :%+v", itemdata)

		f.itemlist.Set(hash, &itemdata)
		if e := f.itemlist.MoveToFront(hash); err != nil {
			log.Error("failed to move to front: ", e)
		}

		// saving item xml
		f.saveItemXml(itemdata, xmldata)

		// download these in order newest to last, to hijack initial population of downloaded items
		newItems = append([]*ItemData{&itemdata}, newItems...)

	}

	f.saveDB()

	// process new entries
	f.processNew(newItems)

}

//--------------------------------------------------------------------------
func (f Feed) parseUrl(urlstr string) (filename string, parsedUrl string, err error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		log.Error("failed url parsing:", err)
		return "", "", err
	}

	// remove querystring/fragment
	u.RawQuery = ""
	u.Fragment = ""

	// handle url parsing, if needed
	if f.UrlParse != "" {
		// assuming host is direct domain..
		trim := strings.SplitAfterN(u.Path, f.UrlParse, 2)
		if len(trim) == 2 {
			u.Host = f.UrlParse
			u.Path = trim[1]
		} else {
			log.Warn("failed parsing url; split failed")
			log.Warnf("url: '%v' urlParse: '%v'", u.String(), f.UrlParse)
		}
	}

	fname := filepath.Base(u.Path)

	return fname, u.String(), nil
}

//--------------------------------------------------------------------------
func (f Feed) generateFilename(xmldata podutils.XItemData, urlfilename string) (string, error) {
	// check to see if we neeed to parse.. simple search/replace

	if f.FilenameParse != "" {
		newstr := f.FilenameParse

		if strings.Contains(f.FilenameParse, "#shortname#") {
			newstr = strings.Replace(newstr, "#shortname#", f.Shortname, 1)
		}
		if strings.Contains(f.FilenameParse, "#linkfinalpath#") {
			// get the final path portion from the link url

			if u, err := url.Parse(xmldata.Link); err == nil {
				finalLink := path.Base(u.Path)
				newstr = strings.Replace(newstr, "#linkfinalpath#", podutils.CleanFilename(finalLink), 1)
			} else {
				log.Error("failed to parse link path.. not replacing:", err)
				return "", err
			}
		}

		if strings.Contains(f.FilenameParse, "#episode#") {
			var padLen = 3
			if f.EpisodePad > 0 {
				padLen = f.EpisodePad
			}
			rep := xmldata.EpisodeStr
			//------------------------------------- DEBUG -------------------------------------
			if config.Debug && f.Shortname == "russo" {
				// grab the episode from the title, as the numbers don't match for these
				r, _ := regexp.Compile("The Russo-Souhan Show ([0-9]*) - ")
				eps := r.FindStringSubmatch(xmldata.Title)
				if len(eps) == 2 {
					rep = eps[1]
				}
			}
			//------------------------------------- DEBUG -------------------------------------

			if rep == "" {
				//------------------------------------- DEBUG -------------------------------------
				// hack.. don't like this specific, but fuck it
				if f.Shortname == "dfo" {
					if r, err := regexp.Compile("([0-9]+)"); err == nil {
						matchslice := r.FindStringSubmatch(xmldata.Title)
						if len(matchslice) > 0 && matchslice[len(matchslice)-1] != "" {
							rep = matchslice[len(matchslice)-1]
							rep = strings.Repeat("0", padLen-len(rep)) + rep
						}
					}
				}
				//------------------------------------- DEBUG -------------------------------------
				if rep == "" { // still
					// use date as a stopgap
					rep = xmldata.Pubdate.Format("20060102")
					//rep = strings.Repeat("X", padLen)
				}

			} else if len(rep) < padLen {
				// pad string with zeros minus length
				rep = strings.Repeat("0", padLen-len(rep)) + rep
			}
			newstr = strings.Replace(newstr, "#episode#", podutils.CleanFilename(rep), 1)
		}

		if strings.Contains(f.FilenameParse, "#date#") {
			// date format YYYYMMDD
			newstr = strings.Replace(newstr, "#date#", xmldata.Pubdate.Format("20060102"), 1)
		}

		if strings.Contains(f.FilenameParse, "#titleregex:") {
			if parsed, err := f.titleSubmatchRegex(f.Regex, newstr, xmldata.Title); err != nil {
				log.Error("failed parsing title:", err)
				return "", err
			} else {
				newstr = podutils.CleanFilename(strings.ReplaceAll(parsed, " ", "_"))
			}
		}

		if strings.Contains(f.FilenameParse, "#urlfilename#") {
			// for now, only applies to urlfilename
			if f.SkipFileTrim == false {
				urlfilename = podutils.CleanFilename(urlfilename)
			}
			newstr = strings.Replace(newstr, "#urlfilename#", urlfilename, 1)
		}

		log.Debug("using generated filename: ", newstr)
		return newstr, nil
	}

	// fallthru to default
	return podutils.CleanFilename(urlfilename), nil
}

//--------------------------------------------------------------------------
// todo: move this
func saveXmlToFile(buf []byte, filename string) {

	log.Debug("Saving to file: " + filename)

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	if err != nil {
		log.Error("error opening file for writing:", err)
		return
	}
	defer file.Close()
	count, err := file.Write(buf)
	if err != nil {
		log.Error("error writing bytes to file: ", err)

	} else {
		log.Debug("bytes written to file: " + fmt.Sprint(count))
	}
}

//--------------------------------------------------------------------------
// feedProcess implementation
//--------------------------------------------------------------------------
func (f Feed) ItemExists(hash string) (exists bool) {
	_, exists = f.itemlist.Get(hash)
	return
}

//--------------------------------------------------------------------------
func (f Feed) MaxDuplicates() uint {
	return config.MaxDupChecks
}

//--------------------------------------------------------------------------
func (f Feed) CheckTimestamp(t time.Time) bool {
	// todo: this
	return true
}

//--------------------------------------------------------------------------
// todo: move this, make config use this
func loadXmlFile(filename string) (buf []byte) {

	log.Debug("loading data from file: " + filename)
	file, err := os.Open(filename)
	if err != nil {
		log.Error("failed to open "+filename+": ", err)
	} else {
		defer file.Close()
		buf, err = io.ReadAll(file)
		if err != nil {
			log.Error("failed to open "+filename+": ", err)
		}
	}

	return
}

//--------------------------------------------------------------------------
func (f *Feed) saveItemXml(item ItemData, xmldata podutils.XItemData) (err error) {
	log.Infof("saving xmldata for %v{%v}", item.Filename, item.Hash)

	if config.Debug && SAVEDATABASE == false {
		log.Debug("skipping saving database due to flag")
		return
	}

	// make sure db is init
	f.initDB()

	jsonFile := strings.TrimSuffix(item.Filename, filepath.Ext(item.Filename))

	var i ItemExport
	i.Hash = item.Hash
	i.ItemXmlData = xmldata

	if e := f.db.Write("items", jsonFile, i); e != nil {
		log.Error("failed to write database file: ", e)
		return e
	}

	return nil
}

//--------------------------------------------------------------------------
func (f *Feed) processNew(newItems []*ItemData) {
	// todo: this

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

		if config.Debug && DOWNLOADFILE == false {
			log.Debug("skipping downloading file due to flag")
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
		// todo: change change modified time
	}

	log.Info("all new downloads completed, saving db")
	f.saveDB()

}
