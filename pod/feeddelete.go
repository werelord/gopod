package pod

import (
	"fmt"
	"gopod/inputoption"
)

func (f *Feed) RunDelete() error {

	if err := f.LoadDBFeed(loadOptions{includeXml: true}); err != nil {
		// f.log.Error("failed to load feed data from db: ", err)
		return err
	}

	// verify input
	var desc = fmt.Sprintf("Deleting feed '%v'; please confirm", f.Shortname)
	if yn, err := inputoption.RunYesNoSelection(desc, inputoption.NO); err != nil {
		f.log.Errorf("error in input selection; exiting: %v", err)
		return err
	} else {
		switch yn {
		case inputoption.YES:
			f.log.Infof("confirmed; deleting feed '%v'", f.Shortname)
		case inputoption.NO:
			fallthrough
		default:
			f.log.Info("not confirmed, exiting...")
			return nil
		}
	}

	// run delete
	if err := f.delete(); err != nil {
		f.log.Errorf("failure to delete feed: %v", err)
		return err
	}

	return nil
}
