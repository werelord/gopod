package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"strings"
	"time"
)

// Because providers can't be trusted with their shit, moving to a new provider
// will reset the GUIDs and other things.. Thus everything comes up as new again
// using this to reset the guids on the feed items, based on item titles

type hackFeedProcess struct {
}

func (hfp hackFeedProcess) SkipParsingItem(string) (bool, bool) { return false, false }
func (hfp hackFeedProcess) CancelOnPubDate(time.Time) bool      { return false }
func (hfp hackFeedProcess) CancelOnBuildDate(time.Time) bool    { return false }
func (hfp hackFeedProcess) CalcItemHash(guid string, url string) (string, error) {
	// don't care abut hash for this, just using guid
	return guid, nil
}

func (f *Feed) HackDB() error {

	var (
		log      = f.log
		hfp      = hackFeedProcess{}
		itemMap  map[string]*Item
		modified = make([]*Item, 0)
	)

	// make sure xml is specified
	if config.GuidHackXml == "" {
		return errors.New("guid xml must be specified for guid hack")
	} else if exists, err := podutils.FileExists(config.GuidHackXml); err != nil {
		return fmt.Errorf("error finding guid hack xml: %w", err)
	} else if exists == false {
		return errors.New("guid xml file not found")
	}

	f.log.Debug("hacking feed db")

	if err := f.LoadDBFeed(loadOptions{dontCreate: true, direction: cASC}); err != nil {
		log.Error("error loading feed from db", "err", err)
		return err
	} else if il, err := f.loadDBFeedItems(AllItems,
		loadOptions{dontCreate: true, includeXml: true, direction: cASC}); err != nil {
		log.Error("error lopading items from db", "err", err)
		return err
	} else {
		itemMap = make(map[string]*Item, len(il))
		for _, it := range il {
			itemMap[strings.TrimSpace(it.XmlData.Title)] = it
		}
	}

	// load hack xml
	if body, err := podutils.LoadFile(config.GuidHackXml); err != nil {
		return fmt.Errorf("error loading xml file: %w", err)
	} else if _, itemPairs, err := podutils.ParseXml(body, hfp); err != nil {
		return fmt.Errorf("error parsing xml: %w", err)
	} else {
		// loop thru xml items, match up with existing items via title.. if match found, update the
		// existing item with the new guid

		for _, ip := range itemPairs {
			if item, exists := itemMap[strings.TrimSpace(ip.ItemData.Title)]; exists == false {
				log.Debug("item does not exist in current map", "title", ip.ItemData.Title)
			} else if item.Guid == ip.ItemData.Guid {
				log.Debug("item found, guids match", "title", ip.ItemData.Title)
			} else {
				log.Debug("item found, guid mismatch.. updating",
					"title", ip.ItemData.Title,
					"old guid", item.Guid,
					"new guid", ip.ItemData.Guid,
				)
				item.Guid = ip.ItemData.Guid
				modified = append(modified, item)
			}
		}
	}

	if len(modified) > 0 {
		f.saveDBFeedItems(modified...)
	}

	return nil
}
