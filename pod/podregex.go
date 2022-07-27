package pod

import (
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
)

//--------------------------------------------------------------------------
//------------------------------------- DEBUG -------------------------------------
func (f Feed) titleSubmatchRegex(regex, dststr, title string) (string, error) {
	//------------------------------------- DEBUG -------------------------------------

	var (
		r   *regexp.Regexp
		err error

		//------------------------------------- DEBUG -------------------------------------
		//seasonhack  = []string{"S1E", "S2E", "S3E", "S4E", "S5E", "S6E", "S7E"}
		episodehack = []string{
			"Rura Penthe™ Brand Gavel",
			"Puts the Chew on the Other Ass",
			"A Real Meet Scary",
			"Wormhole Ambergris",
		}
		//------------------------------------- DEBUG -------------------------------------
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
			// move thise var in after removal
			var repl string
			if i > len(matchSlice) {
				repl = ""
			} else {

				repl = strings.TrimSpace(matchSlice[i])
				//------------------------------------- DEBUG -------------------------------------
				if (config.Debug) && (i == 1) && (repl == "") {
					repl = "TNG"
					for _, t := range episodehack {
						if strings.Contains(title, t) {
							repl = "DS9"
						}
					}
				} else if (config.Debug) && (i == 1) && (repl == "Voyager") {
					repl = "VOY"
				}
			}
			//------------------------------------- DEBUG -------------------------------------
			dststr = strings.Replace(dststr, str, repl, 1)
			//log.Debug(i, ": ", dststr)
		}
	}

	// in case all are blank.. remove any ".." found

	return strings.ReplaceAll(dststr, "..", "."), nil

}
