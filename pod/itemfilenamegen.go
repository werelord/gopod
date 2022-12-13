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
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

var cleanFilename = podutils.CleanFilename
var timeNow = time.Now

// --------------------------------------------------------------------------
func (i *Item) generateFilename(cfg podconfig.FeedToml, collFunc func(string) bool) (string, string, error) {
	// check to see if we neeed to parse.. simple search/replace

	// verify that export data is not null

	// todo: need to check for filename collisions!! fucking shit

	if i.XmlData == nil {
		return "", "", errors.New("unable to generate filename; item xml is nil")
	}

	var (
		filename       = cfg.FilenameParse
		extra          string
		defReplacement string
		err            error
	)

	if cfg.FilenameParse == "" {
		// fallthru to default
		filename = path.Base(i.Url)
		//log.Debug("using default filename (no parsing): ", filename)

	} else {

		if i.XmlData.Pubdate.IsZero() {
			log.Warn("Pubdate not set; default replacement set to Now()")
			defReplacement = timeNow().Format(podutils.TimeFormatStr)
		} else {
			defReplacement = i.XmlData.Pubdate.Format(podutils.TimeFormatStr)
		}

		filename = strings.Replace(filename, "#shortname#", cfg.Shortname, 1)
		filename = i.replaceLinkFinalPath(filename, defReplacement)
		filename = i.replaceEpisode(filename, defReplacement, cfg)
		filename = i.replaceCount(filename, defReplacement, cfg)
		filename = strings.Replace(filename, "#season#", i.XmlData.SeasonStr, 1)
		filename = strings.Replace(filename, "#date#", defReplacement, 1)
		filename = strings.Replace(filename, "#title#", i.XmlData.Title, 1)
		if filename, err = i.replaceTitleRegex(filename, cfg.Regex); err != nil {
			log.Error("failed parsing title:", err)
			return "", "", err
		}
		filename = strings.Replace(filename, "#urlfilename#", path.Base(i.Url), 1)
	}

	// make sure we have a clean filename..
	var rep = "_" // default replacement
	if cfg.CleanRep != nil {
		rep = *cfg.CleanRep
	}
	filename = cleanFilename(filename, rep)

	if i.FilenameXta != "" {
		// filename collisions have already been handled, and the extra already exists..
		// insert the extra into the filename
		ext := filepath.Ext(filename)
		base := filename[:len(filename)-len(ext)]
		filename = fmt.Sprintf("%s%s%s", base, i.FilenameXta, ext)
		// return extra in case its needed
		extra = i.FilenameXta
	} else if filename, extra, err = i.checkFilenameCollisions(filename, collFunc); err != nil {
		log.Error("error in checking filename collision: ", err)
		return "", "", err
	}

	return filename, extra, nil
}

// --------------------------------------------------------------------------
func (i Item) replaceLinkFinalPath(str, failureStr string) string {
	if strings.Contains(str, "#linkfinalpath#") {
		// get the final path portion from the link url
		if i.XmlData.Link == "" {
			log.Warn("item link is empty; replacing with failure option: ", failureStr)
			str = strings.Replace(str, "#linkfinalpath#", failureStr, 1)
		} else if u, err := url.Parse(i.XmlData.Link); err == nil {
			finalLink := path.Base(u.Path)
			str = strings.Replace(str, "#linkfinalpath#", finalLink, 1)
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
		epStr := i.XmlData.EpisodeStr

		if epStr == "" {
			epStr = defaultRep
		} else {
			// pad string with zeros minus length
			epStr = fmt.Sprintf("%0*s", padLen, epStr)
		}
		str = strings.Replace(str, "#episode#", epStr, 1)
	}
	return str
}

// --------------------------------------------------------------------------
func (i Item) replaceCount(str, defaultRep string, cfg podconfig.FeedToml) string {
	if strings.Contains(str, "#count#") {
		// default len of 3 unless otherwise defined

		var (
			countStr string
			padLen   = podutils.Tern(cfg.EpisodePad > 0, cfg.EpisodePad, 3)
		)

		if i.EpNum < 0 { // because episode #s may be zero indexed..
			countStr = defaultRep
		} else {
			countStr = fmt.Sprintf("%0*d", padLen, i.EpNum)
		}

		str = strings.Replace(str, "#count#", countStr, 1)
	}
	return str
}

// --------------------------------------------------------------------------
func (i Item) replaceTitleRegex(dststr, regex string) (string, error) {

	if strings.Contains(dststr, "#titleregex:") == false {
		return dststr, nil
	}

	var (
		r   *regexp.Regexp
		err error
	)
	if regex == "" {
		return "", errors.New("regex is empty")
	} else if r, err = regexp.Compile(regex); err != nil {
		return "", err
	}

	matchSlice := r.FindStringSubmatch(i.XmlData.Title)

	if len(matchSlice) < r.NumSubexp() {
		log.WithFields(log.Fields{
			"xmlTitle": i.XmlData.Title,
			"regex":    regex,
		}).Warn("regex doesn't match; replacing with blank strings")
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
func (i Item) checkFilenameCollisions(fname string, collFunc func(string) bool) (string, string, error) {

	var (
		filename = fname
		extra    string
	)

	if collFunc != nil && collFunc(filename) {
		// check for collisions
		log.Infof("filename collision found on %q; trying alternatives", filename)
		// filename collision detected; start iterating thru alternates (saving alternate string
		// for future generation checks) until one passes
		// if we go beyond this, we're in trouble
		for _, r := range "ABCDEFGHIJKL" {
			ext := filepath.Ext(filename)
			base := filename[:len(filename)-len(ext)]
			newfilename := fmt.Sprintf("%s%s%s", base, string(r), ext)
			if collFunc(newfilename) == false {
				filename = newfilename
				extra = string(r)
				break
			}
		}

		// do one more check for sanity
		if collFunc(filename) {
			err := fmt.Errorf("filename '%s' still collides; something is seriously wrong", filename)
			log.Error(err)
			return "", "", err
		}
	}

	return filename, extra, nil
}
