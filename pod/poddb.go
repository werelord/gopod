package pod

import (
	"errors"
	"fmt"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	log "github.com/sirupsen/logrus"
)

type PodDBModel gorm.Model

type direction bool

const ( // because constants should be capitalized, but I don't want to export these..
	cASC  direction = false
	cDESC direction = true
)

type PodDB struct {
	path   string
	config gorm.Config
}

var defaultConfig = gorm.Config{
	NamingStrategy: schema.NamingStrategy{
		NoLowerCase: true,
	},
}

// --------------------------------------------------------------------------
func NewDB(path string) (*PodDB, error) {
	if path == "" {
		return nil, errors.New("db path cannot be empty")
	}

	var poddb = PodDB{path: path, config: defaultConfig}
	if db, err := gImpl.Open(sqlite.Open(poddb.path), &poddb.config); err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	} else if err = db.AutoMigrate(
		&FeedDBEntry{},
		&FeedXmlDBEntry{},
		&ItemDBEntry{},
		&ItemXmlDBEntry{},
	); err != nil {
		return nil, err
	}

	return &poddb, nil
}

// --------------------------------------------------------------------------
func (pdb PodDB) loadDBFeed(feedEntry *FeedDBEntry, loadXml bool) error {

	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if feedEntry == nil {
		return errors.New("feed cannot be nil")
	} else if feedEntry.ID == 0 && feedEntry.Hash == "" {
		return errors.New("hash or ID has not been set")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// right now, only hash or ID
	var tx = db.Where(&FeedDBEntry{PodDBModel: PodDBModel{ID: feedEntry.ID}, Hash: feedEntry.Hash})
	if loadXml {
		tx = tx.Preload("XmlFeedData")
	}

	// if this is a new feed, will create a new entry
	var res = tx.FirstOrCreate(feedEntry)
	if res.Error != nil {
		return res.Error
	}
	// log.Debug("rows found: ", res.RowsAffected)
	// should be done

	return nil
}

// --------------------------------------------------------------------------
func (pdb PodDB) loadDBFeedXml(feedXml *FeedXmlDBEntry) error {

	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if feedXml == nil {
		return errors.New("feedxml cannot be nil")
	} else if feedXml.ID == 0 && feedXml.FeedId == 0 {
		return errors.New("xmlID or feedID cannot be zero")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}
	// only feed id
	var res = db.Where(&FeedXmlDBEntry{
		PodDBModel: PodDBModel{ID: feedXml.ID},
		FeedId:     feedXml.FeedId}).
		First(feedXml)
	if res.Error != nil {
		return res.Error
	}
	// log.Debug("rows found: ", res.RowsAffected)

	return nil
}

// --------------------------------------------------------------------------
func (pdb PodDB) loadFeedItems(feedId uint, numItems int, includeXml bool, dtn direction) ([]*ItemDBEntry, error) {

	if pdb.path == "" {
		return nil, errors.New("poddb is not initialized; call NewDB() first")
	} else if feedId == 0 {
		return nil, errors.New("feed id cannot be zero")
	}
	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	}

	var itemList = make([]*ItemDBEntry, 0)
	// even tho order doesn't matter in the end, as this list is transitioned to map, for testing
	// purposes and to retain consistency (ordered in all possible runs) adding order here
	var tx = db.Where(&ItemDBEntry{FeedId: feedId}).
		Order(clause.OrderByColumn{Column: clause.Column{Name: "PubTimeStamp"}, Desc: bool(dtn)})
	// if numitems is negative, load everything..
	// we don't care about order; will be transitioned to map anyways
	if numItems > 0 {
		tx = tx.Limit(numItems)
	}
	if includeXml {
		tx = tx.Preload("XmlData")
	}

	var res = tx.Find(&itemList)
	if res.Error != nil {
		return itemList, res.Error
	}
	log.Debugf("rows found: %v (itemlist count: %v)", res.RowsAffected, len(itemList))

	return itemList, nil
}

// --------------------------------------------------------------------------
func (pdb PodDB) saveFeed(feed *FeedDBEntry) error {
	// should save feed xml, if set
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if feed == nil {
		return errors.New("feed cannot be nil")
	} else if feed.ID == 0 {
		// this hopefully will never happen; fail if it does
		return errors.New("feed id is zero; make sure feed is created/loaded first")
	} else if feed.Hash == "" {
		return errors.New("hash cannot be empty")
	}

	// if feed.XmlFeedData.ID == 0 {
	// 	log.Warn("xml feed id is zero; will insert new xml entry instead of replacing existing")
	// }

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// save existing
	var res = db.
		Session(&gorm.Session{FullSaveAssociations: true}).
		Save(feed)

	log.Debugf("rows affected: %v", res.RowsAffected)
	return res.Error
}

// --------------------------------------------------------------------------
// func (pdb PodDB) saveItemEntries(entrylist []*ItemDBEntry) error {
// 	// should save item xml, if set

// 	if len(entrylist) == 0 {
// 		log.Warn("save item entries called with empty list; nothing to do")
// 		return nil
// 	}

// 	db, err := gimpl.Open(pdb.path, &pdb.config)
// 	if err != nil {
// 		return fmt.Errorf("error opening db: %w", err)
// 	}
// 	var (
// 		resUpdate, resCreate *gorm.DB
// 		createlist           = make([]*ItemDBEntry, 0, len(entrylist))
// 		updatelist           = make([]*ItemDBEntry, 0, len(entrylist))
// 	)

// 	// looping thru, separating update from create

// 	for _, entry := range entrylist {
// 		// make sure feed id is set
// 		if entry.FeedId == 0 {
// 			return fmt.Errorf("feed id not set in entry: %v", entry)
// 		}

// 		if entry.ID > 0 {
// 			updatelist = append(updatelist, entry)
// 		} else {
// 			// see if it exists, via hash
// 			var count int64
// 			db.Model(&ItemDBEntry{}).Where(&ItemDBEntry{Hash: entry.Hash}).Count(&count)
// 			if count > 0 {
// 				updatelist = append(updatelist, entry)
// 			} else {
// 				createlist = append(createlist, entry)
// 			}
// 		}
// 	}

// 	if len(updatelist) > 0 {
// 		resUpdate = db.Save(updatelist)
// 		if resUpdate.Error != nil {
// 			log.Error("itemlist save error: ", resUpdate.Error)
// 		}
// 	}
// 	if len(createlist) > 0 {
// 		resCreate = db.Create(createlist)
// 		if resCreate.Error != nil {
// 			log.Error("itemlist create error: ", resCreate.Error)
// 		}
// 	}

// 	if resUpdate != nil && resUpdate.Error != nil {
// 		return resUpdate.Error
// 	} else if resCreate != nil && resCreate.Error != nil {
// 		return resCreate.Error
// 	}

// 	return nil
// }
