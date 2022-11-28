package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"

	//"gorm.io/driver/sqlite"
	"github.com/glebarez/sqlite"
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

const allItems = -1

type PodDB struct {
	path   string
	config gorm.Config
}

const currentModel = 1

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

	} else {
		var result = struct{ ID int }{}

		if res := db.Raw("SELECT ID from poddb_model").Scan(&result); res.Error != nil {
			// handle new database file; this will happen on that error
			log.Debug("finding model id failed; attempting to create new model")
			var sqlStr = "CREATE TABLE poddb_model (ID integer); INSERT INTO poddb_model (ID) VALUES (?)"
			if res := db.Exec(sqlStr, currentModel); res.Error != nil {
				return nil, fmt.Errorf("error finding db version: %w", res.Error)
			}
		} else if result.ID != currentModel {
			// future: custom error for calling migration methods
			return nil, fmt.Errorf("model doesn't match current; migrate needs to happen")
		}
	}

	return &poddb, nil
}

// --------------------------------------------------------------------------
func (pdb PodDB) isFeedDeleted(hash string) (bool, error) {
	if pdb.path == "" {
		return false, errors.New("poddb is not initialized; call NewDB() first")
	} else if hash == "" {
		return false, errors.New("hash cannot be empty")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return false, fmt.Errorf("error opening db: %w", err)
	}

	var count int64
	var res = db. /*Debug().*/ Unscoped().Model(&FeedDBEntry{}).Where("Hash = ? AND DeletedAt not NULL", hash).Count(&count)
	if res.Error != nil {
		return false, res.Error
	}

	return count > 0, nil

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
func (pdb PodDB) loadDBFeedXml(xmlId uint) (*FeedXmlDBEntry, error) {

	if pdb.path == "" {
		return nil, errors.New("poddb is not initialized; call NewDB() first")
	} else if xmlId == 0 {
		return nil, errors.New("xml ID cannot be zero")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	}

	var xmlEntry FeedXmlDBEntry
	if res := db.Where(&FeedXmlDBEntry{PodDBModel: PodDBModel{ID: xmlId}}).First(&xmlEntry); res.Error != nil {
		return nil, res.Error
	}
	// log.Debug("rows found: ", res.RowsAffected)

	return &xmlEntry, nil
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
func (pdb PodDB) loadItemXml(xmlId uint) (*ItemXmlDBEntry, error) {
	if pdb.path == "" {
		return nil, errors.New("poddb is not initialized; call NewDB() first")
	} else if xmlId == 0 {
		return nil, errors.New("xml id cannot be zero")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	}

	var xmlEntry ItemXmlDBEntry
	if res := db.Where(&ItemXmlDBEntry{PodDBModel: PodDBModel{ID: xmlId}}).First(&xmlEntry); res.Error != nil {
		return nil, res.Error
	}
	//log.Tracef("xml found, id: %v", xmlEntry.ID)
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
func (pdb PodDB) deleteFeed(feed *FeedDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if feed == nil {
		return errors.New("feed cannot be nil")
	} else if feed.ID == 0 {
		// this hopefully will never happen; fail if it does
		return errors.New("feed id cannot be zero; make sure it is loaded first")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// get all the items, for deletion
	var itemlist = make([]*ItemDBEntry, 0)
	if res := db. /*Debug().*/ Where(&ItemDBEntry{FeedId: feed.ID}).Order("ID").Find(&itemlist); res.Error != nil {
		return fmt.Errorf("error finding items: %w", res.Error)
	} else if err := pdb.deleteItems(itemlist); err != nil {
		return fmt.Errorf("error deleting items: %w", err)
	}

	// delete feed xml
	if feed.XmlId == 0 {
		log.Warnf("feed xml is zero; xml entry might not exist")
	} else {
		var xmlentry = podutils.Tern(feed.XmlFeedData == nil, &FeedXmlDBEntry{}, feed.XmlFeedData)
		if res := db. /*Debug().*/ Delete(xmlentry, feed.XmlId); res.Error != nil {
			err := fmt.Errorf("failed deleting feed xml: %w", res.Error)
			log.Error(err)
			return err
		} else if res.RowsAffected != 1 {
			log.Warnf("xml delete; expected 1 row, got %v", res.RowsAffected)
		} else {
			log.Debugf("feed xml delete, rows: %v", res.RowsAffected)
		}
	}

	// delete feed
	if res := db. /*Debug().*/ Delete(feed, feed.ID); res.Error != nil {
		err := fmt.Errorf("failed deleting feed: %w", res.Error)
		log.Error(err)
		return err
	} else if res.RowsAffected != 1 {
		log.Warnf("feed delete; expected 1 row, got %v", res.RowsAffected)
	} else {
		log.Debugf("feed delete, rows: %v", res.RowsAffected)
	}

	return nil
}

// --------------------------------------------------------------------------
func (pdb PodDB) deleteItems(list []*ItemDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if len(list) == 0 {
		log.Warn("item list for deletion is empty; doing nothing")
		return nil
	}

	// collect item ids, xml ids
	var (
		itemIdList = make([]uint, 0, len(list))
		xmlIdList  = make([]uint, 0, len(list))
	)

	for _, item := range list {
		if item.ID == 0 {
			return fmt.Errorf("item missing ID; unable to delete: %v", item)
		} else {
			itemIdList = append(itemIdList, item.ID)
			if item.XmlId == 0 {
				log.Warnf("xml id is missing; unable to delete: %v", item)
			} else {
				xmlIdList = append(xmlIdList, item.XmlId)
			}
		}
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// running delete on chuncks
	for _, xmlchunk := range podutils.Chunk(xmlIdList, 100) {
		if res := db. /*Debug().*/ Delete(&ItemXmlDBEntry{}, xmlchunk); res.Error != nil {
			err := fmt.Errorf("failed deleting item xml: %w", res.Error)
			log.Error(err)
			return err
		} else if res.RowsAffected != int64(len(xmlchunk)) {
			log.Warnf("xml delete; expected %v rows, got %v", len(xmlchunk), res.RowsAffected)
		} else {
			log.Debugf("xml delete, rows: %v (expect %v)", res.RowsAffected, len(xmlchunk))
		}
	}

	for _, idchunk := range podutils.Chunk(itemIdList, 100) {
		if res := db. /*Debug().*/ Delete(&ItemDBEntry{}, idchunk); res.Error != nil {
			err := fmt.Errorf("failed deleting item: %w", res.Error)
			log.Error(err)
			return err
		} else if res.RowsAffected != int64(len(idchunk)) {
			log.Warnf("item delete; expected %v rows, got %v", len(idchunk), res.RowsAffected)
		} else {
			log.Debugf("item delete, rows: %v (expect %v)", res.RowsAffected, len(idchunk))
		}
	}

	// future: deleting off list of ids as done above does not modify passed in list's DeletedAt field
	// for now, we're not going to use that outside of this function (tests will reflect that)
	// but if that changes, will need to grab the DeletedAt value from the value passed in to Delete
	// and propegate that to all items listed above

	return nil
}
