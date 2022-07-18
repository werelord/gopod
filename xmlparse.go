package main

import (
	"fmt"
	"strings"

	"hash/maphash"

	"github.com/araddon/dateparse"
	"github.com/beevik/etree"
)

// separating out this to keep feed.go not as verbose

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

	itemList := []*etree.Element{}

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
			f.feedData.image.url = elem.SelectElement("url").Text()
			f.feedData.image.title = elem.SelectElement("title").Text()
			f.feedData.image.link = elem.SelectElement("link").Text()
		case strings.EqualFold(elem.FullTag(), "itunes:author"):
			f.feedData.author = elem.Text()
		case strings.EqualFold(elem.FullTag(), "description"):
			f.feedData.description = elem.Text()
		case strings.EqualFold(elem.FullTag(), "item"):
			// adding items to a slice, for future handling without iterating thru the childlist again
			itemList = append(itemList, elem)
		default:
			//log.Debug("unhandled tag: " + elem.FullTag())

		}
		//log.Debug("tag: " + elem.Tag)
	}

	// itemlist handling
	for _, item := range itemList {
		if f.parseItem(item) == false {
			log.Debug("stopping parsing of items for " + f.Shortname)
			break
		}
	}

	log.Debug(fmt.Sprintf("%+v", f))

	return true

}

func (f *Feed) parseItem(elem *etree.Element) (cont bool) {
	// first, get the guid/url, check to see if it exists
	var (
		// todo: null checks
		guid    = elem.FindElement("guid").Text()
		url     = elem.FindElement("enclosure").SelectAttr("url").Value
		urlhash maphash.Hash
	)

	log.Debug("enclosure: %+v, url: %+v", guid, url)
	urlhash.WriteString(url)

	// todo: set limit on number of children to check
	if _, ok := f.itemlist[urlhash.Sum64()]; ok {
		cont = false
	}

	return cont
}
