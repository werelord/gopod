package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"

	scribble "github.com/nanobox-io/golang-scribble"
	"github.com/schollz/progressbar/v3"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

//--------------------------------------------------------------------------
type Feed struct {
	// toml information, extracted from config
	FeedToml

	// internal, local to feed, not serialized (explicitly)
	feedInternal

	// channel entries from xml, exported
	XMLFeedData XChannelData
}

type FeedToml struct {
	Name      string `toml:"name"`
	Shortname string `toml:"shortname"`
	Url       string `toml:"url"`
}

type feedInternal struct {
	// local items
	db          *scribble.Driver
	config      *Config
	xmlfile     string
	mp3Path     string
	dbPath      string
	initialized bool
	// itemlist is not explicitly exported, but converted to array to be exported
	itemlist *orderedmap.OrderedMap[string, ItemData]
}

// exported fields for database serialization
type FeedDBExport struct {
	Feed *Feed
	// using slice for json db output
	ItemListExport []ItemData
}

// exported fields for database in feed list
type ItemData struct {
	Hash       string
	Filename   string
	Url        string
	Downloaded bool
}

// exported fields for each item
type ItemExport struct {
	Hash        string
	ItemXmlData XItemData
}

//--------------------------------------------------------------------------
var ()

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
		log.Infof("{%v} initializing feed db, path: %v", f.Shortname, f.dbPath)
		var e error
		f.db, e = scribble.New(f.dbPath, nil)
		if e != nil {
			log.Error("Error init db: ", e)
			return
		}

		// todo: load database entry for feed

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

		//log.Debug(db)
		f.initialized = true

	}
}

//--------------------------------------------------------------------------
func (f Feed) saveDB() (err error) {

	log.Info("Saving db for ", f.Shortname)
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
		newItems []ItemData
	)
	// check to see if xml exists
	if f.config.Debug {
		if _, err = os.Stat(f.xmlfile); (f.config.Debug) && (err == nil) {
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
	f.XMLFeedData, itemList, err = parseXml(body, f)

	if err != nil {
		log.Error("failed to parse xml: ", err)
		return
	}

	// maintain order on pairs; go from oldest to newest (each item moved to front)
	for pair := itemList.Newest(); pair != nil; pair = pair.Prev() {
		// all these should be new entries..
		var (
			hash       = pair.Key
			xmldata    = pair.Value
			downloaded bool
			filename   string
			parsedUrl  string
			err        error
		)

		if f.config.Debug {
			if f.Shortname == "cppcast" {
				//downloaded = true
			}
		}

		if filename, parsedUrl, err = parseUrl(xmldata.Enclosure.Url); err != nil {
			log.Error("Failed parsing url, skipping entry: ", err)
			continue
		}

		var itemdata = ItemData{hash, filename, parsedUrl, downloaded}

		log.Infof("adding new item: :%+v", itemdata)

		f.itemlist.Set(hash, itemdata)
		if e := f.itemlist.MoveToFront(hash); err != nil {
			log.Error("failed to move to front: ", e)
		}

		// saving item xml
		f.saveItemXml(itemdata, xmldata)

		newItems = append(newItems, itemdata)

		// todo: save new items
		// todo: check download
	}
	// temporary; don't save database for testing purposes
	if f.config.Debug == false {
		f.saveDB()
	}

	// process new entries
	f.processNew(newItems)

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

//--------------------------------------------------------------------------
func (f *Feed) saveItemXml(item ItemData, xmldata XItemData) (err error) {
	log.Infof("saving xmldata for %v{%v}", item.Filename, item.Hash)

	// make sure db is init
	f.initDB()

	jsonFile := strings.TrimSuffix(item.Filename, path.Ext(item.Filename))

	var i ItemExport
	i.Hash = item.Hash
	i.ItemXmlData = xmldata

	if e := f.db.Write("./", jsonFile, i); e != nil {
		log.Error("failed to write database file: ", e)
		return e
	}

	return nil
}

//--------------------------------------------------------------------------
func (f *Feed) processNew(newItems []ItemData) (err error) {
	// todo: this

	for _, pod := range newItems {
		log.Debugf("processing new item: {%v %v}", pod.Filename, pod.Hash)

		tempfile, err := f.downloadPod(pod)
		if err != nil {
			log.Error("Failed downloading pod:", err)
			continue
		}
		log.Debug("finished downloading file: ", tempfile)
		// todo: change change modified time
	}
	return nil
}

//--------------------------------------------------------------------------
// todo: move this
func (f *Feed) downloadPod(item ItemData) (filepath string, err error) {

	// todo: check to see if file exists, is downloaded

	resp, err := http.Get(item.Url)

	if err != nil {
		log.Error("Failed to download pod episodeS: ", err)
		return
	}
	defer resp.Body.Close()

	file, err := os.CreateTemp(f.mp3Path, item.Filename+".*")
	if err != nil {
		log.Error("Failed creating temp file: ", err)
		return
	}
	defer file.Close()

	bar := progressbar.NewOptions64(resp.ContentLength,
		progressbar.OptionSetDescription("downloading "+item.Filename),
		progressbar.OptionFullWidth(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerPadding: " ", BarStart: "[", BarEnd: "]"}))

	podWriter := bufio.NewWriter(file)
	b, err := io.Copy(io.MultiWriter(podWriter, bar), resp.Body)
	podWriter.Flush()
	if err != nil {
		log.Error("error in writing file: ", err)
	} else {
		log.Debugf("file written, bytes: %.2fKB", float64(b)/(1<<10))
	}

	return path.Clean(file.Name()), nil
}
