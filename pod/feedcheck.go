package pod

import (
	"bufio"
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"

	"github.com/go-test/deep"
)

// --------------------------------------------------------------------------
func (f *Feed) CheckDownloads() error {

	// make sure db is loaded; don't need xml for this
	var (
		itemList  []*Item
		dirtyList = make([]*Item, 0)
		filelist  = make(map[string]*Item, 0)
		err       error
	)

	if err = f.LoadDBFeed(false); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		return err
	} else {
		// load all items (will be sorted desc); we do want item xml
		if itemList, err = f.loadDBFeedItems(-1, true, cASC); err != nil {
			f.log.Error("failed to load item entries: ", err)
			return err
		}
	}
	f.log.Debug("Feed loaded from db for check download")

	// todo: check filename collision

	for _, item := range itemList {
		// check item hashes
		var (
			verifyHash  string
			filePathStr string
			fileExists  bool
			genFilename string
			err         error
		)

		verifyHash, err = calcHash(item.XmlData.Guid, item.XmlData.Enclosure.Url, f.UrlParse)
		if err != nil {
			f.log.Errorf("error calculating hash: %v", err)
			return err
		}
		if verifyHash != item.Hash {
			f.log.Warnf("hash mismatch: calc:'%v', stored:'%v'", verifyHash, item.Hash)
		}

		// check download exists
		filePathStr = filepath.Join(f.mp3Path, item.Filename)
		fileExists, err = podutils.FileExists(filePathStr)
		if err != nil {
			f.log.Error("Error checking file exists: ", err)
			continue
		}

		// check filename collision
		if existItem, exists := filelist[item.Filename]; exists {
			f.log.Warnf("filename collision found: '%v':'%v' ", item, existItem)

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
					f.log.Error("error in scanning; skipping by default: ", err)
				} else {
					switch scanner.Text() {
					case "1":
						f.log.Debugf("deleting existing item: '%v'", existItem.ID)
					case "2":
						f.log.Debugf("deleting current item: '%v'", item.ID)
					default:
						f.log.Debug("Skipping, no entry detected")
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

		if config.SetArchive && fileExists == false {
			f.log.Infof("setting '%v' as archived", item.Filename)
			item.Archived = true
			dirtyList = append(dirtyList, item)
		}

		// logging mismatched file, in case mismatch still exists
		if fileExists == item.Archived {
			f.log.Warnf("%v, archive: %v, exists:%v", filePathStr, item.Archived, fileExists)
		}

		if item.Archived == false {
			if item.Downloaded == false {
				f.log.Warnf("File not downloaded: '%v'", item.Filename)
			} else if fileExists == false {
				f.log.Warnf("file downloaded, but not found: '%v'", item.Filename)
			}
		}

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
						dirtyList = append(dirtyList, item)
					}

				}
			}
		}
	}

	if len(dirtyList) > 0 {
		f.saveDBFeed(nil, dirtyList)
	}

	return nil
}
