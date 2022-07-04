package main

//--------------------------------------------------------------------------
func init() {
	initLogging("gopod.log")
}

//--------------------------------------------------------------------------
func main() {
	//test()

	//todo: command-line vars

	config, _ := loadMaster()

	log.Debug("config:", config)
	//log.Debug(feedlist)

	/*	for _, feed := range feedlist[:1] {
			log.Debug(feed.Name)
			log.Debug(feed.FeetUrl)

			loadFeedInfo(feed)
		}
	*/
}
