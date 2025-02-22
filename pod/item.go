package pod

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	log "gopod/multilogger"
	"gopod/podconfig"
	"gopod/podutils"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"
)

type Item struct {
	// all db entities, exported
	ItemDBEntry

	// all internal, non-exported (no db saving) items
	itemInternal
}

type itemInternal struct {
	parentShortname string // for logging purposes

	log log.Logger
}

type ItemDBEntry struct {
	PodDBModel
	Hash   string `gorm:"uniqueIndex"`
	FeedId uint

	ItemData `gorm:"embedded"`
	XmlId    uint
	XmlData  *ItemXmlDBEntry `gorm:"foreignKey:XmlId"`
	ImageKey string
}

type ItemData struct {
	Filename     string
	FilenameXta  string
	Url          string
	Guid         string
	Downloaded   bool
	CDFilename   string // content-disposition filename
	PubTimeStamp time.Time
	Archived     bool
	EpNum        int
}

type ItemXmlDBEntry struct {
	PodDBModel
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
func createNewItemEntry(
	feedcfg podconfig.FeedToml, hash string, xml *podutils.XItemData,
	epNum int, collFunc func(string) bool) (*Item, error) {
	// new entry, xml coming from feed directly

	if xml == nil {
		return nil, errors.New("xml data cannot be nil")
	}

	item := Item{}

	item.parentShortname = feedcfg.Shortname
	item.Guid = xml.Guid
	item.XmlData = &ItemXmlDBEntry{}
	item.XmlData.XItemData = *xml
	item.PubTimeStamp = xml.Pubdate
	item.EpNum = epNum

	// verify hash
	if cHash, err := calcHash(item.XmlData.Guid, item.XmlData.Enclosure.Url, feedcfg.UrlParse); err != nil {
		return nil, fmt.Errorf("error in calculating hash: %w", err)
	} else if cHash != hash {
		return nil, fmt.Errorf("newly calculated hash don't match; paramHash:'%v' calcHash:'%v'", hash, cHash)
	} else {
		item.Hash = cHash
	}

	// parse url
	if cUrl, err := parseUrl(item.XmlData.Enclosure.Url, feedcfg.UrlParse, feedcfg.RetainQueryStr); err != nil {
		log.Errorf("failed parsing url: %v", err)
		return nil, err
	} else {
		item.Url = cUrl
	}

	// generate filename
	if filename, extra, err := item.generateFilename(feedcfg, collFunc); err != nil {
		log.Errorf("failed to generate filename: %v", err)
		// to make sure we can continue, shortname.uuid.mp3
		item.Filename = feedcfg.Shortname + "." + strings.ReplaceAll(uuid.NewString(), "-", "") + ".mp3"
	} else {
		// at this point, we should have a valid filename
		item.Filename = filename
		item.FilenameXta = extra
		log.Debugf("using generated filename: %v (%v)", item.Filename, item.XmlData.Title)
	}

	item.log = log.With("item", item.Filename)

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
				log:             log.With("item", entry.Filename),
			},
		}
	)

	// sanity check
	if item.Hash == "" {
		return nil, errors.New("failed loading item; hash is empty")
	} else if item.Filename == "" {
		return nil, errors.New("failed loading item; filename is empty (db corrupt?)")
		// } else if item.Guid == "" {
		// 	return nil, errors.New("failed loading item; guid is empty")
	}

	//log.Debugf("Item Loaded: %v", item)

	return &item, nil
}

// --------------------------------------------------------------------------
func (i Item) isSameXmlEntry(new *podutils.XItemData, cfg podconfig.FeedToml) (bool, error) {

	if (i.XmlData == nil) || (i.XmlData.ID <= 0) {
		return false, errors.New("cannot compare to current; xml is not loaded from db")
	}

	var (
		oldUrl, newUrl   *url.URL
		oldbase, newbase string
		err              error
	)

	if oldUrl, err = url.Parse(i.XmlData.Enclosure.Url); err != nil {
		return false, err
	} else if newUrl, err = url.Parse(new.Enclosure.Url); err != nil {
		return false, err
	} else {
		oldbase = path.Base(oldUrl.Path)
		newbase = path.Base(newUrl.Path)
	}

	i.log.With(
		"old length", i.XmlData.Enclosure.Length,
		"new length", new.Enclosure.Length,
		"old url", i.XmlData.Enclosure.Url,
		"new url", new.Enclosure.Url,
		"old base", oldbase,
		"new base", newbase,
	).Debug("item diffs")

	// simple comparison here.. just check case insensitive filename and length
	// not so fucking simple.. simplecast fucks with filenames all the time; if url contains the string
	// in dupFilenameBypass ignore the base filename and go only on enclosure length

	// HACK: getting sick of this shit
	if (i.XmlData.Enclosure.Length == 0) || (new.Enclosure.Length == 0) {
		log.Warn("zero enclosure length found; just assuming its the same entry")
		return true, nil
	} else if i.XmlData.Enclosure.Length == new.Enclosure.Length {
		var logtemp = i.log.With(
			"old url", oldUrl.String(),
			"new url", newUrl.String(),
			"dupFilenameBypass", cfg.DupFilenameBypass,
		)

		if (cfg.DupFilenameBypass != "") && strings.Contains(new.Enclosure.Url, cfg.DupFilenameBypass) {
			logtemp.Debug("same lengths and dupFilenameBypass found; marking as identical")
			return true, nil

			// now do simple comparison
		} else if strings.EqualFold(oldbase, newbase) {
			logtemp.Warn("new url, same item; new redirect?")
			return true, nil

		} else {
			logtemp.Debug("valid modified entry found")
			return false, nil
		}

	} else {
		i.log.Debug("no match due to different lengths; valid modified entry found")
		return false, nil
	}

}

