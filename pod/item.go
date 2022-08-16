package pod

import (
	"errors"
	"fmt"
	"gopod/podconfig"
	"gopod/poddb"
	"gopod/podutils"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
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
	db              *poddb.PodDB

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
func CreateNewItemEntry(parentConfig podconfig.FeedToml,
	db *poddb.PodDB,
	hash string,
	xmlData *podutils.XItemData) (*Item, error) {
	// new entry, xml coming from feed directly
	// if this is loaded from the database, ItemExport should be nil

	item := Item{
		Hash:    hash,
		xmlData: xmlData,
		// rest generated below
	}

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

// --------------------------------------------------------------------------
func (i *Item) generateFilename(cfg podconfig.FeedToml) error {
	// check to see if we neeed to parse.. simple search/replace

	// verify that export data is not null

	// todo: need to check for filename collisions!! fucking shit

	log.Trace("here")

	if i.xmlData == nil {
		return errors.New("item xml data is nil")
	}

	var (
		xmldata = i.xmlData
	)

	if cfg.FilenameParse != "" {
		newstr := cfg.FilenameParse

		if strings.Contains(newstr, "#shortname#") {
			newstr = strings.Replace(newstr, "#shortname#", cfg.Shortname, 1)
		}
		if strings.Contains(newstr, "#linkfinalpath#") {
			// get the final path portion from the link url

			if u, err := url.Parse(xmldata.Link); err == nil {
				finalLink := path.Base(u.Path)
				newstr = strings.Replace(newstr, "#linkfinalpath#", podutils.CleanFilename(finalLink), 1)
			} else {
				log.Error("failed to parse link path.. not replacing:", err)
				return err
			}
		}

		// todo: deprecate this; instead of using #episode#, maintain an episode count within the db
		if strings.Contains(newstr, "#episode#") {
			var padLen = 3
			if cfg.EpisodePad > 0 {
				padLen = cfg.EpisodePad
			}
			rep := xmldata.EpisodeStr
			//------------------------------------- DEBUG -------------------------------------
			if config.Debug && cfg.Shortname == "russo" {
				// grab the episode from the title, as the numbers don't match for these
				r, _ := regexp.Compile("The Russo-Souhan Show ([0-9]*) - ")
				eps := r.FindStringSubmatch(xmldata.Title)
				if len(eps) == 2 {
					rep = eps[1]
				}
			} else if config.Debug && cfg.Shortname == "dfo" {
				var catch = []string{"61", "60"}
				if slices.Contains(catch, xmldata.EpisodeStr) {
					r, _ := regexp.Compile("E[p|P]. ([0-9]*):.*")
					eps := r.FindStringSubmatch(xmldata.Title)
					if len(eps) == 2 {
						rep = eps[1]
						log.Warnf("mismatch, epstr:%v, using:%v, title:%v", xmldata.EpisodeStr, rep, xmldata.Title)
					} else {
						log.Warn("WTF")
					}
				}

			} else if config.Debug && cfg.Shortname == "jjgo" {
				skipList := []string{"Maximum Fun Drive: May 15th-31st",
					"Shootin' the Bries - Episode 2",
					"Ashkon - Soldier Boy",
					"T-Shirt Contest!",
					"Bubble by Jordan Morris",
					"JJGo Bonus: Stop Podcasting Yourself",
					"The MaxFunStore is Open!", "Ashkon - Hey Keezy",
					"MaxFunDrive 2010: May 13-28", "International Waters Ep. 3: Exploding Draculas"}
				if xmldata.EpisodeStr == "194" {
					rep = "184"
					log.Warnf("manual override, ep:%v, used:%v, tit:%v", xmldata.EpisodeStr, rep, xmldata.Title)
				} else if xmldata.EpisodeStr == "0" {
					rep = "1"
					log.Warnf("manual override, ep:%v, used:%v, tit:%v", xmldata.EpisodeStr, rep, xmldata.Title)
				} else if slices.Contains(skipList, xmldata.Title) == false {
					// grab the episode from the title, as the numbers don't match for these
					intVar, err := strconv.Atoi(rep)
					if err != nil {
						log.Panic(err)
					}
					if intVar <= 636 {
						// try using the value from title
						r, _ := regexp.Compile("E[p|P]. ?([0-9]*[A|B]?) ?[:|-].*")
						eps := r.FindStringSubmatch(xmldata.Title)
						if len(eps) == 2 {
							rep = eps[1]
							//log.Warnf("found mismatch, ep:%v, used:%v, tit:%v", xmldata.EpisodeStr, rep, xmldata.Title)
						} else {
							log.Warnf("Unable to find episode string!! (%v)", xmldata.Title)
						}

					}
				} else {
					// skps should use blank
					log.Warnf("skipping, setting to blank (%v)", xmldata.Title)
					rep = ""

				}

			}
			//------------------------------------- DEBUG -------------------------------------

			if rep == "" {
				//------------------------------------- DEBUG -------------------------------------
				// hack.. don't like this specific, but fuck it
				if cfg.Shortname == "dfo" {
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
					rep = xmldata.Pubdate.Format("20060102_150405")
					//rep = strings.Repeat("X", padLen)
				}

			} else if len(rep) < padLen {
				// pad string with zeros minus length
				rep = strings.Repeat("0", padLen-len(rep)) + rep
			}
			newstr = strings.Replace(newstr, "#episode#", podutils.CleanFilename(rep), 1)
		}

		if strings.Contains(newstr, "#date#") {
			// date format YYYYMMDD
			newstr = strings.Replace(newstr, "#date#", xmldata.Pubdate.Format("20060102_150405"), 1)
		}

		if strings.Contains(newstr, "#titleregex:") {
			if parsed, err := i.titleSubmatchRegex(cfg.Regex, newstr, xmldata.Title); err != nil {
				log.Error("failed parsing title:", err)
				return err
			} else {
				newstr = podutils.CleanFilename(strings.ReplaceAll(parsed, " ", "_"))
			}
		}

		if strings.Contains(newstr, "#urlfilename#") {
			// for now, only applies to urlfilename
			var urlfilename = path.Base(i.Url)
			if cfg.SkipFileTrim == false {
				urlfilename = podutils.CleanFilename(urlfilename)
			}
			newstr = strings.Replace(newstr, "#urlfilename#", urlfilename, 1)
		}

		// hack bullshit
		//------------------------------------- DEBUG -------------------------------------
		if newstr == "russo.ep262.20200923.mp3" && xmldata.Guid == "433ca553-d9a8-43de-857c-b377098c4971" {

			newstr = newstr[:len(newstr)-len(filepath.Ext(newstr))] + "_2" + filepath.Ext(newstr)
		}
		//------------------------------------- DEBUG -------------------------------------

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
func (i Item) saveItemData() error {
	// todo: this
	return errors.New("not implemented")
}

// --------------------------------------------------------------------------
func (i Item) saveItemXml() (err error) {
	log.Infof("saving xmldata for %v{%v}, (%v)", i.Filename, i.Hash, i.parentShortname)

	if i.xmlData == nil {
		return errors.New("xml data is nil, cannot save")
	}

	if config.Simulate {
		log.Debug("skipping saving item database due to sim flag")
		return nil
	}

	return errors.New("not implemented")

	// todo: this
	/*
		// make sure db is init
		i.parent.initDB()

		jsonFile := strings.TrimSuffix(i.Filename, filepath.Ext(i.Filename))

		var export ItemExportScribble
		export.Hash = i.Hash
		export.ItemXmlData = *i.xmlData

		if e := i.parent.db.Write("items", jsonFile, export); e != nil {
			log.Error("failed to write database file: ", e)
			return e
		}
		return nil
		*/
}
