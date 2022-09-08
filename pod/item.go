package pod

import (
	"errors"
	"fmt"
	"gopod/podconfig"
	"gopod/poddb"
	"gopod/podutils"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

// exported fields for database in feed list
type Item struct {
	Hash string
	ItemData

	itemInternal

	xmlData *podutils.XItemData
}

type itemInternal struct {
	parentShortname string // for logging purposes

	db       *poddb.PodDB
	dbDataId string // references the id for the data entry in the db
	dbXmlId  string // references the id for the xml entry in the db
}

type ItemData struct {
	Filename     string
	Url          string
	Downloaded   bool
	CDFilename   string // content-disposition filename
	PubTimeStamp time.Time
}

type ItemDataDBEntry struct {
	Hash     string
	ItemData ItemData
}

type ItemXmlDBEntry struct {
	Hash    string
	ItemXml podutils.XItemData
}

// --------------------------------------------------------------------------
func (i Item) Format(fs fmt.State, c rune) {
	str := fmt.Sprintf("Hash:'%v' Filename:'%v' Downloaded:%v PubTimeStamp:'%v'",
		i.Hash, i.Filename, i.Downloaded, i.PubTimeStamp)
	fs.Write([]byte(str))
}

// --------------------------------------------------------------------------
func createNewItemEntry(parentConfig podconfig.FeedToml,
	db *poddb.PodDB,
	hash string,
	xmlData *podutils.XItemData) (*Item, error) {
	// new entry, xml coming from feed directly
	// if this is loaded from the database, ItemExport should be nil

	// todo: nil checks

	item := Item{}

	item.parentShortname = parentConfig.Shortname
	item.Hash = hash
	item.xmlData = xmlData
	item.db = db
	item.PubTimeStamp = xmlData.Pubdate

	// parse url
	if err := item.parseUrl(parentConfig.UrlParse); err != nil {
		log.Error("failed parsing url: ", err)
		return nil, err
	} else if err := item.generateFilename(parentConfig); err != nil {
		log.Error("failed to generate filename:", err)
		// to make sure we can continue, shortname.uuid.mp3
		item.Filename = parentConfig.Shortname + "." + strings.ReplaceAll(uuid.NewString(), "-", "") + ".mp3"
	}

	// everything should be set

	return &item, nil
}

// --------------------------------------------------------------------------
func createItemDataDBEntry() any {
	// new db entry used for db queries
	return &ItemDataDBEntry{}
}

// --------------------------------------------------------------------------
func loadFromDBEntry(parentCfg podconfig.FeedToml, db *poddb.PodDB,
	entry poddb.DBEntry) (*Item, error) {

	item := Item{}

	item.parentShortname = parentCfg.Shortname
	item.db = db

	// generate filename, parsedurl, etc loaded from db entry

	item.dbDataId = *entry.ID
	e, ok := entry.Entry.(*ItemDataDBEntry)
	if ok == false {
		return nil, errors.New("failed loading item; db entry is not *ItemDataDBEntry")
	}
	item.Hash = e.Hash
	item.ItemData = e.ItemData

	// sanity check
	if item.Hash == "" {
		return nil, errors.New("failed loading item; hash is empty")
	} else if item.Filename == "" {
		return nil, errors.New("failed loading item; filename is empty (db corrupt?)")
	}

	//log.Debugf("Item Loaded: %v", item)

	return &item, nil
}

// --------------------------------------------------------------------------
func (i *Item) parseUrl(urlparse string) (err error) {

	log.Trace("here")
	var urlstr = i.xmlData.Enclosure.Url

	u, err := url.ParseRequestURI(urlstr)
	if err != nil {
		log.Error("failed url parsing:", err)
		return err
	}

	// remove querystring/fragment
	u.RawQuery = ""
	u.Fragment = ""

	// handle url parsing, if needed
	if urlparse != "" {
		// assuming host is direct domain..
		trim := strings.SplitAfterN(u.Path, urlparse, 2)
		if len(trim) == 2 {
			u.Host = urlparse
			u.Path = trim[1]
		} else {
			log.Warn("failed parsing url; split failed")
			log.Warnf("url: '%v' UrlParse: '%v'", u.String(), urlparse)
		}
	}

	i.Url = u.String()

	return nil
}

func (i *Item) updateXmlData(hash string, data *podutils.XItemData) {

	if i.Hash != hash {
		log.Warn("Hashes do not match; something is wrong")
		log.Warnf("itemhash:'%v' newhash:'%v'", i.Hash, hash)
	}

	// for now, just set the xml data
	// todo : deep compare??
	i.xmlData = data

	// if the url changes, this would be a new hash..
	// if the guid changes it would be a new hash..
	// no need to urlparse or regenerate filename
}

// --------------------------------------------------------------------------
func (i *Item) getItemXmlDBEntry() *poddb.DBEntry {
	// todo: can we pass id as reference, which would automatically update??
	var entry = poddb.DBEntry{
		ID: &i.dbXmlId,
		Entry: &ItemXmlDBEntry{
			Hash:    i.Hash,
			ItemXml: *i.xmlData,
		},
	}
	return &entry
}

// --------------------------------------------------------------------------
func (i *Item) getItemDataDBEntry() *poddb.DBEntry {
	var entry = poddb.DBEntry{
		ID: &i.dbDataId,
		Entry: &ItemDataDBEntry{
			Hash:     i.Hash,
			ItemData: i.ItemData,
		},
	}
	return &entry
}

//--------------------------------------------------------------------------
func (i Item) createProgressBar() *progressbar.ProgressBar {
	bar := progressbar.NewOptions64(0, //
		progressbar.OptionSetDescription(i.Filename),
		progressbar.OptionFullWidth(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }),
		progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerHead: ">", SaucerPadding: " ", BarStart: "[", BarEnd: "]"}))

	return bar

}

//--------------------------------------------------------------------------
func (i *Item) Download(mp3path string) error {

	var (
		downloadTimestamp = time.Now()
		destfile          = filepath.Join(mp3path, i.Filename)
	)

	file, err := podutils.CreateTemp(filepath.Dir(destfile), filepath.Base(destfile)+"_temp*")
	if err != nil {
		log.Error("Failed creating temp file: ", err)
		return err
	}
	defer file.Close()

	if bw, cd, err := podutils.DownloadBuffered(i.Url, file, i.createProgressBar()); err != nil {
		log.Error("Failed downloading pod:", err)
		return err
	} else {
		log.Debugf("file written {%v} bytes: %.2fKB", filepath.Base(file.Name()), float64(bw)/(1<<10))

		if strings.Contains(cd, "filename") {
			// content disposition header, for the hell of it
			if r, err := regexp.Compile("filename=\"(.*)\""); err == nil {
				if matches := r.FindStringSubmatch(cd); len(matches) == 2 {
					i.CDFilename = matches[1]
				}
			} else {
				log.Warn("parsing content disposition had regex error: ", err)
			}
		}
	}
	// explicit close before rename
	file.Close()

	// move tempfile to finished file
	if err = podutils.Rename(file.Name(), destfile); err != nil {
		log.Debug("error moving temp file: ", err)
		return err
	}

	if err := podutils.Chtimes(destfile, downloadTimestamp, i.PubTimeStamp); err != nil {
		log.Error("failed to change modified time: ", err)
		// don't skip due to timestamp issue
	}

	i.Downloaded = true
	return nil
}
