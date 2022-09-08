package pod

import (
	"errors"
	"fmt"
	"gopod/podconfig"
	"gopod/podutils"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

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
					rep = xmldata.Pubdate.Format(podutils.TimeFormatStr)
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
			newstr = strings.Replace(newstr, "#date#", xmldata.Pubdate.Format(podutils.TimeFormatStr), 1)
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
func (i Item) titleSubmatchRegex(regex, dststr, title string) (string, error) {

	var (
		r   *regexp.Regexp
		err error
	)

	if r, err = regexp.Compile(regex); err != nil {
		return "", err
	}

	matchSlice := r.FindStringSubmatch(title)

	if len(matchSlice) < r.NumSubexp() {
		log.Warn("regex doesn't match; replacing with blank strings")
		// return "", errors.New("return slice of regex doesn't match slices needed")
	}

	// parse the dst for the format "#titleregex:X#", figure out which submatches are needed
	// skipping index 0, as that is the original string
	for i := 1; i <= r.NumSubexp(); i++ {
		str := fmt.Sprintf("#titleregex:%v#", i)

		if strings.Contains(dststr, str) {
			var replaceStr string
			if i > len(matchSlice) {
				replaceStr = ""
			} else {
				replaceStr = strings.TrimSpace(matchSlice[i])
			}
			dststr = strings.Replace(dststr, str, replaceStr, 1)
			//log.Debug(i, ": ", dststr)
		}
	}

	// in case all are blank.. remove any ".." found

	return strings.ReplaceAll(dststr, "..", "."), nil

}
