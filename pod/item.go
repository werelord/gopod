package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// exported fields for database in feed list
type ItemData struct {
	// golang GC apparently doesn't have problems with circular references
	parent *Feed

	FeedItemEntry

	xmlData *podutils.XItemData
}

// stuff exported with the feed database (item list)
type FeedItemEntry struct {
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

// --------------------------------------------------------------------------
func (i ItemData) Format(fs fmt.State, c rune) {
	str := fmt.Sprintf("Hash:'%v' Filename:'%v' Downloaded:%v PubTimeStamp:'%v'",
		i.Hash, i.Filename, i.Downloaded, i.PubTimeStamp)
	fs.Write([]byte(str))
}

func CreateNewItemEntry(f *Feed, hash string, xmlData *podutils.XItemData) (*ItemData, error) {
	// new entry, xml coming from feed directly
	// if this is loaded from the database, ItemExport should be nil

	item := ItemData{
		parent: f,
		FeedItemEntry: FeedItemEntry{
			Hash:         hash,
			PubTimeStamp: xmlData.Pubdate,
			// rest generated below
		},
		xmlData: xmlData,
	}

	// parse url
	if err := item.parseUrl(); err != nil {
		log.Error("failed parsing url: ", err)
		return nil, err
	} else if err := item.generateFilename(); err != nil {
		log.Error("failed to generate filename:", err)
		// to make sure we can continue, shortname.uuid.mp3
		item.Filename = f.Shortname + "." + strings.ReplaceAll(uuid.NewString(), "-", "") + ".mp3"
	}

	// everything should be set

	return &item, nil
}

// --------------------------------------------------------------------------
func (i *ItemData) parseUrl() (err error) {

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
	if i.parent.UrlParse != "" {
		// assuming host is direct domain..
		trim := strings.SplitAfterN(u.Path, i.parent.UrlParse, 2)
		if len(trim) == 2 {
			u.Host = i.parent.UrlParse
			u.Path = trim[1]
		} else {
			log.Warn("failed parsing url; split failed")
			log.Warnf("url: '%v' UrlParse: '%v'", u.String(), i.parent.UrlParse)
		}
	}

	i.Url = u.String()

	return nil
}

// --------------------------------------------------------------------------
func (i *ItemData) generateFilename() error {
	// check to see if we neeed to parse.. simple search/replace

	// verify that export data is not null
	if i.xmlData == nil {
		return errors.New("item xml data is nil")
	}

	var (
		feed    = i.parent
		xmldata = i.xmlData
	)

	if feed.FilenameParse != "" {
		newstr := feed.FilenameParse

		if strings.Contains(feed.FilenameParse, "#shortname#") {
			newstr = strings.Replace(newstr, "#shortname#", feed.Shortname, 1)
		}
		if strings.Contains(feed.FilenameParse, "#linkfinalpath#") {
			// get the final path portion from the link url

			if u, err := url.Parse(xmldata.Link); err == nil {
				finalLink := path.Base(u.Path)
				newstr = strings.Replace(newstr, "#linkfinalpath#", podutils.CleanFilename(finalLink), 1)
			} else {
				log.Error("failed to parse link path.. not replacing:", err)
				return err
			}
		}

		if strings.Contains(feed.FilenameParse, "#episode#") {
			var padLen = 3
			if feed.EpisodePad > 0 {
				padLen = feed.EpisodePad
			}
			rep := xmldata.EpisodeStr
			//------------------------------------- DEBUG -------------------------------------
			if config.Debug && feed.Shortname == "russo" {
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
				if feed.Shortname == "dfo" {
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

		if strings.Contains(feed.FilenameParse, "#date#") {
			// date format YYYYMMDD
			newstr = strings.Replace(newstr, "#date#", xmldata.Pubdate.Format("20060102"), 1)
		}

		if strings.Contains(feed.FilenameParse, "#titleregex:") {
			if parsed, err := i.titleSubmatchRegex(feed.Regex, newstr, xmldata.Title); err != nil {
				log.Error("failed parsing title:", err)
				return err
			} else {
				newstr = podutils.CleanFilename(strings.ReplaceAll(parsed, " ", "_"))
			}
		}

		if strings.Contains(feed.FilenameParse, "#urlfilename#") {
			// for now, only applies to urlfilename
			var urlfilename = path.Base(i.Url)
			if feed.SkipFileTrim == false {
				urlfilename = podutils.CleanFilename(urlfilename)
			}
			newstr = strings.Replace(newstr, "#urlfilename#", urlfilename, 1)
		}

		i.Filename = newstr
		log.Debug("using generated filename: ", i.Filename)

		return nil
	}

	// fallthru to default
	i.Filename = podutils.CleanFilename(path.Base(i.Url))
	log.Debug("using default filename (no parsing): ", i.Filename)
	return nil
}

// --------------------------------------------------------------------------
func (i *ItemData) saveItemXml() (err error) {
	log.Infof("saving xmldata for %v{%v}, (%v)", i.Filename, i.Hash, i.parent.Shortname)

	if i.xmlData == nil {
		return errors.New("xml data is nil, cannot save")
	}

	if config.Simulate {
		log.Debug("skipping saving item database due to sim flag")
		return nil
	}

	// make sure db is init
	i.parent.initDB()

	jsonFile := strings.TrimSuffix(i.Filename, filepath.Ext(i.Filename))

	var export ItemExport
	export.Hash = i.Hash
	export.ItemXmlData = *i.xmlData

	if e := i.parent.db.Write("items", jsonFile, export); e != nil {
		log.Error("failed to write database file: ", e)
		return e
	}

	return nil
}

// todo: add loading item xml data, when needed
