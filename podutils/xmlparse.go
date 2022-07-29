package podutils

import (
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/beevik/etree"
	log "github.com/sirupsen/logrus"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

// separating out this to keep feed.go not as verbose

//--------------------------------------------------------------------------
type XChannelData struct {
	AtomLinkSelf   XAtomLink
	NewFeedUrl     string
	Title          string
	Subtitle       string
	PubDate        time.Time
	LastBuildDate  time.Time
	Link           string
	Image          XChannelImage
	ItunesImageUrl string
	ItunesOwner    XItunesOwner
	Author         string
	Copyright      string
	Description    string
	PodcastFunding XPodcastFunding
	PersonList     []XPodcastPersonData
}

type XAtomLink struct {
	// rel = self
	Type  string
	Href  string
	Title string
}

type XChannelImage struct {
	Url   string
	Title string
	Link  string
}

type XItunesOwner struct {
	Name  string
	Email string
}

type XPodcastFunding struct {
	Url  string
	Text string
}

type XItemData struct {
	Title          string
	Pubdate        time.Time
	EpisodeStr     string
	Guid           string
	Link           string
	Author         string
	ItunesAuthor   string
	Imageurl       string
	Description    string
	ItunesSummary  string
	ContentEncoded string
	Enclosure      XEnclosureData
	PersonList     []XPodcastPersonData
}

type XPodcastPersonData struct {
	Email string
	Href  string
	Img   string
	Role  string
	Name  string
}

type XEnclosureData struct {
	Length  uint
	TypeStr string
	Url     string
}

type FeedProcess interface {
	// returns true if current item should be skipped
	// cancel remaining will skip all remaining items (will have no future checks)
	SkipParsingItem(hash string) (skip bool, cancelRemaining bool)
	// returns true if parsing should halt on pub date; parse returns ParseCanceledError on true
	CancelOnPubDate(timestamp time.Time) (cont bool)
	// returns true if parsing should halt on build date; parse returns ParseCanceledError on true
	CancelOnBuildDate(timestamp time.Time) (cont bool)
}

type ParseCanceledError struct {
	cancelReason string
}

func (r ParseCanceledError) Error() string {
	return r.cancelReason
}

//--------------------------------------------------------------------------
func ParseXml(xmldata []byte, fp FeedProcess) (feedData *XChannelData, newItems *orderedmap.OrderedMap[string, XItemData], err error) {

	// fuck the ignore list
	//ignoreList := []string{"atom:link", "lastBuildDate"}

	feedData = &XChannelData{}
	newItems = orderedmap.New[string, XItemData]()

	doc := etree.NewDocument()
	if err = doc.ReadFromBytes(xmldata); err != nil {
		log.Error("failed to read xml document", err)
		return
	}

	root := doc.SelectElement("rss").SelectElement("channel")
	var (
		// dupItemCount  uint = 0
		// maxDupes           = fp.MaxDuplicates()
		skipRemaining bool
	)

	for _, elem := range root.ChildElements() {
		// for now, direct insert, because we're skipping items after a certain dup count
		// or skipping based on pubdate or lastbuilddate
		// this is really pointless because etree loads the entire xml into memory anyways..
		// future: use reflection
		switch {
		case strings.EqualFold(elem.FullTag(), "atom:link"):
			if getAttributeText(elem, "rel") == "self" {
				feedData.AtomLinkSelf.Type = getAttributeText(elem, "type")
				feedData.AtomLinkSelf.Href = getAttributeText(elem, "href")
				feedData.AtomLinkSelf.Title = getAttributeText(elem, "title")
			}
		case strings.EqualFold(elem.FullTag(), "itunes:new-feed-url"):
			feedData.NewFeedUrl = elem.Text()
		case strings.EqualFold(elem.FullTag(), "title"):
			feedData.Title = elem.Text()
		case strings.EqualFold(elem.FullTag(), "itunes:subtitle"):
			feedData.Subtitle = elem.Text()
		case strings.EqualFold(elem.FullTag(), "pubdate"):
			feedData.PubDate = parseDateEntry(elem.Text())
			if feedData.PubDate.IsZero() == false {
				if fp.CancelOnPubDate(feedData.PubDate) == true {
					return feedData, newItems, &ParseCanceledError{"cancelled by CheckPubDate()"}
				}
			}
		case strings.EqualFold(elem.FullTag(), "lastbuilddate"):
			feedData.LastBuildDate = parseDateEntry(elem.Text())
			if feedData.LastBuildDate.IsZero() == false {
				if fp.CancelOnBuildDate(feedData.LastBuildDate) == true {
					return feedData, newItems, &ParseCanceledError{"cancelled by CheckLastBuildDate()"}
				}
			}
		case strings.EqualFold(elem.FullTag(), "link"):
			feedData.Link = elem.Text()
		case strings.EqualFold(elem.FullTag(), "image"):
			feedData.Image.Url = getChildElementText(elem, "url")
			feedData.Image.Title = getChildElementText(elem, "title")
			feedData.Image.Link = getChildElementText(elem, "link")
		case strings.EqualFold(elem.FullTag(), "itunes:image"):
			feedData.ItunesImageUrl = getAttributeText(elem, "href")
		case strings.EqualFold(elem.FullTag(), "itunes:owner"):
			feedData.ItunesOwner.Name = getChildElementText(elem, "itunes:name")
			feedData.ItunesOwner.Email = getChildElementText(elem, "itunes:email")
		case strings.EqualFold(elem.FullTag(), "itunes:author"):
			feedData.Author = elem.Text()
		case strings.EqualFold(elem.FullTag(), "copyright"):
			feedData.Copyright = elem.Text()
		case strings.EqualFold(elem.FullTag(), "description"):
			feedData.Description = elem.Text()
		case strings.EqualFold(elem.FullTag(), "podcast:funding"):
			feedData.PodcastFunding.Url = getAttributeText(elem, "url")
			feedData.PodcastFunding.Text = elem.Text()
		case strings.EqualFold(elem.FullTag(), "podcast:person"):
			personData := XPodcastPersonData{Email: getAttributeText(elem, "email"),
				Href: getAttributeText(elem, "href"),
				Role: getAttributeText(elem, "role"),
				Img:  getAttributeText(elem, "img"),
				Name: elem.Text(),
			}
			feedData.PersonList = append(feedData.PersonList, personData)

		case strings.EqualFold(elem.FullTag(), "item"):

			if skipRemaining == false {
				// check to see if hash exists
				hash, e := calcHash(elem)
				if e != nil {
					log.Error(e)
					continue
				}
				var skipitem = false
				if skipitem, skipRemaining = fp.SkipParsingItem(hash); skipitem == false {
					// not skipping item; automatically add to new item set
					if item, e := parseItemEntry(elem); e == nil {
						newItems.Set(hash, item)
					} else {
						log.Warnf("parse failed; not adding item {'%v' (%v)}: %v", item.Title, hash, e)
					}
				}
			}

		default:
			//log.Debug("unhandled tag: " + elem.FullTag())
		}
	}

	return
}

//--------------------------------------------------------------------------
func getChildElementText(elem *etree.Element, childnode string) string {
	if node := elem.SelectElement(childnode); node != nil {
		return node.Text()
	}
	return ""
}

//--------------------------------------------------------------------------
func getAttributeText(elem *etree.Element, attr string) string {
	if node := elem.SelectAttr(attr); node != nil {
		return node.Value
	}
	return ""
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
			item.Pubdate = parseDateEntry(child.Text())
		case strings.EqualFold(child.FullTag(), "guid"):
			item.Guid = child.Text()
		case strings.EqualFold(child.FullTag(), "link"):
			item.Link = child.Text()
		case strings.EqualFold(child.FullTag(), "author"):
			item.Author = child.Text()
		case strings.EqualFold(child.FullTag(), "itunes:author"):
			item.ItunesAuthor = child.Text()
		case strings.EqualFold(child.FullTag(), "itunes:image"):
			item.Imageurl = getAttributeText(child, "href")
		case strings.EqualFold(child.FullTag(), "description"):
			item.Description = child.Text()
		case strings.EqualFold(child.FullTag(), "content:encoded"):
			item.ContentEncoded = child.Text()
		case strings.EqualFold(child.FullTag(), "itunes:summary"):
			item.ItunesSummary = child.Text()
		case strings.EqualFold(child.FullTag(), "podcast:person"):
			personData := XPodcastPersonData{Email: getAttributeText(child, "email"),
				Href: getAttributeText(child, "href"),
				Role: getAttributeText(child, "role"),
				Img:  getAttributeText(child, "img"),
				Name: child.Text(),
			}
			item.PersonList = append(item.PersonList, personData)
		case strings.EqualFold(child.FullTag(), "media:content"):
			// hijacking person data for media:credit
			for _, media := range child.SelectElements("media:credit") {
				personData := XPodcastPersonData{
					Role: getAttributeText(media, "role"),
					Name: media.Text(),
				}
				item.PersonList = append(item.PersonList, personData)
			}
		case strings.EqualFold(child.FullTag(), "itunes:episode"):
			item.EpisodeStr = child.Text()
		case strings.EqualFold(child.FullTag(), "enclosure"):
			if lenStr := child.SelectAttr("length"); lenStr != nil {
				if l, e := strconv.Atoi(lenStr.Value); e == nil {
					item.Enclosure.Length = uint(l) // shouldn't be any overflow, or negatives, hopefully
				} else {
					log.Error("error in parsing enclosure length:", e)
				}
			}
			if typestr := child.SelectAttr("type"); typestr != nil {
				item.Enclosure.TypeStr = typestr.Value
			}
			// url is required
			if url := child.SelectAttr("url"); url != nil {
				item.Enclosure.Url = url.Value
			} else {
				err = errors.New("missing url")
			}
		}
	}

	//make sure we have an enclosure
	if item.Enclosure.Url == "" {
		err = errors.New("missing enclosure tag")
	}

	//log.Debugf("%+v", item)
	return
}
