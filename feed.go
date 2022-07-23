package main

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	scribble "github.com/nanobox-io/golang-scribble"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

//--------------------------------------------------------------------------
type Feed struct {
	// toml information, extracted from config
	FeedToml

	// todo: archive flag

	// internal, local to feed, not serialized (explicitly)
	feedInternal

	// channel entries from xml, exported
	XMLFeedData XChannelData
}

type feedInternal struct {
	// local items
	db            *scribble.Driver
	config        *Config
	xmlfile       string
	mp3Path       string
	dbPath        string
	dbinitialized bool
	// itemlist is not explicitly exported, but converted to array to be exported
	itemlist *orderedmap.OrderedMap[string, *ItemData]
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
	PubTimeStamp time.Time
}

// exported fields for each item
type ItemExport struct {
	Hash        string
	ItemXmlData XItemData
}

//------------------------------------- DEBUG -------------------------------------
const (
	DOWNLOADFILE = false
	SAVEDATABASE = false
)

//------------------------------------- DEBUG -------------------------------------

// func (f Feed) Format(fs fmt.State, c rune) {
// 	fs.Write([]byte("Name:" + f.Shortname + " url: " + f.Url))
// }

//--------------------------------------------------------------------------
func (f *Feed) initFeed(config *Config) bool {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	f.config = config
	f.dbPath = path.Join(config.workspace, f.Shortname, "db")
	f.xmlfile = path.Join(f.dbPath, f.Shortname+"."+config.timestampStr+".xml")
	f.mp3Path = path.Join(config.workspace, f.Shortname)

	f.itemlist = orderedmap.New[string, *ItemData]()

	f.initDB()

	return f.dbinitialized
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
			log.Error("error reading feed info:", e)
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
	if f.config.Debug && SAVEDATABASE == false {
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
func (f *Feed) update() {

	var (
		body     []byte
		err      error
		newItems []*ItemData
	)
	// check to see if xml exists
	//------------------------------------- DEBUG -------------------------------------
	if f.config.Debug {
		if _, err = os.Stat(f.xmlfile); (f.config.Debug) && (err == nil) {
			body = loadXmlFile(f.xmlfile)

		} else {
			// download file
			body, _ = Download(f.Url)
			saveXmlToFile(body, f.xmlfile)
		}
		//------------------------------------- DEBUG -------------------------------------
	} else {
		// download file
		body, _ = Download(f.Url)
		saveXmlToFile(body, f.xmlfile)
	}

	// future: comparison operations for feedData?
	var itemList *orderedmap.OrderedMap[string, XItemData]
	f.XMLFeedData, itemList, err = parseXml(body, f)

	if err != nil {
		log.Error("failed to parse xml: ", err)
		return
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

		genfilename = f.generateFilename(xmldata, urlfilename)

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

	fname := path.Base(u.Path)

	return fname, u.String(), nil
}

//--------------------------------------------------------------------------
func (f Feed) generateFilename(xmldata XItemData, urlfilename string) string {
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
				newstr = strings.Replace(newstr, "#linkfinalpath#", finalLink, 1)
			}
		}
		if strings.Contains(f.FilenameParse, "#episode#") {
			var padLen = 3
			rep := xmldata.EpisodeStr
			if rep == "" {
				rep = strings.Repeat("X", padLen)
			} else if len(rep) < padLen {
				// pad string with zeros minus length
				rep = strings.Repeat("0", padLen-len(rep)) + rep
			}
			newstr = strings.Replace(newstr, "#episode#", rep, 1)
		}
		if strings.Contains(f.FilenameParse, "#urlfileregex#") {
			// regex parse of url filename, insert submatch into filename
			r, _ := regexp.Compile(f.Regex)
			match := cleanFilename(r.FindStringSubmatch(urlfilename)[r.NumSubexp()])
			// finally, replace strings with underscores; probably not necessary for this, but meh
			match = strings.ReplaceAll(match, " ", "_")
			newstr = strings.Replace(newstr, "#urlfileregex#", match, 1)
		}
		if strings.Contains(f.FilenameParse, "#titleregex#") {
			r, _ := regexp.Compile(f.Regex)
			match := cleanFilename(r.FindStringSubmatch(xmldata.Title)[r.NumSubexp()])
			// finally, replace strings with underscores
			match = strings.ReplaceAll(match, " ", "_")
			newstr = strings.Replace(newstr, "#titleregex#", match, 1)
		}
		if strings.Contains(f.FilenameParse, "#urlfilename#") {
			newstr = strings.Replace(newstr, "#urlfilename#", urlfilename, 1)
		}

		log.Debug("using generated filename: ", newstr)
		return newstr
	}

	// fallthru to default
	return urlfilename
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
func (f Feed) itemExists(hash string) (exists bool) {
	_, exists = f.itemlist.Get(hash)
	return
}

//--------------------------------------------------------------------------
func (f Feed) maxDuplicates() uint {
	return f.config.MaxDupChecks
}

//--------------------------------------------------------------------------
func (f Feed) checkTimestamp(t time.Time) bool {
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
func (f *Feed) saveItemXml(item ItemData, xmldata XItemData) (err error) {
	log.Infof("saving xmldata for %v{%v}", item.Filename, item.Hash)

	if f.config.Debug && SAVEDATABASE == false {
		log.Debug("skipping saving database due to flag")
		return
	}

	// make sure db is init
	f.initDB()

	jsonFile := strings.TrimSuffix(item.Filename, path.Ext(item.Filename))

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

		podfile := path.Join(f.mp3Path, item.Filename)
		downloadTimestamp := time.Now()
		var fileExists bool

		if _, err := os.Stat(podfile); err == nil {
			fileExists = true
		}

		//------------------------------------- DEBUG -------------------------------------
		if f.config.Debug && skipRemaining {
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
			if f.config.Debug {
				log.Debug("debug setup, setting skip remaining to true")
				skipRemaining = true
			}
			//------------------------------------- DEBUG -------------------------------------

			continue
		}

		if f.config.Debug && DOWNLOADFILE == false {
			log.Debug("skipping downloading file due to flag")
			continue
		}

		if err := DownloadBuffered(item.Url, podfile); err != nil {
			log.Error("Failed downloading pod:", err)
			continue
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
