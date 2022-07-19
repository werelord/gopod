package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"

	scribble "github.com/nanobox-io/golang-scribble"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

//--------------------------------------------------------------------------
type Feed struct {
	FeedToml

	FeedData XChannelData

	// local items
	config      *Config
	xmlfile     string
	mp3Path     string
	dbPath      string
	initialized bool
	itemlist    *orderedmap.OrderedMap[string, ItemData]
}

type FeedToml struct {
	Name      string `toml:"name"`
	Shortname string `toml:"shortname"`
	Url       string `toml:"url"`
}

type FeedDBExport struct {
	FeedData *Feed
	// using slice for json db output
	ItemListExport []ItemData
}

type ItemData struct {
	Hash       string
	Filename   string
	Url        string
	Downloaded bool
}

//--------------------------------------------------------------------------
var (
	db *scribble.Driver
)

// func (f Feed) Format(fs fmt.State, c rune) {
// 	fs.Write([]byte("Name:" + f.Shortname + " url: " + f.Url))
// }

//--------------------------------------------------------------------------
func (f *Feed) initFeed(config *Config) bool {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	f.config = config
	f.xmlfile = path.Join(config.workspace, f.Shortname+"."+config.timestampStr+".xml")
	f.mp3Path = path.Join(config.workspace, f.Shortname)
	f.dbPath = path.Join(config.workspace, f.Shortname, "db")

	f.itemlist = orderedmap.New[string, ItemData]()

	//log.Debug(f)

	f.initDB()

	return f.initialized
}

//--------------------------------------------------------------------------
func (f *Feed) initDB() {

	if f.initialized == false {
		var e error
		db, e = scribble.New(f.dbPath, nil)
		if e != nil {
			log.Error("Error init db: ", e)
			return
		}

		// todo: load database entry for feed

		feedImport := FeedDBExport{FeedData: f}

		// load feed information
		if e := db.Read("./", "feed", &feedImport); e != nil {
			log.Error("error reading feed info:", e)
			// don't return, just log the error
		} else {
			// populate ordered map
			for _, item := range feedImport.ItemListExport {
				f.itemlist.Set(item.Hash, item)
			}
		}

		//log.Debug(db)
		f.initialized = true

	}
}

//--------------------------------------------------------------------------
func (f *Feed) update(config Config) {

	var (
		body []byte
		err  error
	)
	// check to see if xml exists
	if config.Debug {
		if _, err = os.Stat(f.xmlfile); (config.Debug) && (err == nil) {
			body = loadXmlFile(f.xmlfile)

		} else {
			// download file
			body = f.downloadXml(f.xmlfile)
			saveXmlToFile(body, f.xmlfile)
		}

	} else {
		// download file
		body = f.downloadXml(f.xmlfile)
		saveXmlToFile(body, f.xmlfile)
	}

	// future: comparison operations for feedData?
	var itemList *orderedmap.OrderedMap[string, XItemData]
	f.FeedData, itemList, err = parseXml(body, f)

	if err != nil {
		log.Error("failed to parse xml: ", err)
		return
	}

	// maintain order on pairs; go from oldest to newest (each item moved to front)
	for pair := itemList.Newest(); pair != nil; pair = pair.Prev() {
		// all these should be new entries..
		var (
			hash       = pair.Key
			item       = pair.Value
			downloaded bool
			filename   string
			parsedUrl  string
			err        error
		)

		if config.Debug {
			if f.Shortname == "cppcast" {
				downloaded = true
			}
		}

		if filename, parsedUrl, err = parseUrl(item.enclosure.url); err != nil {
			log.Error("Failed parsing url, skipping entry: ", err)
			continue
		}

		var itemdata = ItemData{hash, filename, parsedUrl, downloaded}

		f.itemlist.Set(hash, itemdata)
		if e := f.itemlist.MoveToFront(hash); err != nil {
			log.Error("failed to move to front: ", e)
		}
		//f.ItemListExport = append(f.ItemListExport, itemdata)

		// todo: save item
		// todo: check download
	}

	f.saveDB()

	log.Debugf("%+v", f)
	//log.Debugf("%+v", foo)
}

//--------------------------------------------------------------------------
func (f Feed) saveDB() (err error) {

	// make sure database is initialized
	f.initDB()
	var feedExport FeedDBExport

	feedExport.FeedData = &f
	for pair := f.itemlist.Oldest(); pair != nil; pair = pair.Next() {
		feedExport.ItemListExport = append(feedExport.ItemListExport, pair.Value)
	}

	if e := db.Write("./", "feed", feedExport); e != nil {
		log.Error("failed to write database file: ", e)
		return e
	}

	return nil
}

//--------------------------------------------------------------------------
func parseUrl(urlstr string) (filename string, parsedUrl string, err error) {
	u, err := url.Parse(urlstr)
	if err != nil {
		log.Error("failed url parsing:", err)
		return "", "", err
	}

	// remove querystring/fragment
	u.RawQuery = ""
	u.Fragment = ""

	f := path.Base(u.Path)

	return f, u.String(), nil
}

//--------------------------------------------------------------------------
// todo: move this
func (f Feed) downloadXml(filename string) (body []byte) {
	log.Debug("downloading ", f.Url)

	var err error
	var resp *http.Response

	resp, err = http.Get(f.Url)

	if err != nil {
		log.Error("failed to get xml: ", err)
		return
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Error("failed to get response body: ", err)
		return
	}
	//log.Debug(string(body))
	return
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
func (f Feed) exists(hash string) (exists bool) {
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

func (f *Feed) saveItem(hash string, data ItemData) {
	// todo: this
}
