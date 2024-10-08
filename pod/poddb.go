package pod

import (
	"errors"
	"fmt"
	"path/filepath"

	log "gopod/multilogger"
	"gopod/podutils"

	//"gorm.io/driver/sqlite"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

type (
	PodDBModel       gorm.Model
	direction        bool
	ErrorFeedDeleted struct{}
)

func (e *ErrorFeedDeleted) Error() string { return "feed deleted" }

const ( // because constants should be capitalized, but I don't want to export these..
	cASC  direction = false
	cDESC direction = true

	// shortcut for count of all items
	AllItems = -1

	// current model for database
	currentModel = 3
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

	// in-memory db only used for unit tests.. don't do these checks in those cases
	if path != ":memory:" {
		if exists, err := podutils.FileExists(path); err != nil {
			return nil, err
		} else if exists == false {
			if err := poddb.createNewDb(path); err != nil {
				return nil, err
			}
		}
	}

	if db, err := gImpl.Open(sqlite.Open(poddb.path), &poddb.config); err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)

	} else {
		var result = struct{ ID int }{}

		if res := db.Raw("SELECT ID from poddb_model").Scan(&result); res.Error != nil {
			// handle new database file; this will happen on that error
			return nil, fmt.Errorf("error checking model version: %w", res.Error)

		} else if result.ID != currentModel {
			log.Warnf("database model '%v' doesn't match current model '%v'; attempting upgrade", result.ID, currentModel)
			// future: if more work needs to be done converting the db, should return custom error and handle
			// upgrade/migration in separate launch command.. but for now
			if err := migrateDB(db, result.ID); err != nil {
				return nil, fmt.Errorf("error migrating database: %w", err)
			}
		}
	}

	return &poddb, nil
}

// creates a new database at the specified path with associated tables
func (pdb PodDB) createNewDb(path string) error {
	log.Debug("db file not found; attempting to create new")

	// todo: unit test this shit

	// if this is a new instance, make sure the db path exists; otherwise shit fails
	if err := podutils.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}

	if db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config); err != nil {
		return fmt.Errorf("error opening db: %w", err)

	} else {
		var sqlStr = "CREATE TABLE poddb_model (ID integer); INSERT INTO poddb_model (ID) VALUES (?)"
		if res := db.Exec(sqlStr, currentModel); res.Error != nil {
			return fmt.Errorf("error creating db version: %w", res.Error)
		}

		// in the case of a new db, need to set up tables and such..
		if err := db.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{}, &ItemDBEntry{}, &ItemXmlDBEntry{}, &ImageDBEntry{}); err != nil {
			return err
		}
	}

	return nil
}

type loadOptions struct {
	dontCreate     bool // because default should be to create
	includeXml     bool
	includeDeleted bool
	direction      direction
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
	} else if count > 0 {
		return true, &ErrorFeedDeleted{}
	} else {
		return false, nil
	}
}

