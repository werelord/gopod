package main

import (
	"strings"

	"crypto/sha1"
	"encoding/base64"
	"net/url"

	"github.com/araddon/dateparse"
	"github.com/beevik/etree"
)

// separating out this to keep feed.go not as verbose

//--------------------------------------------------------------------------
func (f *Feed) parseXml(xmldata []byte) bool {
	// fuck the ignore list
	//ignoreList := []string{"atom:link", "lastBuildDate"}

	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmldata); err != nil {
		log.Error("failed to read xml document", err)
		return false
	}

	root := doc.SelectElement("rss").SelectElement("channel")
	log.Debug("root element: " + root.Tag)

	//itemList := []*etree.Element{}

	var dupItemCount = 0

	for _, elem := range root.ChildElements() {

		// for now, direct insert..
		// todo: later use reflection
		switch {
		case strings.EqualFold(elem.FullTag(), "title"):
			f.feedData.title = elem.Text()
		case strings.EqualFold(elem.FullTag(), "pubdate"):
			t, err := dateparse.ParseAny(elem.Text())
			if err != nil {
				log.Debug("Error parsing timestamp: " + elem.Text())
			} else {
				f.feedData.pubdate = t
			}
		case strings.EqualFold(elem.FullTag(), "link"):
			f.feedData.link = elem.Text()
		case strings.EqualFold(elem.FullTag(), "image"):
			if urlnode := elem.SelectElement("url"); urlnode != nil {
				f.feedData.image.url = elem.SelectElement("url").Text()
			}
			if titlenode := elem.SelectElement("title"); titlenode != nil {
				f.feedData.image.title = titlenode.Text()
			}
			if linknode := elem.SelectElement("link"); linknode != nil {
				f.feedData.image.link = linknode.Text()
			}

		case strings.EqualFold(elem.FullTag(), "itunes:author"):
			f.feedData.author = elem.Text()
		case strings.EqualFold(elem.FullTag(), "description"):
			f.feedData.description = elem.Text()
		case strings.EqualFold(elem.FullTag(), "item"):
			// adding items to a slice, for future handling without iterating thru the childlist again

			// todo: move hash checking here, append only new entries
			if dupItemCount < f.config.MaxDupChecks {
				if exists, hash := f.checkExists(elem); exists {
					// exists, increment the dup counter
					log.Debug("item exists, incrementing dup counter")
					dupItemCount++
				} else {
					// todo: parse this shit
					f.saveItem(hash, parseItemEntry(elem))
				}
			} else {
				log.Debug("dup counter over limit, skipping...")
			}

			// itemList = append(itemList, elem)
		default:
			//log.Debug("unhandled tag: " + elem.FullTag())
		}
		//log.Debug("tag: " + elem.Tag)
	}

	// itemlist handling
	// for _, item := range itemList {
	// 	if f.parseItem(item) == false {
	// 		log.Debug("stopping parsing of items for " + f.Shortname)
	// 		break
	// 	}
	// }

	log.Debugf("%+v", f)

	return true

}

//--------------------------------------------------------------------------
func (f *Feed) checkExists(elem *etree.Element) (exists bool, hash string) {
	// first, get the guid/urlstr, check to see if it exists
	var (
		guid string
		urlstr  string
	)

	if guidnode := elem.FindElement("guid"); guidnode != nil {
		guid = guidnode.Text()
	}
	if enclosurenode := elem.FindElement("enclosure"); enclosurenode != nil {
		if urlnode := enclosurenode.SelectAttr("url"); urlnode != nil {
			u, err := url.Parse(urlnode.Value)
			if (err != nil) {
				log.Error("error in parsing url: ", err)
			}
			u.RawQuery = ""
			u.Fragment = ""
			
			urlstr = u.String()
		}
	}

	if guid == "" && urlstr == "" {
		log.Error("failed to hash item; guid and url not parsed!")
		return false, ""
	}

	// at least url or guid exists, hash the combination
	// todo: remove querystring values from url
	sha := sha1.New()
	sha.Write([]byte(guid + urlstr))
	//hash := sha.Sum(nil)
	//bin := hasher.Sum(nil)
	hash = base64.URLEncoding.EncodeToString(sha.Sum(nil))

	// check to see if hash exists
	_, exists = f.itemlist[hash]
	log.Debugf("guid: '%+v', url: '%+v', hash: '%+v', exists: '%+v'", guid, urlstr, hash, exists)
	return exists, hash
}

//--------------------------------------------------------------------------
func parseItemEntry(elem *etree.Element) (item ItemData) {

	return
}
