package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"time"
)

type previewFeedProcess struct {
	feedUpdate
}

func (pfp previewFeedProcess) SkipParsingItem(string) (bool, bool) { return false, false }
func (pfp previewFeedProcess) CancelOnPubDate(time.Time) bool      { return false }
func (pfp previewFeedProcess) CancelOnBuildDate(time.Time) bool    { return false }
func (pfp previewFeedProcess) CalcItemHash(guid string, url string) (string, error) {
	return calcHash(guid, url, pfp.feed.UrlParse)
}

// --------------------------------------------------------------------------
func (f *Feed) Preview() error {

	// like update, but just downloads/loads the xml, outputs the file naming structure as a preview
	// does not require db access at all.. mainly for new feeds

	var (
		fprev = previewFeedProcess{
			feedUpdate: feedUpdate{feed: f},
		}

		itemPairs []podutils.ItemPair
	)

	f.log.Debug("loading preview")

	// download/load feed xml
	if body, err := fprev.loadNewXml(); err != nil {
		f.log.Error("error in loading xml; ", err)
		return err
	} else {
		if config.UseMostRecentXml == false {
			fprev.saveAndRotateXml(body, true)
		}

		if _, itemPairs, err = podutils.ParseXml(body, fprev); err != nil {
			f.log.Error("error in parsing xml: ", err)
			return err
		}
	}

	var (
		fileCollList = make(map[string]*Item, len(itemPairs))
		collFunc     = func(filename string) bool {
			_, exists := fileCollList[filename]
			return exists
		}
		itemCount    = f.EpisodeCount
		filenameList = make([]string, 0, len(itemPairs))
	)

	// list comes out newest (top of xml feed) to oldest.. reverse that,
	// go oldest to newest, to maintain item count
	for i := len(itemPairs) - 1; i >= 0; i-- {

		var (
			hash    = itemPairs[i].Hash
			xmldata = itemPairs[i].ItemData
		)

		if itemPairs[i].ItemData == nil {
			err := errors.New("xml data is nil")
			f.log.Error(err)
			return err
		} else if previewItem, err := createNewItemEntry(f.FeedToml, hash, xmldata, itemCount+1, collFunc); err != nil {
			f.log.Error("error creating item: ", err)
			return err
		} else {
			// add the filename to collision list
			fileCollList[previewItem.Filename] = previewItem
			filenameList = append(filenameList, previewItem.Filename)
			itemCount++
		}
	}

	var fileout string

	for idx, fname := range filenameList {
		// output to screen, and also file
		fmt.Printf("%3d: %v\n", idx+1, fname)

		fileout += fmt.Sprintf("%v\n", fname)
	}

	var previewFile = filepath.Join(config.WorkspaceDir, fmt.Sprintf("%v.preview.txt", f.Shortname))
	if err := podutils.SaveToFile([]byte(fileout), previewFile); err != nil {
		return err
	}

	return nil
}