// --------------------------------------------------------------------------
func (pdb PodDB) loadFeed(feedEntry *FeedDBEntry, opt loadOptions) error {

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

	// right now, only hash or ID.. always include image data
	var tx = db.Where(&FeedDBEntry{PodDBModel: PodDBModel{ID: feedEntry.ID}, Hash: feedEntry.Hash}).
		Preload("ImageList")
	if opt.includeDeleted {
		tx = tx.Unscoped()
	}
	if opt.includeXml {
		tx = tx.Preload("XmlFeedData")
	}

	var res *gorm.DB
	if opt.dontCreate {
		res = tx.First(feedEntry)
	} else {
		// if this is a new feed, will create a new entry
		res = tx.FirstOrCreate(feedEntry)
	}

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
func (pdb PodDB) loadFeedItems(feedId uint, numItems int, opt loadOptions) ([]*ItemDBEntry, error) {

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
		Order(clause.OrderByColumn{Column: clause.Column{Name: "PubTimeStamp"}, Desc: bool(opt.direction)})
	if opt.includeDeleted {
		tx = tx.Unscoped()
	}
	// if numitems is negative, load everything..
	// we don't care about order; will be transitioned to map anyways
	if numItems > 0 {
		tx = tx.Limit(numItems)
	}
	if opt.includeXml {
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
		// Debug().
		Session(&gorm.Session{FullSaveAssociations: true}).
		Save(feed)

	log.Debugf("rows affected: %v", res.RowsAffected)
	return res.Error
}

// --------------------------------------------------------------------------
func (pdb PodDB) saveItems(itemlist ...*ItemDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if len(itemlist) == 0 {
		return errors.New("item entry list is empty")
	}
	// check for hash & id
	for _, entry := range itemlist {
		if entry == nil {
			return fmt.Errorf("entry is nil")
		} else if entry.FeedId == 0 {
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
func (pdb PodDB) saveImages(imglist ...*ImageDBEntry) error {
	if pdb.path == "" {
		return errors.New("poddb is not initialized; call NewDB() first")
	} else if len(imglist) == 0 {
		return errors.New("item entry list is empty")
	}

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	var res = db.Save(imglist)
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

	// if true {
	// 	db = db.Debug()
	// }

	// get all the items, for deletion
	var itemlist = make([]*ItemDBEntry, 0)
	if res := db.Where(&ItemDBEntry{FeedId: feed.ID}).Order("ID").Find(&itemlist); res.Error != nil {
		return fmt.Errorf("error finding items: %w", res.Error)
	} else if err := pdb.deleteItems(itemlist); err != nil {
		return fmt.Errorf("error deleting items: %w", err)
	}

	// get all images for deletion
	var imglist = make([]*ImageDBEntry, 0)
	if res := db.Where(&ImageDBEntry{FeedId: feed.ID}).Order("ID").Find(&imglist); res.Error != nil {
		return fmt.Errorf("error finding images: %w", res.Error)
	} else if err := pdb.deleteImages(imglist); err != nil {
		return fmt.Errorf("error deleting images: %w", err)
	}

	// delete feed xml
	if feed.XmlId == 0 {
		log.Warn("feed xml is zero; xml entry might not exist")
	} else {
		var xmlentry = podutils.Tern(feed.XmlFeedData == nil, &FeedXmlDBEntry{}, feed.XmlFeedData)
		if res := db.Delete(xmlentry, feed.XmlId); res.Error != nil {
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
	if res := db.Delete(feed, feed.ID); res.Error != nil {
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

	// if true {
	// 	db = db.Debug()
	// }

	// running delete on chuncks
	for _, xmlchunk := range podutils.Chunk(xmlIdList, 100) {
		if res := db.Delete(&ItemXmlDBEntry{}, xmlchunk); res.Error != nil {
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
		if res := db.Delete(&ItemDBEntry{}, idchunk); res.Error != nil {
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

func (pdb PodDB) deleteImages(list []*ImageDBEntry) error {

	db, err := gImpl.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		return fmt.Errorf("error opening db: %w", err)
	}

	// running delete in chuncks
	for _, imageChunk := range podutils.Chunk(list, 100) {
		if res := db.Delete(&imageChunk); res.Error != nil {
			we := fmt.Errorf("failed deleting images: %w", res.Error)
			log.Error(we)
			return we
		} else if res.RowsAffected != int64(len(imageChunk)) {
			log.Warnf("xml delete; expected %v rows, got %v", len(imageChunk), res.RowsAffected)

		} else {
			log.Debug("image delete successful", "rows", res.RowsAffected)
		}

	}

	return nil
}

// --------------------------------------------------------------------------
func (f *FeedDBEntry) AfterFind(tx *gorm.DB) error {
	// log.With("feed", f.DBShortname).Debug("After Find")
	f.imageMap = make(map[string]*ImageDBEntry, len(f.ImageList))
	f.etagMap = make(map[string]*ImageDBEntry, len(f.ImageList))
	for _, im := range f.ImageList {
		f.imageMap[im.Url] = im
		// only for find; checking etag on different urls
		if im.LastModified.ETag != "" {
			f.etagMap[im.LastModified.ETag] = im
		}
	}
	return nil
}

// --------------------------------------------------------------------------
func (f *FeedDBEntry) BeforeSave(tx *gorm.DB) error {
	// log.With("feed", f.DBShortname).Debug("Before Save")
	f.ImageList = make([]*ImageDBEntry, 0, len(f.imageMap))
	for _, im := range f.imageMap {
		f.ImageList = append(f.ImageList, im)
	}
	return nil
}
