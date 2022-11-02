package pod

import (
	"bufio"
	"errors"
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"

	"github.com/go-test/deep"
)

type ActionTakenError struct{ actionTaken string }

func (r ActionTakenError) Error() string        { return r.actionTaken }
func (r ActionTakenError) Is(target error) bool { return target == ActionTakenError{} }

type fileCheckStatus struct {
	feed          *Feed
	fileExistsMap map[string]bool

	itemList  []*Item
	dirtyList []*Item
}

// todo: additional command instead of option, based on how I'm handling shit here..

// --------------------------------------------------------------------------
func (f *Feed) CheckDownloads() error {

	var fcs = fileCheckStatus{
		fileExistsMap: make(map[string]bool),
		dirtyList:     make([]*Item, 0),
	}

	// make sure db is loaded; don't need xml for this
	var (
		err error
	)

	if err = f.LoadDBFeed(false); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		return err
	} else {
		// load all items (will be sorted desc); we do want item xml
		if fcs.itemList, err = f.loadDBFeedItems(-1, true, cASC); err != nil {
			f.log.Error("failed to load item entries: ", err)
			return err
		}
	}
	f.log.Debug("Feed loaded from db for check download")

	fcs.feed = f

	if err := fcs.checkHashes(); err != nil {
		f.log.Error("error in checking hashes: ", err)
		return err
	}
	if err := fcs.checkCollisions(); err != nil {
		if errors.Is(err, ActionTakenError{}) {
			return nil
		}
		f.log.Error("error in checking collisions: ", err)
		return err
	}
	if err := fcs.checkArchiveStatus(); err != nil {
		if errors.Is(err, ActionTakenError{}) {
			return nil
		}
		f.log.Error("error in checking archive: ", err)
		return err
	}

	for _, item := range fcs.itemList {
		// check item hashes
		var (
			fileExists  bool
			genFilename string
			err         error
		)

		// check filename generation, in case shit changed.. only check non-archived (as to the arcane rules initially set up)
		if item.Archived == false {

			if genFilename, err = item.generateFilename(f.FeedToml); err != nil {
				f.log.Error("error generating filename: ", err)
				continue
			}
			if genFilename != item.Filename {
				if config.DoRename == false {
					f.log.Warnf("filename mismatch; item.Filename: '%v', genFilename: '%v'", item.Filename, genFilename)
				} else {

					if fileExists == false {
						f.log.Warnf("cannot rename file '%v'; file does not exist.. skipping rename", item.Filename)
					} else if err = podutils.Rename(filepath.Join(f.mp3Path, item.Filename),
						filepath.Join(f.mp3Path, genFilename)); err != nil {

						f.log.Warnf("error in renaming file '%v'; skipping commit: %v", item.Filename, err)
					} else {
						// rename successful, commit the change

						item.Filename = genFilename
						fcs.dirtyList = append(fcs.dirtyList, item)
					}

				}
			}
		}
	}

	if len(fcs.dirtyList) > 0 {
		f.saveDBFeed(nil, fcs.dirtyList)
	}

	return nil
}

// --------------------------------------------------------------------------
func (fcs *fileCheckStatus) checkHashes() error {

	var log = fcs.feed.log
	for _, item := range fcs.itemList {
		var verifyHash, err = calcHash(item.XmlData.Guid, item.XmlData.Enclosure.Url, fcs.feed.UrlParse)
		if err != nil {
			log.Errorf("error calculating hash: %v", err)
			return err
		}
		if verifyHash != item.Hash {
			log.Warnf("hash mismatch: calc:'%v', stored:'%v'", verifyHash, item.Hash)
		}
	}
	return nil
}

