package pod

func (f *Feed) RunDelete() error {

	/* todo:
	verification input
	fetch feed & xml
	fetch items & xml

	run (soft) delete

	todo: check for deleted items still in master

	*/

	var (
		itemlist []*Item
		err      error
	)

	if err = f.LoadDBFeed(true); err != nil {
		f.log.Error("failed to load feed data from db: ", err)
		return err
	} else if itemlist, err = f.loadDBFeedItems(allItems, true, cASC); err != nil {
		f.log.Error("failed to load item entries: ", err)
		return err
	}
	f.log.Debug("Feed loaded from db for check download")
	f.log.Debug("do something, ", itemlist)

	// for _, item := range itemlist {
		
	// }

	// todo: check for deleted items

	return nil
}
