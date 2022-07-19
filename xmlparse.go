package main

import (
	"strconv"
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
	Title       string
	LastPubDate time.Time
	Link        string
	Image       XChannelImage
	Author      string
	Description string
}

type XChannelImage struct {
	Url   string
	Title string
	Link  string
}

type XItemData struct {
	Title       string
	pubdate     time.Time
	guid        string
	link        string
	imageurl    string
	description string
	enclosure   XEnclosureData
}

type XEnclosureData struct {
	length  uint
	typeStr string
	url     string
}

type feedProcess interface {
	maxDuplicates() uint
	exists(hash string) bool
	checkTimestamp(timestamp time.Time) bool
}

//--------------------------------------------------------------------------
func parseXml(xmldata []byte, fp feedProcess) (feedData XChannelData, newItems map[string]XItemData, err error) {
	// fuck the ignore list
	//ignoreList := []string{"atom:link", "lastBuildDate"}

	newItems = make(map[string]XItemData)

	doc := etree.NewDocument()
	if err = doc.ReadFromBytes(xmldata); err != nil {
		log.Error("failed to read xml document", err)
		return
	}

	root := doc.SelectElement("rss").SelectElement("channel")
	//log.Debug("root element: " + root.Tag)
	var (
		dupItemCount uint = 0
		maxDupes          = fp.maxDuplicates()
	)

	for _, elem := range root.ChildElements() {

		// for now, direct insert..
		// todo: later use reflection
		switch {
		case strings.EqualFold(elem.FullTag(), "title"):
			feedData.Title = elem.Text()
		case strings.EqualFold(elem.FullTag(), "pubdate"):
			// todo: shortcut, check pub date
			// we're assuming this would be early in the process
			feedData.LastPubDate = parseDateEntry(elem.Text())
		case strings.EqualFold(elem.FullTag(), "link"):
			feedData.Link = elem.Text()
		case strings.EqualFold(elem.FullTag(), "image"):
			if urlnode := elem.SelectElement("url"); urlnode != nil {
				feedData.Image.Url = elem.SelectElement("url").Text()
			}
			if titlenode := elem.SelectElement("title"); titlenode != nil {
				feedData.Image.Title = titlenode.Text()
			}
			if linknode := elem.SelectElement("link"); linknode != nil {
				feedData.Image.Link = linknode.Text()
			}

		case strings.EqualFold(elem.FullTag(), "itunes:author"):
			feedData.Author = elem.Text()
		case strings.EqualFold(elem.FullTag(), "description"):
			feedData.Description = elem.Text()
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

				exists := fp.exists(hash)
				//log.Debugf("hash: '%+v', exists: '%+v'", hash, exists)

				if exists {
					// exists, increment the dup counter
					log.Debug("item exists, incrementing dup counter")
					dupItemCount++
				} else {
					// todo: parse this shit
					if item, e := parseItemEntry(elem); e == nil {
						newItems[hash] = item
					} else {
						log.Errorf("not adding item '", hash, "': ", e)
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
		return "", errors.New("failed to hash item; guid and url not parsed")
	}

	// at least url or guid exists, hash the combination
	sha := sha1.New()
	sha.Write([]byte(guid + urlstr))
	hash := base64.URLEncoding.EncodeToString(sha.Sum(nil))
	//log.Debugf("guid: '%+v', url: '%+v', hash: '%+v'", guid, urlstr, hash)

	return hash, nil
}

//--------------------------------------------------------------------------
func parseDateEntry(date string) time.Time {
	t, e := dateparse.ParseAny(date)
	if e != nil {
		log.Debug("Error parsing timestamp: ", date)
	}
	return t

}

//--------------------------------------------------------------------------
func parseItemEntry(elem *etree.Element) (item XItemData, err error) {

	for _, child := range elem.ChildElements() {
		switch {
		case strings.EqualFold(child.FullTag(), "title"):
			item.Title = child.Text()
		case strings.EqualFold(child.FullTag(), "pubdate"):
			item.pubdate = parseDateEntry(child.Text())
		case strings.EqualFold(child.FullTag(), "guid"):
			item.guid = child.Text()
		case strings.EqualFold(child.FullTag(), "link"):
			item.link = child.Text()
		case strings.EqualFold(child.FullTag(), "itunes:image"):
			if href := child.SelectAttr("href"); href != nil {
				item.imageurl = href.Value
			}
		case strings.EqualFold(child.FullTag(), "description"):
			item.description = child.Text()
		case strings.EqualFold(child.FullTag(), "enclosure"):
			if lenStr := child.SelectAttr("length"); lenStr != nil {
				if l, e := strconv.Atoi(lenStr.Value); e == nil {
					item.enclosure.length = uint(l) // shouldn't be any overflow, or negatives, hopefully
				} else {
					log.Error("error in parsing enclosure length:", e)
				}
			}
			if typestr := child.SelectAttr("type"); typestr != nil {
				item.enclosure.typeStr = typestr.Value
			}
			// url is required
			if url := child.SelectAttr("url"); url != nil {
				item.enclosure.url = url.Value
			} else {
				err = errors.New("missing url")
			}
		}
	}
	//log.Debugf("%+v", item)
	return
}
