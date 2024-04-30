package podutils

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/araddon/dateparse"
	"github.com/beevik/etree"

	log "gopod/multilogger"
)

// todo: move to either sax or xml tag parsing before writing tests..

// --------------------------------------------------------------------------
type XChannelData struct {
	AtomLinkSelf   struct{ Type, Href, Title string } `gorm:"embedded;embeddedPrefix:AtomLinkSelf_"`
	NewFeedUrl     string
	Title          string
	Subtitle       string
	PubDate        time.Time
	LastBuildDate  time.Time
	Link           string
	Image          struct{ Url, Title, Link string } `gorm:"embedded;embeddedPrefix:Image_"`
	ItunesImageUrl string
	ItunesOwner    struct{ Name, Email string } `gorm:"embedded;embeddedPrefix:ItunesOwner_"`
	Author         string
	Copyright      string
	Description    string
	PodcastFunding struct{ Url, Text string } `gorm:"embedded;embeddedPrefix:PodcastFunding_"`
	// PersonList     []XPersonDataChannel       `gorm:"foreignKey:XChannelDataID"`
	PersonList []XPodcastPersonData `gorm:"serializer:json"`
}

type XItemData struct {
	Title          string
	Pubdate        time.Time
	SeasonStr      string
	EpisodeStr     string
	Guid           string
	Link           string
	Author         string
	ItunesAuthor   string
	Imageurl       string
	Description    string
	ItunesSummary  string
	ContentEncoded string
	Enclosure      struct {
		Length  uint
		TypeStr string
		Url     string
	} `gorm:"embedded;embeddedPrefix:Enclosure_"`
	// PersonList []XPersonDataItem `gorm:"foreignKey:XItemDataID"`
	PersonList []XPodcastPersonData `gorm:"serializer:json"`
}

// type XPersonDataChannel struct {
// 	XPodcastPersonData
// 	XChannelDataID uint
// }

// type XPersonDataItem struct {
// 	XPodcastPersonData
// 	XItemDataID uint
// }

type XPodcastPersonData struct {
	// gorm.Model
	Email string
	Href  string
	Img   string
	Role  string
	Name  string
}

type FeedProcess interface {
	// returns true if current item should be skipped
	// cancel remaining will skip all remaining items (will have no future checks)
	SkipParsingItem(hash string) (skip bool, cancelRemaining bool)
	// returns true if parsing should halt on pub date; parse returns ParseCanceledError on true
	CancelOnPubDate(timestamp time.Time) (cont bool)
	// returns true if parsing should halt on build date; parse returns ParseCanceledError on true
	CancelOnBuildDate(timestamp time.Time) (cont bool)
	// calculate item hash dependant on feed config
	CalcItemHash(guid string, url string) (string, error)
}

type ParseCanceledError struct {
	cancelReason string
}

func (r ParseCanceledError) Error() string {
	return r.cancelReason
}

func (r ParseCanceledError) Is(target error) bool { return target == ParseCanceledError{} }

type ItemPair struct {
	Hash     string
	ItemData *XItemData
}

// --------------------------------------------------------------------------
func ParseXml(xmldata []byte, fp FeedProcess) (feedData *XChannelData, newItems []ItemPair, err error) {

	if len(xmldata) == 0 {
		err = errors.New("xml data is empty")
		return
	}

	feedData = &XChannelData{PersonList: make([]XPodcastPersonData, 0)}
	newItems = make([]ItemPair, 0)

	doc := etree.NewDocument()
	if err = doc.ReadFromBytes(xmldata); err != nil {
		log.Errorf("failed to read xml document: %v", err)
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
			// feedData.PersonList = append(feedData.PersonList, XPersonDataChannel{XPodcastPersonData: personData})
			feedData.PersonList = append(feedData.PersonList, personData)

		case strings.EqualFold(elem.FullTag(), "item"):

			if skipRemaining == false {
				// check to see if hash exists
				hash, e := calcHash(elem, fp)
				if e != nil {
					log.Errorf("error in calculating hash; skipping item entry: %v", e)
					continue
				}
				var skipitem = false
				if skipitem, skipRemaining = fp.SkipParsingItem(hash); skipitem == false {
					// not skipping xItemData; automatically add to new xItemData set
					if xItemData, e := parseItemEntry(elem); e == nil {
						var newPair = ItemPair{Hash: hash, ItemData: &xItemData}
						newItems = append(newItems, newPair)
					} else {
						log.Warnf("parse failed; not adding item {'%v' (%v)}: %v", xItemData.Title, hash, e)
					}
				}
			}

		default:
			//log.Debug("unhandled tag: " + elem.FullTag())
		}
	}

	return
}

// --------------------------------------------------------------------------
func getChildElementText(elem *etree.Element, childnode string) string {
	if node := elem.SelectElement(childnode); node != nil {
		return node.Text()
	}
	return ""
}

// --------------------------------------------------------------------------
func getAttributeText(elem *etree.Element, attr string) string {
	if node := elem.SelectAttr(attr); node != nil {
		return node.Value
	}
	return ""
}

// --------------------------------------------------------------------------
func calcHash(elem *etree.Element, fp FeedProcess) (string, error) {
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
			urlstr = urlnode.Value
		}
	}

	// by moving the calcHash up to feed, it will handle urlparse factoring as well
	// by doing so, just with the guid and parsed url we can recreate the hash

	return fp.CalcItemHash(guid, urlstr)
}

// --------------------------------------------------------------------------
func parseDateEntry(date string) time.Time {
	t, e := dateparse.ParseAny(date)
	if e != nil {
		log.Debug("Error parsing timestamp: ", date)
	}
	return t
}

// --------------------------------------------------------------------------
func parseItemEntry(elem *etree.Element) (item XItemData, err error) {

	item = XItemData{PersonList: make([]XPodcastPersonData, 0)}

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
			// item.PersonList = append(item.PersonList, XPersonDataItem{XPodcastPersonData: personData})
			item.PersonList = append(item.PersonList, personData)
		case strings.EqualFold(child.FullTag(), "media:content"):
			// hijacking person data for media:credit
			for _, media := range child.SelectElements("media:credit") {
				personData := XPodcastPersonData{
					Role: getAttributeText(media, "role"),
					Name: media.Text(),
				}
				// item.PersonList = append(item.PersonList, XPersonDataItem{XPodcastPersonData: personData})
				item.PersonList = append(item.PersonList, personData)
			}
		case strings.EqualFold(child.FullTag(), "itunes:season"):
			item.SeasonStr = child.Text()
		case strings.EqualFold(child.FullTag(), "itunes:episode"):
			item.EpisodeStr = child.Text()
		case strings.EqualFold(child.FullTag(), "enclosure"):
			if lenStr := child.SelectAttr("length"); lenStr != nil {
				if lenStr.Value == "" {
					log.Warn("length attribute is empty")
				} else if l, e := strconv.Atoi(lenStr.Value); e == nil {
					item.Enclosure.Length = uint(l) // shouldn't be any overflow, or negatives, hopefully
				} else {
					log.Errorf("error in parsing enclosure length: %v", e)
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
