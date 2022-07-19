package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	scribble "github.com/nanobox-io/golang-scribble"
)

//--------------------------------------------------------------------------
type Feed struct {
	Name      string `toml:"name"`
	Shortname string `toml:"shortname"`
	Url       string `toml:"url"`

	config *Config

	xmlfile     string
	mp3Path     string
	dbPath      string
	initialized bool

	FeedData XChannelData
	Itemlist map[string]ItemData
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

//--------------------------------------------------------------------------
func (f *Feed) initFeed(config *Config) bool {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	f.config = config
	f.xmlfile = path.Join(config.workspace, f.Shortname+"."+config.timestampStr+".xml")
	f.mp3Path = path.Join(config.workspace, f.Shortname)
	f.dbPath = path.Join(config.workspace, f.Shortname, "db")

	f.Itemlist = make(map[string]ItemData)

	//log.Debug(f)

	f.initDb()

	return f.initialized
}

//--------------------------------------------------------------------------
func (f *Feed) initDb() {

	if f.initialized == false {
		var e error
		db, e = scribble.New(f.dbPath, nil)
		if e != nil {
			log.Error("Error init db: ", e)
			return
		}

		// todo: load database entry for feed

		//log.Debug(db)
		f.initialized = true

		// todo: load existing setup
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

	// todo: comparison operations?
	var itemList map[string]XItemData
	f.FeedData, itemList, err = parseXml(body, f)

	if err != nil {
		log.Error("failed to parse xml: ", err)
		return
	}

	for k, _ := range itemList {
		// all these should be new entries..
		f.Itemlist[k] = ItemData{k, "todo:this", "todo:this", false}

		// todo: save v
		// todo: check download
	}

	if e := db.Write("./", "feed", f); e != nil {
		log.Error("failed to write database file: ", e)
	}

	log.Debugf("%+v", f)
	//log.Debugf("%+v", foo)
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
	_, exists = f.Itemlist[hash]
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
