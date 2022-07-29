package podutils

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/flytam/filenamify"
	log "github.com/sirupsen/logrus"
)

const filenamMaxLength = 200

//--------------------------------------------------------------------------
func SaveToFile(buf []byte, filename string) error {

	log.Debug("Saving to file: " + filename)

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	if err != nil {
		return err
	}
	defer file.Close()
	count, err := file.Write(buf)
	if err != nil {
		return err
	} else {
		log.Debug("bytes written to file: " + fmt.Sprint(count))
	}
	return nil
}

//--------------------------------------------------------------------------
// load file (unbuffered)
func loadFile(filename string) (buf []byte, err error) {

	var file *os.File

	log.Debug("loading data from file: " + filename)
	if file, err = os.Open(filename); err != nil {
		return nil, err
	}
	defer file.Close()

	if buf, err = io.ReadAll(file); err != nil {
		return nil, err
	}

	return buf, nil
}

//--------------------------------------------------------------------------
func CleanFilename(filename string) string {
	// only error this generates is if the replacement is a reserved character
	fname, _ := filenamify.Filenamify(filename, filenamify.Options{Replacement: "-", MaxLength: filenamMaxLength})
	return fname
}

//--------------------------------------------------------------------------
func RotateFiles(path, regexPattern string, numToKeep uint) error {

	var (
		r        *regexp.Regexp
		err      error
		filelist []fs.FileInfo
	)

	if r, err = regexp.Compile(regexPattern); err != nil {
		return err
	}

	err = filepath.WalkDir(path, func(filename string, d fs.DirEntry, err error) error {
		if d.IsDir() == false {
			if r.MatchString(filename) {
				if fi, e := d.Info(); e == nil {
					filelist = append(filelist, fi)
				} else {
					log.Warn("error getting file info (not adding to list): ", e)
				}
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	// make sure we have any to remove
	if len(filelist) > int(numToKeep) {

		// sort the resulting slice
		sort.Slice(filelist, func(i, j int) bool {
			return filelist[i].ModTime().After(filelist[j].ModTime())
		})

		// remove the X number of files beyond the limit
		for _, f := range filelist[numToKeep:] {
			log.Debug("removing file: ", f.Name())
			if err := os.Remove(filepath.Join(path, f.Name())); err != nil {
				log.Warnf("failed to remove '%v': %v", f.Name(), err)
			}
		}
	}

	return nil
}
