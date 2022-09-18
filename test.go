package main

//lint:file-ignore U1000 Ignore all unused methods, one-time or testing use only

import (
	"fmt"
	"gopod/pod"
	"gopod/podutils"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var config = gorm.Config{
	NamingStrategy: schema.NamingStrategy{
		NoLowerCase: true,
	},
}

func convertdb(flist map[string]*pod.Feed) {
	// moving generating hash outside of xmlparse into item, which includes urlparse config value,
	// we need to convert hashes already in the database to the new hash, if it has changed
	// this should be a one-time run thing..

	/*	type FeedXmlCloverDBEntry struct {
			Hash        string
			XmlFeedData podutils.XChannelData
		}

		type ItemDataCloverEntry struct {
			Hash     string
			ItemData struct {
				Filename     string
				Url          string
				Downloaded   bool
				CDFilename   string // content-disposition filename
				PubTimeStamp time.Time
			}
		}

		type ItemXmlCloverEntry struct {
			Hash    string
			ItemXml *podutils.XItemData
		}

		db, err := gorm.Open(sqlite.Open(filepath.Join(defaultworking, "gormtest.db")), &config)
		if err != nil {
			log.Error(err)
			return
		}

		// migrate the schema
		err = db.AutoMigrate(
			&pod.FeedDBEntry{},
			&pod.FeedXmlDBEntry{},
			&pod.ItemDBEntry{},
			&pod.ItemXmlDBEntry{},
			// &podutils.XPersonDataChannel{},
			// &podutils.XPersonDataItem{},
		)
		if err != nil {
			log.Error("migrate error: ", err)
			return
		}

		var f *pod.Feed

		var feedlist = make([]*pod.FeedDBEntry, 0, len(flist))

		//f = flist["twit"]
		for _, f = range flist {
			log.Debug(f.Shortname)

			var exportpath = filepath.Join(defaultworking, ".dbexport")
			bfeedxml, err := podutils.LoadFile(filepath.Join(exportpath, f.Shortname+".json"))
			bitemdata, err2 := podutils.LoadFile(filepath.Join(exportpath, f.Shortname+"_itemdata.json"))
			bitemxml, err3 := podutils.LoadFile(filepath.Join(exportpath, f.Shortname+"_itemxml.json"))

			if err != nil || err2 != nil || err3 != nil {
				log.Errorf("'%v' '%v' '%v'", err, err2, err3)
				return
			}

			// load entries

			log.Debugf("convert json for %v", f.Shortname)
			var cloverfeedxml []FeedXmlCloverDBEntry
			err = json.Unmarshal(bfeedxml, &cloverfeedxml)
			if err != nil {
				log.Errorf("error in xml: %w", err)
				return
			} else if len(cloverfeedxml) != 1 {
				log.Errorf("fucking length: %v", len(cloverfeedxml))
				return
			} else if cloverfeedxml[0].Hash != pod.GenerateHash(*f) {
				log.Errorf("hash mismatch (%v): '%v' '%v'", f.Shortname, pod.GenerateHash(*f), cloverfeedxml[0].Hash)
				return
			}
			f.FeedDBEntry.XmlFeedData.XChannelData = cloverfeedxml[0].XmlFeedData

			var cloveritemdata []ItemDataCloverEntry
			err = json.Unmarshal(bitemdata, &cloveritemdata)
			if err != nil {
				log.Errorf("error in itemdata: %w", err)
				return
			}

			var cloveritemxml []ItemXmlCloverEntry
			err = json.Unmarshal(bitemxml, &cloveritemxml)
			if err != nil {
				log.Errorf("error in itemxml: %w", err)
				return
			}

			if len(cloveritemdata) != len(cloveritemxml) {
				log.Errorf("item lenght wrong, item: %v xml: %v", len(cloveritemdata), len(cloveritemxml))
				return
			}

			var oldHashList = make(map[string]*pod.Item, len(cloveritemdata))

			// assign item to map, using old hashes
			for _, itemdata := range cloveritemdata {
				var item = pod.Item{
					ItemDBEntry: pod.ItemDBEntry{Hash: itemdata.Hash, ItemData: itemdata.ItemData},
				}
				oldHashList[item.Hash] = &item
			}

			// assign xml to assoc item
			for _, itemxml := range cloveritemxml {
				if itemxml.ItemXml == nil {
					log.Errorf("item xml is nil: %v", itemxml.Hash)
					return
				}
				item, exists := oldHashList[itemxml.Hash]
				if exists == false {
					log.Errorf("xml hash doesnt match existing: %v", itemxml)
					return
				}
				// pointer, shouldn't have to reassign
				item.XmlData.XItemData = *itemxml.ItemXml
			}

			// time to check hashes
			var warn = false
			for _, item := range oldHashList {
				newHash, err := pod.CalcHash(item.XmlData.Guid, item.XmlData.Enclosure.Url, f.UrlParse)
				if err != nil {
					log.Errorf("error checking hash: %w", err)
					return
				}
				if newHash != item.Hash {
					//log.Warnf("(%v) hash mismatch (%v); setting item to new hash\noldhash: '%v'\nnewhash: '%v'", f.Shortname, item.Filename, item.Hash, newHash)
					warn = true
					item.Hash = newHash
				}
				// set using the new hash
				f.ItemMap[item.Hash] = item
			}

			if warn {
				log.Warnf("(%v) hash mismatch found (urlparse:%v)", f.Shortname, f.UrlParse)
			}

			feedEntry := f.GetFeedDBEntry()

			feedlist = append(feedlist, feedEntry)

		}

		// insert new
		for _, feed := range feedlist {
			result := db.Create(feed)
			if result.Error != nil {
				log.Errorf("rows: '%v', err: '%v'", result.RowsAffected, result.Error)
			} else {
				log.Debugf("rows: '%v', err: '%v'", result.RowsAffected, result.Error)
			}
		}

		log.Debug("done")
	*/
}

func checkdb(flist map[string]*pod.Feed) {

	db, err := gorm.Open(sqlite.Open(filepath.Join(defaultworking, "gormtest.db")), &config)
	if err != nil {
		log.Error(err)
		return
	}

	var feed = flist["adsp"]
	var res *gorm.DB
	// for _, feed := range flist {

	res = db.Where(&pod.FeedDBEntry{/*Hash: pod.GenerateHash(*feed)*/}).First(&feed.FeedDBEntry)
	if res.Error != nil {
		log.Errorf("rows: '%v', err: '%v'", res.RowsAffected, res.Error)
	} else {
		log.Debugf("rows: '%v', err: '%v'", res.RowsAffected, res.Error)
		log.Debug("%+v", feed)
	}
	var itemListFull = make([]*pod.ItemDBEntry, 0)
	//db.Where(&pod.ItemDBEntry{FeedId: feed.ID}).Order("pub_time_stamp").Find(&itemListFull)
	res = db.Where(&pod.ItemDBEntry{FeedId: feed.ID}).
		//Order("pub_time_stamp").
		Order(clause.OrderByColumn{Column: clause.Column{Name: "PubTimeStamp"}, Desc: true}).
		Limit(6).
		Preload("XmlData").
		Find(&itemListFull)
	if res.Error != nil {
		log.Errorf("rows: '%v', err: '%v'", res.RowsAffected, res.Error)
	} else {
		log.Debugf("rows: '%v', err: '%v'", res.RowsAffected, res.Error)
		log.Debugf("%v", itemListFull)
	}

	//res = db.Where()

	// }

	log.Debug("done")
}

func checkHashes(flist map[string]*pod.Feed) {
	// fuck this shit
	db, err := gorm.Open(sqlite.Open(filepath.Join(defaultworking, "gopod_TEST.db")), &config)
	if err != nil {
		log.Error(err)
		return
	}

	for _, feed := range flist {

		var (
			res      *gorm.DB
			itemlist = make([]*pod.ItemDBEntry, 0)
		)

		res = db.Where(&pod.FeedDBEntry{/*Hash: pod.GenerateHash(*feed)*/}).First(&feed.FeedDBEntry)
		if res.Error != nil {
			log.Error("error in feed query: ", err)
			return
		}

		res = db.Where(&pod.ItemDBEntry{FeedId: feed.ID}).Preload("XmlData").Find(&itemlist)
		if res.Error != nil {
			log.Error("error in itemlist query: ", err)
			return
		}

		log.Debugf("checking hashes for %v, item count: %v", feed.Shortname, len(itemlist))

		for _, item := range itemlist {

			hash, err := feed.CalcItemHash(item.XmlData.Guid, item.XmlData.Enclosure.Url)
			if err != nil {
				log.Error("error in calc hash: ", err)
				continue
			}
			if hash != item.Hash {
				log.Warn("Hash mismatch{%v}:\n\t'%v'\n\t'%v%\n")
			}
		}
	}
	log.Debug("done")
}

func testConflict(flist map[string]*pod.Feed) {

	var (
		//feed  = flist["cultureoftech"]
		ftest = pod.FeedDBEntry{}
	)

	// try to insert existing
	db, err := gorm.Open(sqlite.Open(filepath.Join(defaultworking, "gormtest.db")), &config)
	if err != nil {
		log.Error(err)
		return
	}

	var itemlist = make([]*pod.ItemDBEntry, 0)

	res := db.Debug().Where(&pod.FeedDBEntry{/*Hash: pod.GenerateHash(*feed)*/}).Preload("XmlFeedData").Find(&ftest)
	if res.Error != nil {
		log.Error("error in query: ", res.Error)
		return
	}

	res = db.Debug().Where(&pod.ItemDBEntry{FeedId: ftest.ID}).Find(&itemlist)
	if res.Error != nil {
		log.Error("error in entrylist query: ", res.Error)
		return
	}

	itemlist = append(itemlist, &pod.ItemDBEntry{
		FeedId:   ftest.ID,
		Hash:     "foobar",
		ItemData: pod.ItemData{Filename: "foobar.mp3"}})

	for i, foo := range itemlist {
		foo.Downloaded = false
		foo.CDFilename = fmt.Sprintf("foo-a%v", i)
		fmt.Printf("%v:%+v\n", i, foo)
	}

	var hashconf = clause.OnConflict{
		Columns:   []clause.Column{{Name: "Hash"}},
		UpdateAll: true,
	}

	res = db.Debug().
		Clauses(hashconf).
		Save(itemlist)
	var fn = podutils.Tern(res.Error == nil, log.Debugf, log.Errorf)
	fn("rows: %v, hash conflict: %v\n%v", res.RowsAffected, res.Error)
	log.Debug("done")

	// var pr = func(f pod.FeedDBEntry) string {
	// 	return fmt.Sprintf("id:%v hash:%v, xmlId:%v, xmlTitle:'%v'", f.ID, f.Hash, f.XmlFeedData.ID, f.XmlFeedData.Title)
	// }

	// var onHashConflict = pod.FeedDBEntry{}
	// onHashConflict.Hash = feed.GenerateHash()
	// log.Debugf("\n%v", pr(onHashConflict))
	// res = db.Clauses(hashconf).Create(&onHashConflict)
	// var fn = podutils.Tern(res.Error == nil, log.Debugf, log.Errorf)
	// fn("rows: %v, hash conflict: %v\n%v", res.RowsAffected, res.Error, pr(onHashConflict))

	// var onIdConflict = pod.FeedDBEntry{}
	// onIdConflict.ID = ftest.ID
	// onIdConflict.Hash = "foo"
	// log.Debugf("\n%v", pr(onIdConflict))
	// res = db.Clauses(hashconf).Create(&onIdConflict)
	// fn = podutils.Tern(res.Error == nil, log.Debugf, log.Errorf)
	// fn("rows: %v, id conflict: %v\n%v", res.RowsAffected, res.Error, pr(onIdConflict))

	// onIdConflict.Hash = pod.GenerateHash(*feed)
	// onIdConflict.XmlFeedData.ID = ftest.XmlFeedData.ID
	// onIdConflict.XmlFeedData.Title = "fucker"
	// log.Debugf("\n%v", pr(onIdConflict))
	// res = db.Debug().
	// 	Session(&gorm.Session{FullSaveAssociations: true}).
	// 	Clauses(hashconf).
	// 	Create(&onIdConflict)
	// var fn = podutils.Tern(res.Error == nil, log.Debugf, log.Errorf)
	// fn("rows: %v, id conflict: %v\n%v", res.RowsAffected, res.Error, pr(onIdConflict))

	log.Debug("done")
}
