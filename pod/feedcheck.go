package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"gopod/inputoption"
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

	itemList []*Item
}

// todo: additional command instead of option, based on how I'm handling shit here..

// --------------------------------------------------------------------------
func (f *Feed) CheckDownloads() error {

	var (
		fcs = fileCheckStatus{
			fileExistsMap: make(map[string]bool),
		}
		err error
	)

	// make sure db is loaded; don't need xml for this
	if err = f.LoadDBFeed(false); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		if errors.Is(err, &ErrorFeedDeleted{}) {
			// todo: for now, this works.. eventually put this message in main based on error
			f.log.Warn("Feed deleted; make sure to remove from config file")
		}
		return err
	} else {
		// load all items (will be sorted desc); we do want item xml
		if fcs.itemList, err = f.loadDBFeedItems(AllItems, true, cASC); err != nil {
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
	if err := fcs.checkGuids(); err != nil {
		f.log.Error("error in checking guids: ", err)
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
	if err := fcs.checkGenFilename(); err != nil {
		if errors.Is(err, ActionTakenError{}) {
			return nil
		}
		f.log.Error("error in checking filename generation: ", err)
		return err
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
func (fcs *fileCheckStatus) checkGuids() error {
	var (
		log     = fcs.feed.log
		guidmap = make(map[string]*Item, len(fcs.itemList))
	)
	for _, item := range fcs.itemList {
		if existItem, exists := guidmap[item.XmlData.Guid]; exists {
			log.Warnf("guid collision found (%v); existing: %v, current: %v", item.XmlData.Guid, existItem.ID, item.ID)
		} else {
			guidmap[item.XmlData.Guid] = item
		}
	}
	return nil
}

// --------------------------------------------------------------------------
func (fcs *fileCheckStatus) checkCollisions() error {

	var (
		filelist   = make(map[string]*Item, len(fcs.itemList))
		log        = fcs.feed.log
		deleteList = make([]*Item, 0)
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
			if config.DoCollision {

				var (
					inpExisting = inputoption.GenOption(fmt.Sprintf("%v", existItem.ID), '1', false)
					inpCurrent  = inputoption.GenOption(fmt.Sprintf("%v", item.ID), '2', false)
					inpSkip     = inputoption.GenOption("Skip (no entry)", '\n', true)
				)

				if opt, err := inputoption.RunSelection("which to keep:", inpExisting, inpCurrent, inpSkip); err != nil {
					log.Errorf("error encountered: '%v' skipping by default", err)
				} else {
					switch opt {
					case inpExisting:
						log.Debugf("deleting current item: '%v'", item.ID)
						deleteList = append(deleteList, item)
					case inpCurrent:
						log.Debugf("deleting existing item: '%v'", existItem.ID)
						deleteList = append(deleteList, existItem)
					case inpSkip:
						fallthrough
					default:
						log.Debug("Skipping collision; no action taken")
					}
				}
			} else if config.SaveCollision {
				// write the file to output for comnparison
				var collisionPath = filepath.Join(config.WorkspaceDir, ".collision")
				if err := podutils.MkdirAll(collisionPath); err != nil {
					return err
				}
				for _, it := range []*Item{item, existItem} {
					filepath := filepath.Join(collisionPath, fmt.Sprintf("%s.%d.txt", it.Filename, it.ID))
					file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0666)
					if err != nil {
						log.Error(err)
						return err
					}
					defer file.Close()
					bytes, err := json.MarshalIndent(it, "", "   ")
					if err != nil {
						log.Error(err)
						return err
					}
					if _, err := file.Write(bytes); err != nil {
						return err
					}
				}
			}

		} else {
			// save the reference, for subsequent checks
			filelist[item.Filename] = item
		}
	}

	if config.DoCollision {
		if len(deleteList) > 0 {
			if err := fcs.feed.deleteFeedItems(deleteList); err != nil {
				log.Error(err)
			} else {
				log.Debugf("successfully deleted items, len: %v", len(deleteList))
			}
		} else {
			log.Info("No collisions found, nothing to delete")
		}
		return ActionTakenError{actionTaken: "filename collision handling"}
	}
	return nil
}

// --------------------------------------------------------------------------
func (fcs *fileCheckStatus) checkArchiveStatus() error {

	var (
		log       = fcs.feed.log
		dirtyList = make([]*Item, 0)
	)

	for _, item := range fcs.itemList {

		var fileExists = fcs.fileExists(item)

		// don't need to do this for stuff already archived
		if (config.DoArchive) && (item.Archived == false) && (fileExists == false) {
			log.Infof("setting '%v' as archived", item.Filename)
			item.Archived = true
			dirtyList = append(dirtyList, item)
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
		if len(dirtyList) > 0 {
			if err := fcs.feed.saveDBFeed(nil, dirtyList); err != nil {
				return err
			}
		}
		return ActionTakenError{actionTaken: "set archive"}
	}

	return nil
}

func (fcs *fileCheckStatus) checkGenFilename() error {

	// check filename generation, in case shit changed.. only check non-archived (as to the arcane rules initially set up)
	var (
		log       = fcs.feed.log
		dirtyList = make([]*Item, 0)
	)
	for _, item := range fcs.itemList {
		if item.Archived == false {

			var (
				genFilename string
				err         error
			)
			// collision already handled, and filename extra should already be set.. just return
			if genFilename, _, err = item.generateFilename(fcs.feed.FeedToml, nil); err != nil {
				log.Error("error generating filename: ", err)
				continue
			}
			if genFilename != item.Filename {
				if config.DoRename {
					if fcs.fileExists(item) == false {
						log.Warnf("cannot rename file '%v'; file does not exist.. skipping rename", item.Filename)
					} else if err = podutils.Rename(filepath.Join(fcs.feed.mp3Path, item.Filename),
						filepath.Join(fcs.feed.mp3Path, genFilename)); err != nil {

						log.Warnf("error in renaming file '%v'; skipping commit: %v", item.Filename, err)
					} else {
						// rename successful, commit the change
						item.Filename = genFilename
						dirtyList = append(dirtyList, item)
					}

				} else {
					log.Warnf("filename mismatch; item.Filename: '%v', genFilename: '%v'", item.Filename, genFilename)
				}
			}
		}
	}

	if config.DoRename {
		if len(dirtyList) > 0 {
			fcs.feed.saveDBFeed(nil, dirtyList)
		}

		return ActionTakenError{"generate filename rename"}
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