// --------------------------------------------------------------------------
func (i *Item) updateFromEntry(
	feedcfg podconfig.FeedToml, hash string, xml *podutils.XItemData,
	collFunc func(string) bool) error {
	// almost the same as createNew, with changes:

	// sanity checks
	if xml == nil {
		return errors.New("xml data cannot be nil")
	} else if xml.Guid != i.Guid {
		return errors.New("guid from xml doesn't match items; this should not happen")
	} else if i.XmlData == nil {
		return errors.New("xml is nil; make sure previous xml is loaded before updating")
	} else if i.XmlData.ID == 0 {
		return errors.New("xml ID is 0; make sure previous xml is loaded before updating")
	} else if i.parentShortname != feedcfg.Shortname {
		// parent shortname should not change
		return errors.New("feed shortname does not match this item's parent shortname")
	}

	// before changing the xml data, see if this actuaelly changed
	sameEntry, err := i.isSameXmlEntry(xml, feedcfg)
	if err != nil {
		return err
	}

	// todo: rather than change already existing xml entry, create a new one.. will require db changes
	// moving relationship to one to many, requiring ItemXmlDBEntry to retain reference to ItemDBEntry
	// and be potentially "orphaned"

	i.XmlData.XItemData = *xml
	i.PubTimeStamp = xml.Pubdate
	// episode count should not change

	// verify hash; hash will change (old guid, new url)
	if cHash, err := calcHash(xml.Guid, xml.Enclosure.Url, feedcfg.UrlParse); err != nil {
		return fmt.Errorf("error in calculating hash: %w", err)
	} else if cHash != hash {
		return fmt.Errorf("newly calculated hash don't match; paramHash:'%v' calcHash:'%v'", hash, cHash)
	} else {
		i.Hash = cHash
	}

	// parse url
	if cUrl, err := parseUrl(xml.Enclosure.Url, feedcfg.UrlParse, feedcfg.RetainQueryStr); err != nil {
		log.Errorf("failed parsing url: %v", err)
		return err
	} else {
		log.With(
			"oldUrl", i.Url,
			"newUrl", cUrl,
			"shortname", i.parentShortname,
		).Debug("new urls from hash")

		if i.Url == cUrl {
			return fmt.Errorf("urls are the same; this shouldn't happen")
		}
		i.Url = cUrl
	}

	if sameEntry == false {
		// filename collision detection should rename the filename; set downloaded to false
		i.Downloaded = false

		// generate filename
		if filename, extra, err := i.generateFilename(feedcfg, collFunc); err != nil {
			log.Errorf("failed to generate filename: %v", err)
			// to make sure we can continue, shortname.uuid.mp3
			i.Filename = feedcfg.Shortname + "." + strings.ReplaceAll(uuid.NewString(), "-", "") + ".mp3"
		} else {
			// at this point, we should have a valid filename
			i.Filename = filename
			i.FilenameXta = extra
			log.Debug("using generated filename: ", i.Filename)
		}
	} else {
		log.Debug("same item detected, not resetting downloaded or updating filename")
	}

	i.log = log.With("item", i.Filename)

	// everything should be set
	return nil
}

// --------------------------------------------------------------------------
func (i *Item) loadItemXml(db *PodDB) error {
	if (i.XmlData != nil) && (i.XmlData.ID > 0) {
		// already loaded
		return nil
	} else if db == nil {
		return errors.New("db is nil")
	}

	if xmlEntry, err := db.loadItemXml(i.XmlId); err != nil {
		return err
	} else {
		i.XmlData = xmlEntry
	}

	return nil
}

