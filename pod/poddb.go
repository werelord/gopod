package pod

import (
	"errors"
	"fmt"

	//"gorm.io/driver/sqlite"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"

	log "github.com/sirupsen/logrus"
)

type PodDBModel gorm.Model

type RecordDeletedError struct {
	Msg string
}

func (rde *RecordDeletedError) Error() string {
	return rde.Msg
}

type direction bool

const ( // because constants should be capitalized, but I don't want to export these..
	cASC  direction = false
	cDESC direction = true
)

const allItems = -1

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

		// todo: pull the db version from the db; if changes, do migration
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
func (pdb PodDB) isFeedDeleted(feedEntry *FeedDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if feedEntry == nil {
		return errors.New("feed cannot be nil")
	} else if feedEntry.ID == 0 && feedEntry.Hash == "" {
		return errors.New("hash or ID has not been set")
	}

	//db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	// if err != nil {
	// 	return fmt.Errorf("error opening db: %w", err)
	// }

	//	var tx = db.Unscoped().Where()

	return nil

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
func (pdb PodDB) loadItemXml(itemId uint) (*ItemXmlDBEntry, error) {
	if pdb.path == "" {
		return nil, errors.New("poddb is not initialized; call NewDB() first")
	} else if itemId == 0 {
		return nil, errors.New("feed id cannot be zero")
	}
	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	}
	var xmlEntry ItemXmlDBEntry

	var res = db.Where(&ItemXmlDBEntry{ItemId: itemId}).First(&xmlEntry)
	if res.Error != nil {
		return nil, res.Error
	}
	log.Tracef("xml found, id: %v", xmlEntry.ID)
	return &xmlEntry, nil
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
func (pdb PodDB) saveItems(itemlist []*ItemDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if len(itemlist) == 0 {
		return errors.New("item entry list is empty")
	}
	// check for hash & id
	for _, entry := range itemlist {
		if entry.FeedId == 0 {
			return fmt.Errorf("entry '%v' feed id is zero", entry.Filename)
		} else if entry.Hash == "" {
			return fmt.Errorf("entry '%v' hash is empty", entry.Filename)
		} else if entry.Guid == "" {
			return fmt.Errorf("entry '%v' guid is empty", entry.Filename)
		}
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// save items
	var res = db.Session(&gorm.Session{FullSaveAssociations: true}).Save(itemlist)
	log.Debugf("rows affected: %v", res.RowsAffected)
	return res.Error

}

// --------------------------------------------------------------------------
func (pdb PodDB) deleteItems(list []*ItemDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if len(list) == 0 {
		log.Warn("item list for deletion is empty; doing nothing")
		return nil
	} else {
		for _, item := range list {
			if item.ID == 0 {
				return fmt.Errorf("item missing ID; unable to delete: %v", item)
				// } else if item.XmlData.ID == 0 {
				// 	log.Warnf("attempting to delete item id '%v', but xml ID is 0; will leave orphaned data", item.ID)
				// rather than orphaning xmldata, will do a delete where all match item id
			}
		}
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// attempt with primary key
	for _, item := range list {
		var res = db.Debug().Delete(item)
		if res.Error != nil {
			return res.Error
		} else if res.RowsAffected == 0 {
			// deleting non-existing item will have 0 rows affected, which could indicate an item that doesn't exist..
			return fmt.Errorf("delete returned 0 rows affected; item id '%v' might not exist", item.ID)
			// log.Warn("delete returned 0 rows affected; does item exist?")
		} else {
			// log.Tracef("deleted record; row affedcted: %v", res.RowsAffected)
			// delete all associated xml data, whether its loaded or not
			res = db.Debug().Where(&ItemXmlDBEntry{ItemId: item.ID}).Delete(&ItemXmlDBEntry{})
			if res.Error != nil {
				return res.Error
			} else if res.RowsAffected == 0 {
				log.Warnf("delete xml returned 0 rows affected; xml for item id '%v' might not exist", item.ID)
			} else {
				// log.Tracef("deleted record; row affedcted: %v", res.RowsAffected)
			}

		}
	}

	return nil
}
