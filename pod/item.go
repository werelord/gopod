package pod

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopod/podconfig"
	"gopod/podutils"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
	log "github.com/sirupsen/logrus"
)

type Item struct {
	// all db entities, exported
	ItemDBEntry

	// all internal, non-exported (no db saving) items
	itemInternal
}

type itemInternal struct {
	parentShortname string // for logging purposes

	log *log.Entry
}

type ItemDBEntry struct {
	PodDBModel
	Hash   string `gorm:"uniqueIndex"`
	FeedId uint

	ItemData `gorm:"embedded"`
	XmlData  ItemXmlDBEntry `gorm:"foreignKey:ItemId"`
}

type ItemData struct {
	Filename     string
	Url          string
	Downloaded   bool
	CDFilename   string // content-disposition filename
	PubTimeStamp time.Time
	Archived     bool
}

type ItemXmlDBEntry struct {
	PodDBModel
	ItemId             uint
	podutils.XItemData `gorm:"embedded"`
}

// --------------------------------------------------------------------------
func (i Item) Format(fs fmt.State, c rune) {
	i.ItemDBEntry.Format(fs, c)
}

// --------------------------------------------------------------------------
func (idb ItemDBEntry) Format(fs fmt.State, c rune) {
	str := fmt.Sprintf("id: %v, Hash:'%v' Filename:'%v' Downloaded:%v PubTimeStamp:'%v'",
		idb.ID, idb.Hash, idb.Filename, idb.Downloaded, idb.PubTimeStamp)
	fs.Write([]byte(str))
}

// --------------------------------------------------------------------------
func createNewItemEntry(feedcfg podconfig.FeedToml, hash string, xml *podutils.XItemData) (*Item, error) {
	// new entry, xml coming from feed directly

	var (
		err error
	)

	if xml == nil {
		return nil, errors.New("xml data cannot be nil")
	}

	item := Item{}

	item.parentShortname = feedcfg.Shortname
	item.Hash = hash
	item.XmlData.XItemData = *xml
	item.PubTimeStamp = xml.Pubdate

	// verify hash
	if cHash, err := calcHash(item.XmlData.Guid, item.XmlData.Enclosure.Url, feedcfg.UrlParse); err != nil {
		return nil, fmt.Errorf("error in calculating hash: %w", err)
	} else if cHash != item.Hash {
		return nil, fmt.Errorf("newly calculated hash don't match; paramHash:'%v' calcHash:'%v'", item.Hash, cHash)
	}

	// parse url
	if item.Url, err = parseUrl(item.XmlData.Enclosure.Url, feedcfg.UrlParse); err != nil {
		log.Error("failed parsing url: ", err)
		return nil, err
	} else if item.Filename, err = item.generateFilename(feedcfg); err != nil {
		log.Error("failed to generate filename:", err)
		// to make sure we can continue, shortname.uuid.mp3
		item.Filename = feedcfg.Shortname + "." + strings.ReplaceAll(uuid.NewString(), "-", "") + ".mp3"
	} else {
		log.Debug("using generated filename: ", item.Filename)
	}

	item.log = log.WithField("item", item.Filename)

	// everything should be set
	return &item, nil
}

// --------------------------------------------------------------------------
func loadFromDBEntry(parentCfg podconfig.FeedToml, entry *ItemDBEntry) (*Item, error) {

	if entry == nil {
		return nil, errors.New("item dbentry is nil")
	} else if entry.ID == 0 {
		return nil, errors.New("item dbentry ID is zero")
	} else if entry.FeedId == 0 {
		// not sure if we need to check this.. ideally, we should
		return nil, errors.New("item dbentry feedId is zero")
	}

	// generated filename, parsedurl, etc loaded from db entry
	var (
		item = Item{
			ItemDBEntry: *entry,
			itemInternal: itemInternal{
				parentShortname: parentCfg.Shortname,
			},
		}
	)

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
// todo: future
/*
func (i *Item) LoadItemXml() error {
	if i.db == nil {
		return errors.New("db is nil")
	}

	var itemxml = podutils.XItemData{}
	var entry = ItemXmlDBEntry{Hash: i.Hash, ItemXml: itemxml}

	id, err := i.db.ItemXmlCollection().FetchByEntry(entry)
	if err != nil {
		i.log.Error("error fetching item xml: ", err)
		return err
	}
	i.dbXmlId = id
	i.xmlData = &entry.ItemXml

	return nil

}
*/

// --------------------------------------------------------------------------
func parseUrl(urlstr, urlparse string) (string, error) {

	u, err := url.ParseRequestURI(urlstr)
	if err != nil {
		return "", fmt.Errorf("failed url parsing: %w", err)
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

	return u.String(), nil
}

// --------------------------------------------------------------------------
func calcHash(guid, url, urlparse string) (string, error) {

	parsedUrl, err := parseUrl(url, urlparse)
	if err != nil {
		newerr := fmt.Errorf("failed to calc hash: %w", err)
		log.Errorf("%v", newerr)
		log.Errorf("check item with guid '%v' in xml", guid)
		return "", newerr
		// removing below.. if parse succeeded, url will never b empty
		// and since a url is always required, this really never will be called
		// } else if parsedUrl == "" && guid == "" {
		// 	return "", errors.New("failed to hash item; both url and guid are empty")
	}

	hash := podutils.GenerateHash(guid + parsedUrl)

	return hash, nil
}

// --------------------------------------------------------------------------
func (i *Item) updateXmlData(hash string, data *podutils.XItemData) {

	if i.Hash != hash {
		i.log.Warn("Hashes do not match; something is wrong")
		i.log.Warnf("itemhash:'%v' newhash:'%v'", i.Hash, hash)
	}

	// for now, just set the xml data
	// todo : deep compare??
	i.XmlData.XItemData = *data

	// if the url changes, this would be a new hash..
	// if the guid changes it would be a new hash..
	// no need to urlparse or regenerate filename
}

// --------------------------------------------------------------------------
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

// --------------------------------------------------------------------------
func (i *Item) Download(mp3path string) error {

	var (
		downloadTimestamp = time.Now()
		destfile          = filepath.Join(mp3path, i.Filename)
	)

	file, err := podutils.CreateTemp(filepath.Dir(destfile), filepath.Base(destfile)+"_temp*")
	if err != nil {
		i.log.Error("Failed creating temp file: ", err)
		return err
	}
	defer file.Close()

	if bw, cd, err := podutils.DownloadBuffered(i.Url, file, i.createProgressBar()); err != nil {
		i.log.Error("Failed downloading pod:", err)
		return err
	} else {
		i.log.Debugf("file written {%v} bytes: %.2fKB", filepath.Base(file.Name()), float64(bw)/(1<<10))

		if strings.Contains(cd, "filename") {
			// content disposition header, for the hell of it
			if r, err := regexp.Compile("filename=\"(.*)\""); err == nil {
				if matches := r.FindStringSubmatch(cd); len(matches) == 2 {
					i.CDFilename = matches[1]
				}
			} else {
				i.log.Warn("parsing content disposition had regex error: ", err)
			}
		}
	}
	// explicit close before rename
	file.Close()

	// move tempfile to finished file
	if err = podutils.Rename(file.Name(), destfile); err != nil {
		i.log.Debug("error moving temp file: ", err)
		return err
	}

	if err := podutils.Chtimes(destfile, downloadTimestamp, i.PubTimeStamp); err != nil {
		i.log.Error("failed to change modified time: ", err)
		// don't skip due to timestamp issue
	}

	i.Downloaded = true
	return nil
}