// --------------------------------------------------------------------------
func parseUrl(urlstr, urlparse string, disableStrip bool) (string, error) {

	u, err := url.ParseRequestURI(urlstr)
	if err != nil {
		return "", fmt.Errorf("failed url parsing: %w", err)
	}

	// remove querystring/fragment
	if disableStrip == false {
		u.RawQuery = ""
		u.Fragment = ""
	}

	// handle url parsing, if needed
	if urlparse != "" {
		// urlparse may be comma delimited
		parseList := strings.Split(urlparse, ",")

		var found = false
		for _, parse := range parseList {
			// assuming host is direct domain..
			trim := strings.SplitAfterN(u.Path, parse, 2)
			if len(trim) == 2 {
				found = true

				// url parsing is either <domain>/<path>, or a http encoded url (with scheme)
				// make sure scheme is included in reparsing, to ensure domain/path are separated correctly
				var newUrlStr = parse + trim[1]
				if strings.Contains(newUrlStr, "http") == false {
					newUrlStr = u.Scheme + "://" + newUrlStr
				}
				if newUrl, err := url.Parse(newUrlStr); err != nil {
					return "", fmt.Errorf("failed url parsing: %w", err)
				} else {
					u.Scheme = newUrl.Scheme
					u.Host = newUrl.Host
					u.Path = newUrl.Path
				}

				break
			}
		}
		if found == false {
			// see if parseList contains current host; if it doesn't THEN log the warning
			if slices.Contains(parseList, u.Host) == false {
				log.With("url", u.String(), "UrlParse", urlparse).Warn("failed parsing url; split failed")
			}
		}

	}

	return u.String(), nil
}

// --------------------------------------------------------------------------
func calcHash(guid, url, urlparse string) (string, error) {
	// hash only on non-query parameters
	parsedUrl, err := parseUrl(url, urlparse, false)
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
func (i *Item) updateXmlData(hash string, data *podutils.XItemData) error {

	if i.Hash != hash {
		err := fmt.Errorf("hashes do not match; something is wrong; itemhash:'%v' newhash:'%v'", i.Hash, hash)
		i.log.Error(err)
		return err
	} else if i.XmlData == nil {
		err := fmt.Errorf("unable to update xml data; xml is nil")
		i.log.Error(err)
		return err
	} else if i.XmlData.ID == 0 {
		err := fmt.Errorf("unable to update xml data; xml data id is zero")
		i.log.Error(err)
		return err
	}

	i.XmlData.XItemData = *data

	// if the url changes, this would be a new hash..
	// if the guid changes it would be a new hash..
	// no need to urlparse or regenerate filename
	return nil
}

// --------------------------------------------------------------------------
func (i Item) createProgressBar() *progressbar.ProgressBar {
	bar := progressbar.NewOptions64(int64(i.XmlData.Enclosure.Length), // set the default length to amount in enclosure
		progressbar.OptionSetDescription(i.Filename),
		progressbar.OptionFullWidth(),
		progressbar.OptionShowBytes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() { fmt.Fprint(os.Stderr, "\n") }),

		progressbar.OptionSetTheme(progressbar.Theme{Saucer: "=", SaucerHead: ">", SaucerPadding: " ", BarStart: "[", BarEnd: "]"}))

	return bar

}

// --------------------------------------------------------------------------
func (i *Item) Download(mp3path string) (int64, error) {

	var (
		destfile   = filepath.Join(mp3path, i.Filename)
		bytesWrote int64
	)

	file, err := podutils.CreateTemp(filepath.Dir(destfile), filepath.Base(destfile)+"_temp*")
	if err != nil {
		i.log.Errorf("Failed creating temp file: %v", err)
		return bytesWrote, err
	}
	defer file.Close()

	var (
		cd          string
		pbar        = i.createProgressBar()
		multiWriter = io.MultiWriter(file, pbar)
	)

	// get content disposition, set the length of progress bar
	var onResp = func(resp *http.Response) {
		cd = resp.Header.Get("Content-Disposition")
		if resp.ContentLength > 0 {
			pbar.ChangeMax64(resp.ContentLength)
		}
	}

	if bw, err := podutils.DownloadBuffered(i.Url, multiWriter, onResp); err != nil {
		i.log.Errorf("Failed downloading pod: %v", err)
		return bw, err
	} else {
		i.log.Debugf("file written {%v} bytes: %v", filepath.Base(file.Name()), podutils.FormatBytes(uint64(bw)))
		bytesWrote = bw

		if strings.Contains(cd, "filename") {
			// content disposition header, for the hell of it
			if r, err := regexp.Compile("filename=\"(.*)\""); err == nil {
				if matches := r.FindStringSubmatch(cd); len(matches) == 2 {
					i.CDFilename = matches[1]
				}
			} else {
				i.log.Warnf("parsing content disposition had regex error: %v", err)
			}
		}
	}
	// explicit close before rename
	file.Close()

	// move tempfile to finished file
	if err = podutils.Rename(file.Name(), destfile); err != nil {
		i.log.Debug("error moving temp file: ", err)
		return bytesWrote, err
	}

	// set downloaded, and make sure timestamps match
	if err := podutils.Chtimes(destfile, time.Now(), i.PubTimeStamp); err != nil {
		i.log.Errorf("failed to change modified time: %v", err)
		// don't skip due to timestamp issue
	}
	i.Downloaded = true

	return bytesWrote, nil
}

// --------------------------------------------------------------------------
func (i Item) genImageFilename() string {
	var base = strings.TrimSuffix(i.Filename, filepath.Ext(i.Filename))
	return fmt.Sprintf("%v__%v%v", base, lastModStr, extStr)
}
