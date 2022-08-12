package main

import (
	"gopod/pod"
	"gopod/poddb"

	log "github.com/sirupsen/logrus"
)

func test(flist map[string]*pod.Feed) {

	
	//tryFetch(db)
	
	for _, f := range flist {

		db, err := poddb.NewDB(f.Shortname)
		if err != nil {
			log.Error("failed: ", err)
			return
		}

		fexport := f.CreateExport()

		for _, entry := range fexport {
			log.Debugf("Inserting entry: %+v", entry)
			id, err := db.InsertyByEntry(entry)
			if err != nil {
				log.Error("error: ", err)
			}
			log.Debugf("id returned: %v", id)
		}
	}

	poddb.DumpCollections(".\\dbexport")
}

func tryFetch(db *poddb.PodDB) {
	var (
		err       error
		id        string
		xmlStruct pod.XmlDBEntry
		itemlist  pod.ItemListDBEntry

		arm = struct {
			Name string
			Age  int
		}{}

		leg = struct {
			Foobar string
		}{}
	)

	if id, err = db.FetchById("foobar", &xmlStruct); err != nil {
		log.Errorf("error fetching by id: %v (id='%v')", err, id)
	}

	if id, err = db.FetchByEntry(&arm); err != nil {
		log.Errorf("error fetching by entry: %v (id='%v')", err, id)
	}

	if id, err = db.FetchByEntry(&leg); err != nil {
		log.Errorf("error fetching by entry: %v (id='%v')", err, id)
	}

	if id, err = db.FetchById("53d30df7-7a39-4a2e-a4fb-dca70da3736f", &xmlStruct); err != nil {
		log.Error("error fetching by id: ", err)
	}
	log.Debugf("id: %v, xml retrieved: %+v", id, xmlStruct.XmlFeedData.Title)

	if id, err = db.FetchByEntry(&itemlist); err != nil {
		log.Error("error fetching by entry: ", err)
	}
	log.Debugf("id: %v, list retrieved: %+v", id, len(itemlist.ItemEntryList))
}
