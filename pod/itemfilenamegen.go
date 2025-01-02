package pod

import (
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	log "gopod/multilogger"
	"gopod/podconfig"
	"gopod/podutils"
)

var cleanFilename = podutils.CleanFilename
var timeNow = time.Now

// --------------------------------------------------------------------------
func (i *Item) generateFilename(cfg podconfig.FeedToml, collFunc func(string) bool) (string, string, error) {
	// check to see if we neeed to parse.. simple search/replace

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
		filename = i.replaceTitle(filename)
		if filename, err = i.replaceTitleRegex(filename, cfg.Regex); err != nil {
			log.Errorf("failed parsing title: %v", err)
			return "", "", err
		}
		filename = i.replaceExtension(filename)
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
		log.Errorf("error in checking filename collision: %v", err)
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
			log.Errorf("failed to parse link path: %v", err)
			log.Warnf("Replacing with failure option: %v", failureStr)
			str = strings.Replace(str, "#linkfinalpath#", failureStr, 1)
		}
	}
	return str
}

// --------------------------------------------------------------------------
func (i Item) replaceEpisode(str, defaultRep string, cfg podconfig.FeedToml) string {
	if strings.Contains(str, "#episode#") {
		// default length of 3, unless otherwise defined
		// future: check itunes:episodeType (full or otherwise) before using the episode string..
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
func (i Item) replaceTitle(str string) string {
	// adding this as separate method, to make sure spaces are replaced
	// todo: make a config value for controlling this

	if strings.Contains(str, "#title#") {
		str = strings.Replace(str, "#title#", i.XmlData.Title, 1)

		str = strings.ReplaceAll(str, " ", "_")
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
		log.With(
			"xmlTitle", i.XmlData.Title,
			"regex", regex,
		).Warn("regex doesn't match; replacing with full title")
		// return "", errors.New("return slice of regex doesn't match slices needed")
	}

	// parse the dst for the format "#titleregex:X#", figure out which submatches are needed
	// skipping index 0, as that is the original string
	for idx := 1; idx <= r.NumSubexp(); idx++ {
		str := fmt.Sprintf("#titleregex:%v#", idx)

		if strings.Contains(dststr, str) {
			var replaceStr string
			if idx > len(matchSlice) {
				if idx == 1 {
					// first iteration, and no matches found.. replace with full title
					replaceStr = i.XmlData.Title
				} else {
					// otherwise, empty string
					replaceStr = ""
				}
			} else {
				replaceStr = strings.TrimSpace(matchSlice[idx])
			}
			dststr = strings.Replace(dststr, str, replaceStr, 1)
			// log.Debug(i, ": ", dststr)
		}
	}

	// in case all are blank.. remove any ".." found
	dststr = strings.ReplaceAll(dststr, "..", ".")
	// replace any spaces with underscores
	dststr = strings.ReplaceAll(dststr, " ", "_")
	return dststr, nil
}

// --------------------------------------------------------------------------
func (i Item) replaceExtension(dststr string) string {

	var ext = path.Ext(i.Url)
	// make sure any querystring params are removed
	ext, _, _ = strings.Cut(ext, "?")

	dststr = strings.Replace(dststr, "#ext#", ext, 1)
	dststr = strings.Replace(dststr, "#extension#", ext, 1)

	return dststr
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
			newExtra := fmt.Sprintf(".%v", string(r)) // append dot to extra
			newfilename := fmt.Sprintf("%s%s%s", base, newExtra, ext)
			if collFunc(newfilename) == false {
				filename = newfilename
				extra = newExtra
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
