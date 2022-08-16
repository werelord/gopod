package main

import (
	"gopod/pod"
	"gopod/poddb"
	"path/filepath"

	"github.com/ostafen/clover/v2"
	log "github.com/sirupsen/logrus"
)

func migratedbs(flist map[string]*pod.Feed, dbpath string) {
	// explicit drop collections
	if true {
		dropCollections(dbpath)
	}

	for _, f := range flist {

		//f := flist["twit"]

		fexport := f.CreateExport()
		itemDataExport := f.CreateItemDataExport()
		itemXmlExport := f.CreateItemXmlExport()

		db, err := poddb.NewDB(f.Shortname)
		if err != nil {
			log.Error("failed: ", err)
			return
		}

		// todo: move these try methods into unit tests
		//	tryFetch(db)
		// tryItemFetch(db)
		// tryItemFetchMulti(db)
		//tryItemFetchMultiQuery(db)

		//return

		// log.Debugf("feedExport:%v\nitemDataExport:%+v\nitemXmlExport:%+v\n", *fexport, itemDataExport, itemXmlExport)

		log.Debugf("Inserting entry: %+v", fexport.Hash)
		id, err := db.FeedCollection().InsertyByEntry(fexport)
		if err != nil {
			log.Error("error: ", err)
		}
		log.Debugf("id returned: %v", id)

		// todo: something to not make this fucker thrash
		for _, entry := range itemDataExport {
			log.Debugf("inserting entry: %+v", entry.Hash)
			id, err := db.ItemDataCollection().InsertyByEntry(entry)
			if err != nil {
				log.Error("error: ", err)
			}
			log.Debugf("id returned: %v", id)
		}

		for _, entry := range itemXmlExport {
			log.Debugf("inserting entry: %+v", entry.Hash)
			id, err := db.ItemXmlCollection().InsertyByEntry(entry)
			if err != nil {
				log.Error("error: ", err)
			}
			log.Debugf("id returned: %v", id)
		}

	}

	poddb.DumpCollections(filepath.Join(defaultworking, ".dbexport"))
}

func tryFetch(db *poddb.PodDB) {
	var (
		err        error
		id         string
		emptyFeed  pod.FeedDBEntry
		feedById   pod.FeedDBEntry
		feedByHash pod.FeedDBEntry
		//itemlist  pod.ItemListDBEntry

		arm = struct {
			Name string
			Age  int
		}{}

		leg = struct {
			Foobar string
		}{}
	)

	// failure
	if id, err = db.FeedCollection().FetchByEntry(&emptyFeed); err != nil {
		log.Errorf("error fetching by id: %v (id='%v')", err, id)
	}

	// failure
	if id, err = db.FeedCollection().FetchById("foobar", &emptyFeed); err != nil {
		log.Errorf("error fetching by id: %v (id='%v')", err, id)
	}

	// failure
	if id, err = db.FeedCollection().FetchByEntry(&arm); err != nil {
		log.Errorf("error fetching by entry: %v (id='%v')", err, id)
	}

	// failure
	if id, err = db.FeedCollection().FetchByEntry(&leg); err != nil {
		log.Errorf("error fetching by entry: %v (id='%v')", err, id)
	}

	// feed by id
	if id, err = db.FeedCollection().FetchById("fa33a9b2-18c2-4d1f-9114-5756a3e77403", &feedById); err != nil {
		log.Error("error fetching by id: ", err)
	}
	log.Debugf("id: %v, feed retrieved: %v(%v)", id, feedById.Hash, feedById.XmlFeedData.Title)

	// feed by hash
	feedByHash.Hash = "L6LI9eoaNnrQdPgShEHI3I2uOjI="
	if id, err = db.FeedCollection().FetchByEntry(&feedByHash); err != nil {
		log.Error("error fetching by entry: ", err)
	}
	log.Debugf("id: %v, feed retrieved: %v(%v)", id, feedByHash.Hash, feedByHash.XmlFeedData.Title)
}

