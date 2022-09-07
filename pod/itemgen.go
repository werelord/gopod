package pod

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

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
