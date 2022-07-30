package podutils

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
func RotateFiles(path, pattern string, numToKeep uint) error {

	var (
		err      error
		filelist []string
	)

	fp := filepath.Join(path, pattern)

	if filelist, err = filepath.Glob(fp); err != nil {
		return err
	}

	// make sure we have any to remove
	if len(filelist) > int(numToKeep) {
		sort.Sort(sort.Reverse(sort.StringSlice(filelist)))
		for i, f := range filelist {
			fmt.Printf("%v: %v\n", i, f)
		}

		// remove the X number of files beyond the limit
		for _, f := range filelist[numToKeep:] {
			log.Debug("removing file: ", filepath.Base(f))
			if err := os.Remove(f); err != nil {
				log.Warnf("failed to remove '%v': %v", filepath.Base(f), err)
			}
		}
	}

	return nil
}

//--------------------------------------------------------------------------
func CreateSymlink(source, symDest string) error {
	if FileExists(symDest) {
		// remove the symlink before recreating it..
		if err := os.Remove(symDest); err != nil {
			log.Warn("failed to remove latest symlink: ", err)
			return err
		}
	}

	if err := os.Symlink(source, symDest); err != nil {
		log.Warn("failed to create symlink: ", err)
		return err
	}
	return nil
}

//--------------------------------------------------------------------------
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	bo1 := err == nil
	bo2 := (errors.Is(err, os.ErrNotExist) == false)
	return bo1 || bo2
	//return (err == nil) || (errors.Is(err, os.ErrNotExist) == false)
}
