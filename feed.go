package main

var initialized bool = false

//--------------------------------------------------------------------------
func initDb() {
	return
	// if initialized == false {
	// 	var err error
	// 	dab, err = scribble.New("./db", nil)
	// 	if err != nil {
	// 		log.Error("Error", err)
	// 	}
	// 	initialized = true
	// }
	// log.Debug(dab)
}

//--------------------------------------------------------------------------
func loadFeedInfo(feed feed) {
	initDb()

	if len(feed.shortname) == 0 {
		feed.shortname = feed.name
	}

	log.Debug(feed)

}