// --------------------------------------------------------------------------
func (fcs *fileCheckStatus) checkCollisions() error {

	var (
		filelist = make(map[string]*Item, 0)
		log      = fcs.feed.log
	)

	for _, item := range fcs.itemList {
		// check filename collision
		if existItem, exists := filelist[item.Filename]; exists {
			log.Warnf("filename collision found: '%v':'%v' ", item, existItem)

			// display comparision on collision
			for _, diff := range deep.Equal(item, existItem) {
				fmt.Printf("\t%v\n", diff)
			}

			// input choice
			// todo: separate out into separate package??
			if config.DoCollision {
				scanner := bufio.NewScanner(os.Stdin)
				fmt.Printf("Which to delete:\n\t'%v' (1)\n\t'%v' (2)\n\tSkip (no entry)\n\t(1|2|<newline>)>", existItem.ID, item.ID)
				scanner.Scan()
				if scanner.Err() != nil {
					log.Error("error in scanning; skipping by default: ", scanner.Err())
				} else {
					switch scanner.Text() {
					case "1":
						log.Debugf("deleting existing item: '%v'", existItem.ID)
					case "2":
						log.Debugf("deleting current item: '%v'", item.ID)
					default:
						log.Debug("Skipping collision; no action taken")
					}
				}
			}

			// write the file to output for comnparison
			// file1path := filepath.Join(config.WorkspaceDir, ".collision", "one", item.Filename+".txt")
			// file2path := filepath.Join(config.WorkspaceDir, ".collision", "two", item.Filename+".txt")
			// podutils.MkdirAll(filepath.Dir(file1path), filepath.Dir(file2path))
			// file1, err := os.OpenFile(file1path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			// if err != nil {
			// 	f.log.Error(err)
			// }
			// file2, err := os.OpenFile(file2path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
			// if err != nil {
			// 	f.log.Error(err)
			// }
			// defer file1.Close()
			// defer file2.Close()
			// itembytes, _ := json.MarshalIndent(item, "", "   ")
			// existbytes, _ := json.MarshalIndent(existItem, "", "   ")
			// file1.Write(itembytes)
			// file2.Write(existbytes)

		} else {
			// save the reference, for subsequent checks
			filelist[item.Filename] = item
		}
	}

	if config.DoCollision {
		// todo: do the delete motherfucker
		return ActionTakenError{actionTaken: "filename collision handling"}
	}
	return nil
}

// --------------------------------------------------------------------------
func (fcs *fileCheckStatus) checkArchiveStatus() error {

	var (
		log = fcs.feed.log
	)

	for _, item := range fcs.itemList {

		var fileExists = fcs.fileExists(item)
		if config.DoArchive && fileExists == false {
			log.Infof("setting '%v' as archived", item.Filename)
			item.Archived = true
			fcs.dirtyList = append(fcs.dirtyList, item)
		}

		// logging mismatched file, in case mismatch still exists
		if fileExists == item.Archived {
			log.Warnf("%v, archive: %v, exists:%v", item.Filename, item.Archived, fileExists)
		}

		if item.Archived == false {
			if item.Downloaded == false {
				log.Warnf("File not downloaded: '%v'", item.Filename)
			} else if fileExists == false {
				log.Warnf("file downloaded, but not found: '%v'", item.Filename)
			}
		}
	}

	if config.DoArchive {
		if len(fcs.dirtyList) > 0 {
			if err := fcs.feed.saveDBFeed(nil, fcs.dirtyList); err != nil {
				return err
			}
		}
		return ActionTakenError{actionTaken: "set archive"}
	}

	return nil
}

// --------------------------------------------------------------------------
func (fcs *fileCheckStatus) fileExists(item *Item) bool {

	// check download exists
	if status, ok := fcs.fileExistsMap[item.Filename]; ok {
		return status
	} else {
		// cache the result
		var filePathStr = filepath.Join(fcs.feed.mp3Path, item.Filename)
		var fileExists, err = podutils.FileExists(filePathStr)
		if err != nil {
			fcs.feed.log.Error("Error checking file exists: ", err)
		}
		fcs.fileExistsMap[item.Filename] = fileExists
		return fileExists
	}
}
