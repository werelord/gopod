package podutils

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/flytam/filenamify"
	log "github.com/sirupsen/logrus"
)

const filenamMaxLength = 200

// --------------------------------------------------------------------------
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

// --------------------------------------------------------------------------
// load file (unbuffered)
func LoadFile(filename string) (buf []byte, err error) {

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

// --------------------------------------------------------------------------
func CleanFilename(filename string) string {
	// only error this generates is if the replacement is a reserved character
	fname, _ := filenamify.Filenamify(filename, filenamify.Options{Replacement: "-", MaxLength: filenamMaxLength})
	return fname
}

// --------------------------------------------------------------------------
func RotateFiles(path, pattern string, numToKeep uint) error {

	var (
		err      error
		filelist []fs.FileInfo
	)

	// note: always keep latest, even if it is empty.. symlinks might be pointing to that
	// anything older than latest that is empty (size == 0) straight up remove,
	// don't include in the num to keep
	if path == "" {
		return errors.New("path cannot be empty")
	} else if pattern == "" {
		return errors.New("pattern cannot be empty")
	} else if numToKeep == 0 {
		log.Warn("numToKeep must be greater than 0; setting to 1")
		numToKeep = 1
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	filelist = make([]fs.FileInfo, 0)

	for _, entry := range entries {
		if entry.IsDir() == false {
			if match, _ := filepath.Match(pattern, entry.Name()); match {
				if info, err := entry.Info(); err == nil {
					//fmt.Printf("file: %v\n", info.Name())
					filelist = append(filelist, info)
				}
			}
		}
	}

	// remove the X number of files beyond the limit
	// always keep the last entry in the list regardless of size,
	// as that one likely has the most recent symlink attached
	var keepCount uint = 0
	var startIndex = len(filelist) - 1 // don't need to store this, just makes referencing it twice easier
	for i := startIndex; i >= 0; i-- {
		f := filelist[i]
		var remove bool
		if i < startIndex && f.Size() == 0 {
			// auto-delete
			//fmt.Printf("autodelete (size): %v\n", f.Name())
			remove = true
		} else if keepCount < numToKeep {
			keepCount++
			//fmt.Printf("        keep (%v): %v\n", keepCount, f.Name())
		} else {
			//fmt.Printf("           delete: %v\n", f.Name())
			remove = true
		}

		if remove {
			if err := os.Remove(filepath.Join(path, f.Name())); err != nil {
				log.Warnf("failed to remove '%v': %v", f.Name(), err)
			}
		}
	}
	return nil
}

// --------------------------------------------------------------------------
func FindMostRecent(path, pattern string) (string, error) {
	// glob uses fstat, but just returns the filename.. walkdir is beter

	latestFile := struct {
		t        time.Time
		filename string
	}{}

	if err := filepath.WalkDir(path,
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() == false {
				if fi, err := d.Info(); err != nil {
					log.Warnf("info returned error: ", err)
				} else if match, err := filepath.Match(pattern, fi.Name()); err != nil {
					log.Warnf("match returned error: ", err)
				} else if match == true {
					// check timestamp
					if fi.ModTime().After(latestFile.t) {
						latestFile.t = fi.ModTime()
						latestFile.filename = fi.Name()
					}
				}
			}
			return nil
		},
	); err != nil {
		log.Warnf("walkdir returned error: ", err)
	} else if latestFile.t.IsZero() || latestFile.filename == "" {
		return "", errors.New("unable to find file")
	}
	return filepath.Join(path, latestFile.filename), nil
}

// --------------------------------------------------------------------------
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

// --------------------------------------------------------------------------
func FileExists(filename string) bool {
	_, err := os.Stat(filename)
	bo1 := err == nil
	bo2 := (errors.Is(err, os.ErrNotExist) == false)
	return bo1 || bo2
	//return (err == nil) || (errors.Is(err, os.ErrNotExist) == false)
}

// --------------------------------------------------------------------------
func MkDirAll(path string) error {
	err := os.MkdirAll(path, 0666)
	if err != nil && os.IsExist(err) == false {
		return err
	}
	return nil
}
