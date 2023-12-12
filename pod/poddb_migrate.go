package pod

import (
	"fmt"
	log "gopod/multilogger"
)

func migrateDB(db gormDBInterface, oldVersion int) error {
	if (oldVersion < currentModel) == false {
		return fmt.Errorf("unable to migrate; old version not less than current")
	}

	if oldVersion <= 1 {
		if err := migrateV1toV2(db); err != nil {
			return err
		}
	}
	// if oldVersion <= 2 {
	// future upgrades
	// }

	// finally, make sure current model is set
	var sqlStr = "UPDATE poddb_model SET (ID) = (?)"
		if res := db.Exec(sqlStr, currentModel); res.Error != nil {
			return fmt.Errorf("error setting db version: %w", res.Error)
		}

	return nil
}

func migrateV1toV2(db gormDBInterface) error {
	log.Info("upgrading from v1 to v2")
	// v1 to v2 introduced image table, refs in feed and item
	if err := db.AutoMigrate(&FeedDBEntry{}, &ItemDBEntry{}, &ImageDBEntry{}); err != nil {
		return err
	}
	return nil
}
