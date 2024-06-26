package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"slices"
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
		f.log.Errorf("error in loading xml: %v", err)
		return err
	} else {
		if config.UseMostRecentXml == false {
			fprev.saveAndRotateXml(body, true)
		}

		if _, itemPairs, err = podutils.ParseXml(body, fprev); err != nil {
			f.log.Errorf("error in parsing xml: %v", err)
			return err
		}
	}

	var (
		fileCollList = make(map[string]*Item, len(itemPairs))
		collFunc     = func(filename string) bool {
			_, exists := fileCollList[filename]
			return exists
		}
		itemCount int
		itemList  = make([]*Item, 0, len(itemPairs))
	)

	if (f.EpisodeCount == 0) && f.CountStart != 0 {
		f.log.Debugf("new feed (?); episode count == 0 and countStart == %v; setting episodeCount to countStart", f.CountStart)
		itemCount = f.CountStart
	} else {
		itemCount = f.EpisodeCount
	}

	// list comes out newest (top of xml feed) to oldest.. reverse that,
	// go oldest to newest, to maintain item count
	// unless std chrono; then just reverse the reversal.. bah
	if (f.StdChrono) {
		slices.Reverse(itemPairs)
	}
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
			f.log.Errorf("error creating item: %v", err)
			return err
		} else {
			// add the filename to collision list
			fileCollList[previewItem.Filename] = previewItem

			// f.log.Debugf("item: '%v'", previewItem)
			// f.log.Debugf("\nOldurl: '%v'\nnewUrl: '%v'\n", previewItem.XmlData.Enclosure.Url, previewItem.Url)

			itemList = append(itemList, previewItem)
			itemCount++
		}
	}

	var fileout string

	for idx, item := range itemList {
		// output to screen, and also file
		fmt.Printf("%3d: %v (%v)\n", idx+1, item.Filename, item.Url)

		fileout += fmt.Sprintf("%v\n", item.Filename)
	}

	var previewFile = filepath.Join(config.WorkspaceDir, fmt.Sprintf("%v.preview.txt", f.Shortname))
	if err := podutils.SaveToFile([]byte(fileout), previewFile); err != nil {
		return err
	}

	return nil
}
