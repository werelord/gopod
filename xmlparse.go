package main

import (
	"strings"
	"time"

	"crypto/sha1"
	"encoding/base64"
	"net/url"

	"errors"

	"github.com/araddon/dateparse"
	"github.com/beevik/etree"
)

// separating out this to keep feed.go not as verbose

//--------------------------------------------------------------------------
type XChannelData struct {
	title       string
	pubdate     time.Time
	link        string
	image       XChannelImage
	author      string
	description string
}

type XChannelImage struct {
	url   string
	title string
	link  string
}

type XItemData struct {
	Title       string
	pubdate     time.Time
	guid        string
	link        string
	image       string
	description string
	enclosure   XEnclosureData
}

type XEnclosureData struct {
	length  uint
	typeStr string
	url     string
}

//--------------------------------------------------------------------------
func parseXml(xmldata []byte, maxDupes uint, hashExists func(string) bool) (feedData XChannelData, feedlist map[string]XItemData, err error) {
	// fuck the ignore list
	//ignoreList := []string{"atom:link", "lastBuildDate"}

	feedlist = make(map[string]XItemData)

	doc := etree.NewDocument()
	if err = doc.ReadFromBytes(xmldata); err != nil {
		log.Error("failed to read xml document", err)
		return
	}

	root := doc.SelectElement("rss").SelectElement("channel")
	//log.Debug("root element: " + root.Tag)
	var dupItemCount uint = 0

	for _, elem := range root.ChildElements() {

		// for now, direct insert..
		// todo: later use reflection
		switch {
		case strings.EqualFold(elem.FullTag(), "title"):
			feedData.title = elem.Text()
		case strings.EqualFold(elem.FullTag(), "pubdate"):
			t, e := dateparse.ParseAny(elem.Text())
			if e != nil {
				log.Debug("Error parsing timestamp: " + elem.Text())
			} else {
				feedData.pubdate = t
			}
		case strings.EqualFold(elem.FullTag(), "link"):
			feedData.link = elem.Text()
		case strings.EqualFold(elem.FullTag(), "image"):
			if urlnode := elem.SelectElement("url"); urlnode != nil {
				feedData.image.url = elem.SelectElement("url").Text()
			}
			if titlenode := elem.SelectElement("title"); titlenode != nil {
				feedData.image.title = titlenode.Text()
			}
			if linknode := elem.SelectElement("link"); linknode != nil {
				feedData.image.link = linknode.Text()
			}

		case strings.EqualFold(elem.FullTag(), "itunes:author"):
			feedData.author = elem.Text()
		case strings.EqualFold(elem.FullTag(), "description"):
			feedData.description = elem.Text()
		case strings.EqualFold(elem.FullTag(), "item"):
			// adding items to a slice, for future handling without iterating thru the childlist again

			// todo: move hash checking here, append only new entries
			if dupItemCount < maxDupes {

				// check to see if hash exists
				hash, e := calcHash(elem)
				if e != nil {
					log.Error(e)
					continue
				}

				exists := hashExists(hash)
				log.Debugf("hash: '%+v', exists: '%+v'", hash, exists)

				if exists {
					// exists, increment the dup counter
					log.Debug("item exists, incrementing dup counter")
					dupItemCount++
				} else {
					// todo: parse this shit
					if item, e := parseItemEntry(elem); e == nil {
						feedlist[hash] = item
					}
				}
			} else {
				log.Debug("dup counter over limit, skipping...")
			}

			// itemList = append(itemList, elem)
		default:
			//log.Debug("unhandled tag: " + elem.FullTag())
		}
	}

	return
}

//--------------------------------------------------------------------------
func calcHash(elem *etree.Element) (string, error) {
	// first, get the guid/urlstr, check to see if it exists
	var (
		guid   string
		urlstr string
	)

	if guidnode := elem.FindElement("guid"); guidnode != nil {
		guid = guidnode.Text()
	}
	if enclosurenode := elem.FindElement("enclosure"); enclosurenode != nil {
		if urlnode := enclosurenode.SelectAttr("url"); urlnode != nil {
			u, err := url.Parse(urlnode.Value)
			if err != nil {
				log.Error("error in parsing url: ", err)
			}
			u.RawQuery = ""
			u.Fragment = ""

			urlstr = u.String()
		}
	}

	if guid == "" && urlstr == "" {
		return "", errors.New("failed to hash item; guid and url not parsed!")
	}

	// at least url or guid exists, hash the combination
	sha := sha1.New()
	sha.Write([]byte(guid + urlstr))
	hash := base64.URLEncoding.EncodeToString(sha.Sum(nil))
	log.Debugf("guid: '%+v', url: '%+v', hash: '%+v'", guid, urlstr, hash)

	return hash, nil
}

//--------------------------------------------------------------------------
func parseItemEntry(elem *etree.Element) (item XItemData, err error) {

	return
}