func tryItemFetch(db *poddb.PodDB) {
	var (
		err        error
		id         string
		emptyItem  pod.ItemXmlDBEntry_Clover
		itemById   pod.ItemXmlDBEntry_Clover
		itemByHash pod.ItemXmlDBEntry_Clover
		//itemlist  pod.ItemListDBEntry

		// allItems []*pod.ItemDBEntry
		// queryItems []*pod.ItemDBEntry
		// limitItems []*pod.ItemDBEntry

		arm = struct {
			Name string
			Age  int
		}{}

		leg = struct {
			Foobar string
		}{}
	)

	// failure
	if id, err = db.ItemXmlCollection().FetchByEntry(&emptyItem); err != nil {
		log.Errorf("error fetching by id: %v (id='%v')", err, id)
	}

	// failure
	if id, err = db.ItemXmlCollection().FetchById("foobar", &emptyItem); err != nil {
		log.Errorf("error fetching by id: %v (id='%v')", err, id)
	}

	// failure
	if id, err = db.ItemXmlCollection().FetchByEntry(&arm); err != nil {
		log.Errorf("error fetching by entry: %v (id='%v')", err, id)
	}

	// failure
	if id, err = db.ItemXmlCollection().FetchByEntry(&leg); err != nil {
		log.Errorf("error fetching by entry: %v (id='%v')", err, id)
	}

	// item by id
	if id, err = db.ItemXmlCollection().FetchById("524752b2-4b55-424a-9911-a2db808904c5", &itemById); err != nil {
		log.Error("error fetching by id: ", err)
	}
	log.Debugf("\nid: %v, feed retrieved: %v(%v)", id, itemById.Hash, itemById.ItemXml.Enclosure.Url)

	// item by hash
	itemByHash.Hash = "CjrSMaant6OP28juPs7G07B-kV8="
	if id, err = db.ItemXmlCollection().FetchByEntry(&itemByHash); err != nil {
		log.Error("error fetching by entry: ", err)
	}
	log.Debugf("\nid: %v, feed retrieved: %v(%v)", id, itemByHash.Hash, itemByHash.ItemXml.Enclosure.Url)
}

func tryItemFetchMulti(db *poddb.PodDB) {
	var (
		err error
		//id         string

		entryList []poddb.DBEntry

		// allItems []*pod.ItemDBEntry
		// queryItems []*pod.ItemDBEntry
		// limitItems []*pod.ItemDBEntry
	)

	fn := func() any {
		return &pod.ItemXmlDBEntry_Clover{}
	}

	entryList, err = db.ItemXmlCollection().FetchAll(fn)
	if err != nil {
		log.Error("error in fetchall: ", err)
	}
	log.Debug("count: ", len(entryList))
	for _, e := range entryList {

		var newItem = e.Entry.(*pod.ItemXmlDBEntry_Clover)

		log.Debugf("\nid:'%v' entry:'%v'(%v)\n", e.ID, newItem.Hash, newItem.ItemXml.Pubdate)
	}
	log.Debug("done")
}

func tryItemFetchMultiQuery(db *poddb.PodDB) {
	var (
		err error
		//id         string

		entryList []poddb.DBEntry

		// allItems []*pod.ItemDBEntry
		// queryItems []*pod.ItemDBEntry
		// limitItems []*pod.ItemDBEntry
	)

	fn := func() any {
		return &pod.ItemXmlDBEntry_Clover{}
	}

	var opt = clover.SortOption{Field: "XmlItemData.Pubdate", Direction: -1}
	var query = db.ItemXmlCollection().NewQuery().Sort(opt).Limit(3)
	//var query = db.ItemCollection().NewQuery().Where(clover.Field("XmlItemData.EpisodeStr").Eq("885"))

	entryList, err = db.ItemXmlCollection().FetchAllWithQuery(fn, query)
	if err != nil {
		log.Error("error in fetchall: ", err)
	}
	log.Debug("count: ", len(entryList))
	for _, e := range entryList {

		var newItem = e.Entry.(*pod.ItemXmlDBEntry_Clover)

		log.Debugf("\nid:'%v' entry:'%v'(%v)\n", e.ID, newItem.Hash, newItem.ItemXml.Pubdate)
	}
	log.Debug("done")
}

func dropCollections(dbpath string) {
	db, err := clover.Open(dbpath)
	if err != nil {
		log.Error("failed opening db: ", err)
		return
	}
	defer db.Close()

	collList, _ := db.ListCollections()
	for _, coll := range collList {
		db.DropCollection(coll)
	}
	colFinal, _ := db.ListCollections()

	log.Debug("collection count: ", len(colFinal))

}
