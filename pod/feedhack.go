package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"strings"
	"time"
)

func (f *Feed) HackDB() error {

	if config.HackGuidXml != "" {
		return hackGuidXml(f)
	} else if config.HackShortnameRename != "" {
		return hackShortnameRename(f)
	} else {
		return errors.New("valid hack command not found")
	}
}

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

func hackGuidXml(f *Feed) error {
	var (
		log      = f.log
		hfp      = hackFeedProcess{}
		itemMap  map[string]*Item
		modified = make([]*Item, 0)
	)

	// make sure xml is specified
	if config.HackGuidXml == "" {
		return errors.New("guid xml must be specified for guid hack")
	} else if exists, err := podutils.FileExists(config.HackGuidXml); err != nil {
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
	if body, err := podutils.LoadFile(config.HackGuidXml); err != nil {
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

// --------------------------------------------------------------------------
func hackShortnameRename(nfeed *Feed) error {

	if config.HackShortnameRename == "" {
		return errors.New("old shortname must be specified for hack")
	}

	// load the shortname in a new feed
	var (
		log         = nfeed.log
		oldfeedToml = nfeed.FeedToml
		workingfeed *Feed
		itemlist    = make([]*Item, 0)

		oldShortname = config.HackShortnameRename
		newShortname = nfeed.Shortname

		retErr error
	)
	// for loading purposes, make the feed have the old shortname
	oldfeedToml.Shortname = oldShortname

	// load everything into working feed
	if of, err := NewFeed(oldfeedToml); err != nil {
		log.Error("error init old feed", "err", err)
		return err
	} else if err := of.LoadDBFeed(loadOptions{includeXml: true}); err != nil {
		log.Error("error loading feed from db", "err", err)
		return err
	} else if il, err := of.loadDBFeedItems(AllItems,
		loadOptions{dontCreate: true, includeXml: true, direction: cASC}); err != nil {
		log.Error("error lopading items from db", "err", err)
		return err
	} else {
		workingfeed = of
		itemlist = il
	}

	log.Debug("hashes", "old", workingfeed.Hash)

	// everything should be collected.. lets start renaming things
	workingfeed.Shortname = newShortname
	workingfeed.Hash = ""
	workingfeed.generateHash()

	log.Debug("hashes", "new", workingfeed.Hash)

	// loop thru all old xml files, moving them to new shortname
	var (
		oldXmlFilePath = filepath.Join(config.WorkspaceDir, oldShortname, ".xml")
		newXmlFilePath = filepath.Join(config.WorkspaceDir, newShortname, ".xml")
	)

	if xmllist, err := podutils.FindAll(oldXmlFilePath, fmt.Sprintf("%v.*", oldShortname)); err != nil {
		log.Error("error finding xml files", "err", err)
		return err
	} else {
		for _, xmlfile := range xmllist {
			// just use cut
			if _, after, found := strings.Cut(xmlfile, fmt.Sprintf("%v.", oldShortname)); found == false {
				log.Error("bad formatting xml file", "xmlfile", xmlfile)
			} else {
				var newfile = fmt.Sprintf("%v.%v", newShortname, after)
				if config.Simulate {
					log.Info("skipping file move due to sim flag", "old", xmlfile, "new", newfile)
				} else if err := podutils.Rename(filepath.Join(oldXmlFilePath, xmlfile),
					filepath.Join(newXmlFilePath, newfile)); err != nil {

					log.Error("error moving file", "err", err)
					retErr = errors.Join(retErr, err)
				} else {
					log.Info("moved file", "old", xmlfile, "new", newfile)
				}
			}
		}
	}

	// do the same with image files, but will need to change filenames in db
	// index key is url, so shouldn't need to be changed
	var (
		oldImgFilePath = workingfeed.imgPath
		newImgFilePath = nfeed.imgPath
		modImages      = make([]*ImageDBEntry, 0)
	)

	for _, img := range workingfeed.ImageList {
		if exists, err := podutils.FileExists(filepath.Join(oldImgFilePath, img.Filename)); err != nil {
			log.Error("error finding file", "err", err)
			retErr = errors.Join(retErr, err)

		} else if exists {
			// move the file, change the filename in db
			// if it doesn't exists, ignore, probably been archived

			var newfile = strings.ReplaceAll(img.Filename, oldShortname, newShortname)
			if config.Simulate {
				log.Info("skipping file move due to sim flag", "old", img.Filename, "new", newfile)
			} else if err := podutils.Rename(filepath.Join(oldImgFilePath, img.Filename),
				filepath.Join(newImgFilePath, newfile)); err != nil {

				log.Error("error moving file", "err", err)
				retErr = errors.Join(retErr, err)

			} else {
				log.Info("moved file", "old", img.Filename, "new", newfile)
				img.Filename = newfile
				modImages = append(modImages, img)
			}
		}
	}
	// todo: move any extra files??

	var (
		oldItemPath = workingfeed.mp3Path
		newItemPath = nfeed.mp3Path
		modItems    = make([]*Item, 0)

		collfunc = func(string) bool { return false }
	)

	// finally, move items
	for _, item := range itemlist {
		if exists, err := podutils.FileExists(filepath.Join(oldItemPath, item.Filename)); err != nil {
			log.Error("error finding file", "err", err)
			retErr = errors.Join(retErr, err)
		} else if exists {
			// move the file, change the filename in db
			// if it doesn't exists, ignore, probably been archived

			if newFilename, xtra, err := item.generateFilename(nfeed.FeedToml, collfunc); err != nil {
				log.Error("error generating new file file", "err", err)
				retErr = errors.Join(retErr, err)
			} else if config.Simulate {
				log.Info("skipping file move due to sim flag", "old", item.Filename, "new", newFilename)
			} else if err := podutils.Rename(filepath.Join(oldItemPath, item.Filename),
				filepath.Join(newItemPath, newFilename)); err != nil {
				log.Error("error moving file", "err", err)
				retErr = errors.Join(retErr, err)
			} else {
				log.Info("moved file", "old", item.Filename, "new", newFilename)
				item.Filename = newFilename
				item.FilenameXta = xtra
				modItems = append(modItems, item)
			}
		}
	}

	if len(modImages) > 0 {
		if err := workingfeed.saveDBFeedImageList(modImages); err != nil {
			log.Error("error saving images", "err", err)
			retErr = errors.Join(retErr, err)
		}
	}
	if len(modItems) > 0 {
		if err := workingfeed.saveDBFeedItems(modItems...); err != nil {
			log.Error("error saving items", "err", err)
			retErr = errors.Join(retErr, err)
		}
	}
	if err := workingfeed.saveDBFeed(nil, nil); err != nil {
		log.Error("error saving feed", "err", err)
		retErr = errors.Join(retErr, err)
	}

	return retErr
}
