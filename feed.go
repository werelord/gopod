package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"

	//etree "github.com/beevik/etree"

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

	feedData ChannelData
	itemlist map[string]ItemData
}

//--------------------------------------------------------------------------
type ChannelData struct {
	title string
	pubdate time.Time
	link string
	image ChannelImage
	author string
	description string
}

//--------------------------------------------------------------------------
type ChannelImage struct {
	url string
	title string
	link string
}

//--------------------------------------------------------------------------
type ItemData struct {
	Title string
	pubdate time.Time
	guid string
	link string
	image string
	description string
	enclosure EnclosureData
}

type EnclosureData struct {
	length uint
	typeStr string
	url string
}

//--------------------------------------------------------------------------
var (
//f           Feed
)

//--------------------------------------------------------------------------
func (f *Feed) initFeed(config *Config) bool {

	if len(f.Shortname) == 0 {
		f.Shortname = f.Name
	}

	f.config = config
	f.xmlfile = path.Join(config.workspace, f.Shortname + "." + config.timestampStr + ".xml")
	f.mp3Path = path.Join(config.workspace, f.Shortname)
	f.dbPath = path.Join(config.workspace, f.Shortname, "db")

	//log.Debug(f)

	f.initDb()

	return f.initialized

}

//--------------------------------------------------------------------------
func (f *Feed) initDb() {

	if f.initialized == false {
		_, err := scribble.New(f.dbPath, nil)
		if err != nil {
			log.Error("Error init db: ", err)
			return
		}

		// todo: someting with database

		//log.Debug(db)
		f.initialized = true

		// todo: load existing setup

	}
}

//--------------------------------------------------------------------------
func (f Feed) update(config Config) {

	var (
		body []byte
		err  error
	)
	// check to see if xml exists
	if config.Debug {
		if _, err = os.Stat(f.xmlfile); (config.Debug) && (err == nil) {
			body = loadFile(f.xmlfile)

		} else {
			// download file
			body = f.downloadXml(f.xmlfile)
			saveToFile(body, f.xmlfile)
		}

	} else {
		// download file
		body = f.downloadXml(f.xmlfile)
		saveToFile(body, f.xmlfile)
	}

	f.parseXml(body)

}

//--------------------------------------------------------------------------
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
func saveToFile(buf []byte, filename string) {

	log.Debug("Saving to file: " + filename)

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 666)
	defer file.Close()

	if err != nil {
		log.Error("error opening file for writing:", err)
		return
	}
	count, err := file.Write(buf)
	if err != nil {
		log.Error("error writing bytes to file: ", err)

	} else {
		log.Debug("bytes written to file: " + fmt.Sprint(count))
	}

}

//--------------------------------------------------------------------------
// todo: move this, make config use this
func loadFile(filename string) (buf []byte) {

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
