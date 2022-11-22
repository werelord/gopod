package pod
/*
// future: basic structure for migration tools
import (
	"gopod/podutils"
	"path/filepath"

	"github.com/glebarez/sqlite"
	log "github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type oldItemXmlDBEntry struct {
	PodDBModel
	ItemId             uint
	podutils.XItemData `gorm:"embedded"`
}

type oldItemDBEntry struct {
	PodDBModel
	Hash   string `gorm:"uniqueIndex"`
	FeedId uint

	ItemData `gorm:"embedded"`
	XmlData  oldItemXmlDBEntry `gorm:"foreignKey:ItemId"`
}

type oldFeedXmlDBEntry struct {
	PodDBModel
	FeedId                uint
	podutils.XChannelData `gorm:"embedded"`
}

type oldFeedDBEntry struct {
	// anything that needs to be persisted between runs, go here
	PodDBModel
	Hash         string `gorm:"uniqueIndex"`
	DBShortname  string // just for db browsing
	EpisodeCount int
	XmlFeedData  oldFeedXmlDBEntry `gorm:"foreignKey:FeedId"`
	ItemList     []*oldItemDBEntry `gorm:"foreignKey:FeedId"`
}

func convertItemXml(old oldItemXmlDBEntry) *ItemXmlDBEntry {
	var itemxml = ItemXmlDBEntry{
		PodDBModel: PodDBModel{
			ID:        old.ID,
			CreatedAt: old.CreatedAt,
			UpdatedAt: old.UpdatedAt,
			DeletedAt: old.DeletedAt,
		},
		XItemData: old.XItemData,
	}
	return &itemxml
}

func convertItem(old *oldItemDBEntry) *ItemDBEntry {
	var item = ItemDBEntry{
		PodDBModel: PodDBModel{
			ID:        old.ID,
			CreatedAt: old.CreatedAt,
			UpdatedAt: old.UpdatedAt,
			DeletedAt: old.DeletedAt,
		},
		Hash:     old.Hash,
		FeedId:   old.FeedId,
		ItemData: old.ItemData,
		XmlData:  convertItemXml(old.XmlData),
	}
	item.XmlId = item.XmlData.ID
	return &item
}
func convertFeedXml(old oldFeedXmlDBEntry) *FeedXmlDBEntry {
	var xml = FeedXmlDBEntry{
		PodDBModel: PodDBModel{
			ID:        old.ID,
			CreatedAt: old.CreatedAt,
			UpdatedAt: old.UpdatedAt,
			DeletedAt: old.DeletedAt,
		},
		XChannelData: old.XChannelData,
	}
	return &xml
}

func convertFeed(old *oldFeedDBEntry) *FeedDBEntry {
	var feed = FeedDBEntry{
		PodDBModel: PodDBModel{
			ID:        old.ID,
			CreatedAt: old.CreatedAt,
			UpdatedAt: old.UpdatedAt,
			DeletedAt: old.DeletedAt,
		},
		Hash:         old.Hash,
		DBShortname:  old.DBShortname,
		EpisodeCount: old.EpisodeCount,
		XmlFeedData:  convertFeedXml(old.XmlFeedData),
	}
	feed.XmlId = feed.XmlFeedData.ID

	return &feed
}

func (pdb PodDB) getMigrationPath() string {
	return filepath.Join(filepath.Dir(pdb.path), "poddb.migrate.db")
}

func (pdb PodDB) DoMigrateFeed() {

	if err := pdb.autoMigrateNew(); err != nil {
		log.Error("error automigrating: ", err)
		return
	}

	olddb, err := gorm.Open(sqlite.Open(pdb.path), &pdb.config)
	if err != nil {
		log.Errorf("error opening db: %v", err)
		return
		// } else {
		// debugging
		//olddb = olddb.Debug()
	}

	if true { // convert feeds
		var oldfeedList = make([]*oldFeedDBEntry, 0)
		if res := olddb.Unscoped().Table("FeedDBEntries").Order("ID").Find(&oldfeedList); res.Error != nil {
			log.Error("error getting feeds: ", res.Error)
			return
		} else {
			log.Debug("getting feeds, rows: ", res.RowsAffected)
		}

		var newfeedlist = make([]*FeedDBEntry, 0, len(oldfeedList))
		for _, oldfeed := range oldfeedList {

			log.Debug(oldfeed.DBShortname)
			// get the xml
			if res := olddb.Unscoped().Table("FeedXmlDBEntries").Where("FeedId = ?", oldfeed.ID).
				Last(&oldfeed.XmlFeedData); res.Error != nil {
				log.Error("error getting feed xml: ", res.Error)
				continue
				// } else {
				// log.Debug("getting xml, rows: ", res.RowsAffected)
			}

			newfeedlist = append(newfeedlist, convertFeed(oldfeed))

		}

		if err := pdb.saveFeedMigration(newfeedlist); err != nil {
			log.Error("error in saving feeds: ", err)
		}
	}

	var oldItemlist []*oldItemDBEntry
	res := olddb.Unscoped().Table("ItemDBEntries").Order("ID").FindInBatches(&oldItemlist, 100,
		func(tx *gorm.DB, batch int) error {

			log.Debugf("batch %v", batch)
			var newitemlist = make([]*ItemDBEntry, 0, len(oldItemlist))
			for _, oldItem := range oldItemlist {
				if res := olddb.Unscoped().Table("ItemXmlDBEntries").Where("ItemId = ?", oldItem.ID).
					Last(&oldItem.XmlData); res.Error != nil {
					log.Error("error getting feed xml: ", res.Error)
					continue
				}

				newitemlist = append(newitemlist, convertItem(oldItem))
			}

			if err := pdb.saveItemMigration(newitemlist); err != nil {
				log.Error("error in saving itemlist: ", err)
				return err
			}
			return nil
		})
	if res.Error != nil {
		log.Error("error in processing items: ", res.Error)
		return
	} else {
		log.Debug("item rows handled: ", res.RowsAffected)
	}

}

func (pdb PodDB) autoMigrateNew() error {

	newdb, err := gorm.Open(sqlite.Open(pdb.getMigrationPath()), &pdb.config)
	if err != nil {
		log.Errorf("error opening db: %v", err)
		return err
	} else {
		newdb = newdb.Debug()
		type Result struct {
			ID int
		}

		var result Result
		if res := newdb.Raw("SELECT ID from poddb_model").Scan(&result); (res.Error != nil) || (result.ID != currentModel) {

			if err := newdb.AutoMigrate(
				&FeedDBEntry{},
				&FeedXmlDBEntry{},
				&ItemDBEntry{},
				&ItemXmlDBEntry{}); err != nil {

				log.Errorf("error in automigrate: %v", err)
				return err
			}

			if res := newdb.Exec("CREATE TABLE poddb_model (ID integer)"); res.Error != nil {
				log.Error("error in saving model: ", res.Error)
				return res.Error
			} else if res := newdb.Exec("INSERT INTO poddb_model (ID) VALUES (?)", currentModel); res.Error != nil {
				log.Error("error in saving model: ", res.Error)
				return res.Error
			}
		} else {
			log.Debug("Current model: ", result.ID)
		}
	}
	return nil
}

func (pdb PodDB) saveFeedMigration(feedlist []*FeedDBEntry) error {

	newdb, err := gorm.Open(sqlite.Open(pdb.getMigrationPath()), &pdb.config)
	if err != nil {
		log.Errorf("error opening db: %v", err)
		return err
		// } else {
		//newdb = newdb.Debug()
	}

	if res := newdb.Session(&gorm.Session{FullSaveAssociations: true}).Save(feedlist); res.Error != nil {
		log.Error("error saving feed: ", res.Error)
		return res.Error
	} else {
		log.Debugf("saving feed, rows affected: %v", res.RowsAffected)
		// if res := newdb.Model(&FeedXmlDBEntry{}).Updates(&xmllist); res.Error != nil {
		// 	log.Error("error saving feed xml: ", res.Error)
		// 	return res.Error
		// } else {
		// 	log.Debugf("saving feed xml, rows affected: %v", res.RowsAffected)
		// }
	}

	return nil
}

func (pdb PodDB) saveItemMigration(itemlist []*ItemDBEntry) error {

	newdb, err := gorm.Open(sqlite.Open(pdb.getMigrationPath()), &pdb.config)
	if err != nil {
		log.Errorf("error opening db: %v", err)
		return err
		// } else {
		//newdb = newdb.Debug()
	}

	if res := newdb.Session(&gorm.Session{FullSaveAssociations: true}).Save(&itemlist); res.Error != nil {
		log.Error("error saving items: ", res.Error)
		return res.Error
	} else {
		log.Debugf("saving items, rows affected: %v", res.RowsAffected)
		// if res := newdb.Save(&xmllist); res.Error != nil {
		// 	log.Error("error saving item xml: ", res.Error)
		// 	return res.Error
		// } else {
		// 	log.Debugf("saving item xml, rows affected: %v", res.RowsAffected)
		// }
	}

	return nil
}
*/