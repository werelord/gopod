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

const filenamMaxLength = 240

// --------------------------------------------------------------------------
func SaveToFile(buf []byte, filename string) error {

	//log.Debug("Saving to file: " + filename)

	file, err := osimpl.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)

	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(buf)
	if err != nil {
		return err
	} /*else {
		log.Debug("bytes written to file: " + fmt.Sprint(count))
	}*/
	return nil
}

// --------------------------------------------------------------------------
// load file (unbuffered)
func LoadFile(filename string) ([]byte, error) {

	var (
		file fileInterface
		err  error
	)

	//log.Debug("loading data from file: " + filename)
	if file, err = osimpl.Open(filename); err != nil {
		return nil, err
	}
	defer file.Close()

	return io.ReadAll(file)
}

// --------------------------------------------------------------------------
func CleanFilename(filename string, rep string) string {
	// only error this generates is if the replacement is a reserved character
	fname, _ := filenamify.FilenamifyV2(filename, func(options *filenamify.Options) {
		options.Replacement = rep
		options.MaxLength = filenamMaxLength
	})
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
	entries, err := osimpl.ReadDir(path)
	if err != nil {
		return err
	}

	filelist = make([]fs.FileInfo, 0)

	for _, entry := range entries {
		if entry.IsDir() == false {
			if match, err := filepath.Match(pattern, entry.Name()); err != nil {
				return err
			} else if match {
				if info, err := entry.Info(); err == nil {
					//fmt.Printf("file: %v\n", info.Name())
					filelist = append(filelist, info)
				}
			}
		}
	}

	// the file list is by default sorted by filename; as we're assuming rotate has the timestamp in the filename,
	// these list should already be sorted by timestamp

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
			if err := osimpl.Remove(filepath.Join(path, f.Name())); err != nil {
				log.Warnf("failed to remove '%v': %v", f.Name(), err)
			}
		}
	}
	return nil
}

// --------------------------------------------------------------------------
func FindMostRecent(path, pattern string) (string, error) {
	// glob uses fstat, but just returns the filename..
	// walkdir does return fs.DirEntry, but is recursive on the tree
	// instead, use os.ReadDir() just to search the given path directory

	latestFile := struct {
		t        time.Time
		filename string
	}{}

	filelist, err := osimpl.ReadDir(path)
	if err != nil {
		return "", err
	}

	for _, file := range filelist {
		if file.IsDir() == false {
			if fi, err := file.Info(); err != nil {
				log.Warn("info returned error: ", err)
			} else if match, err := filepath.Match(pattern, fi.Name()); err != nil {
				log.Error("match returned error: ", err)
				return "", err
			} else if match == true {
				// check timestamp
				if fi.ModTime().After(latestFile.t) {
					latestFile.t = fi.ModTime()
					latestFile.filename = fi.Name()
				}
			}
		}
	}
	if latestFile.t.IsZero() || latestFile.filename == "" {
		return "", fmt.Errorf("unable to find file: %w", fs.ErrNotExist)
	}
	return filepath.Join(path, latestFile.filename), nil
}

// --------------------------------------------------------------------------
func CreateSymlink(source, symDest string) error {
	if exists, err := FileExists(symDest); err != nil {
		log.Error("Failed checking symlink already exists: ", err)
		return err
	} else if exists {
		// remove the symlink before recreating it..
		if err := osimpl.Remove(symDest); err != nil {
			log.Warn("failed to remove latest symlink: ", err)
			return err
		}
	}

	if err := osimpl.Symlink(source, symDest); err != nil {
		log.Warn("failed to create symlink: ", err)
		return err
	}
	return nil
}

// --------------------------------------------------------------------------
func FileExists(filename string) (bool, error) {
	// corrected based on https://stackoverflow.com/questions/12518876/how-to-check-if-a-file-exists-in-go
	if _, err := osimpl.Stat(filename); err == nil {
		return true, nil
	} else if errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else {
		// Schrodinger: file may or may not exist. See err for details.
		return false, err
	}
}

// --------------------------------------------------------------------------
// simply a shortcut to os.MkdirAll with permissions
func MkdirAll(pathList ...string) error {
	for _, path := range pathList {
		if err := osimpl.MkdirAll(path, 0666); err != nil {
			return err
		}
	}
	return nil
}

func CreateTemp(dir, pattern string) (*os.File, error) {
	return osimpl.CreateTemp(dir, pattern)
}
func Rename(oldpath, newpath string) error {
	return osimpl.Rename(oldpath, newpath)
}
func Chtimes(path string, access time.Time, modification time.Time) error {
	return osimpl.Chtimes(path, access, modification)
}
