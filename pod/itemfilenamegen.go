package pod

import (
	"errors"
	"fmt"
	"gopod/podconfig"
	"gopod/podutils"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var cleanFilename = podutils.CleanFilename

// --------------------------------------------------------------------------
func (i *Item) generateFilename(cfg podconfig.FeedToml) error {
	// check to see if we neeed to parse.. simple search/replace

	// verify that export data is not null

	// todo: need to check for filename collisions!! fucking shit

	if i.xmlData == nil {
		return errors.New("item xml data is nil")
	}

	if cfg.FilenameParse == "" {
		// fallthru to default
		log.Debug("using default filename (no parsing): ", i.Filename)
		i.Filename = path.Base(i.Url)
		return nil
	}

	var (
		newstr             = cfg.FilenameParse
		defaultReplacement string
		err                error
	)

	if i.xmlData.Pubdate.IsZero() {
		log.Warn("Pubdate not set; default replacement set to Now()")
		defaultReplacement = time.Now().Format(podutils.TimeFormatStr)
	} else {
		defaultReplacement = i.xmlData.Pubdate.Format(podutils.TimeFormatStr)
	}

	newstr = strings.Replace(newstr, "#shortname#", cfg.Shortname, 1)
	newstr = i.replaceLinkFinalPath(newstr, defaultReplacement)
	newstr = i.replaceEpisode(newstr, defaultReplacement, cfg)
	newstr = strings.Replace(newstr, "#date#", defaultReplacement, 1)
	if newstr, err = i.replaceTitleRegex(newstr, cfg.Regex, i.xmlData.Title); err != nil {
		log.Error("failed parsing title:", err)
		return err
	}
	newstr = i.replaceUrlFilename(newstr, cfg)

	// complete
	i.Filename = newstr
	log.Debug("using generated filename: ", i.Filename)
	return nil
}

// --------------------------------------------------------------------------
func (i Item) replaceLinkFinalPath(str, failureStr string) string {
	if strings.Contains(str, "#linkfinalpath#") {
		// get the final path portion from the link url
		if i.xmlData.Link == "" {
			log.Warn("item link is empty; replacing with failure option: ", failureStr)
			str = strings.Replace(str, "#linkfinalpath#", failureStr, 1)
		} else if u, err := url.Parse(i.xmlData.Link); err == nil {
			finalLink := path.Base(u.Path)
			str = strings.Replace(str, "#linkfinalpath#", cleanFilename(finalLink), 1)
		} else {
			log.Error("failed to parse link path: ", err)
			log.Warn("Replacing with failure option: ", failureStr)
			str = strings.Replace(str, "#linkfinalpath#", failureStr, 1)
		}
	}
	return str
}

// --------------------------------------------------------------------------
func (i Item) replaceEpisode(str, defaultRep string, cfg podconfig.FeedToml) string {
	// todo: deprecate this; instead of using #episode#, maintain an episode count within the db
	if strings.Contains(str, "#episode#") {
		// default length of 3, unless otherwise defined
		var padLen = podutils.Tern(cfg.EpisodePad > 0, cfg.EpisodePad, 3)
		epStr := i.xmlData.EpisodeStr

		if epStr == "" {
			epStr = defaultRep
		} else if len(epStr) < padLen {
			// pad string with zeros minus length
			epStr = strings.Repeat("0", padLen-len(epStr)) + epStr
		}
		str = strings.Replace(str, "#episode#", epStr, 1)
	}
	return str
}

// --------------------------------------------------------------------------
func (i Item) replaceTitleRegex(dststr, regex, title string) (string, error) {

	if strings.Contains(dststr, "#titleregex:") == false {
		return dststr, nil
	}

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
	dststr = strings.ReplaceAll(dststr, "..", ".")
	// replace any spaces with underscores
	dststr = strings.ReplaceAll(dststr, " ", "_")
	return dststr, nil
}

// --------------------------------------------------------------------------
func (i Item) replaceUrlFilename(str string, cfg podconfig.FeedToml) string {
	if strings.Contains(str, "#urlfilename#") {
		// for now, only applies to urlfilename
		var urlfilename = path.Base(i.Url)
		if cfg.SkipFileTrim == false {
			urlfilename = cleanFilename(urlfilename)
		}
		str = strings.Replace(str, "#urlfilename#", urlfilename, 1)
	}
	return str
}
