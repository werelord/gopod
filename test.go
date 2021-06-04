package main

import (
	"encoding/json"

	// json-based database, for the hell of it
	scribble "github.com/nanobox-io/golang-scribble"
)

type tFish struct {
	Name string `json:"name"`
	Foo  string `json:"foo"`
	Bar  string `json:"bar"`
}

func init() {
	initLogging("gopod.log")
}

func main() {

	// log.Debug("debug")
	// log.Warn("ofo")
	// log.WithFields(logrus.Fields{
	// 	"arm": true,
	// }).Info("true")
	//logger.Print("foo")

	// load the

	db, err := scribble.New("./", nil)
	if err != nil {
		log.Error("Error", err)
	}

	// Write a fish to the database
	for _, name := range []string{"onefish", "twofish", "redfish", "bluefish"} {
		db.Write("armfish", name, tFish{Name: name})
	}

	// Write a fish to the database
	f := tFish{Name: "fucker", Foo: "foobar", Bar: "barfoo"}
	log.Debug("fish:", f)
	if err := db.Write("armfish", "onefish", f); err != nil {
		log.Error("Error", err)
	}

	// Read a fish from the database (passing fish by reference)
	onefish := tFish{}
	if err := db.Read("armfish", "onefish", &onefish); err != nil {
		log.Error("Error", err)
	}
	log.Debug("onefish:", onefish)

	// Read all fish from the database, unmarshaling the response.
	records, err := db.ReadAll("armfish")
	if err != nil {
		log.Error("Error", err)
	}
	log.Debug(records)

	fishies := []tFish{}
	for _, f := range records {
		fishFound := tFish{}
		if err := json.Unmarshal([]byte(f), &fishFound); err != nil {
			log.Debug("Error", err)
		}
		fishies = append(fishies, fishFound)
	}
	log.Debug("fishies: ", fishies)

	// Delete a fish from the database
	// if err := db.Delete("fish", "onefish"); err != nil {
	// 	fmt.Println("Error", err)
	// }

	// // Delete all fish from the database
	// if err := db.Delete("fish", ""); err != nil {
	// 	fmt.Println("Error", err)
	// }
	/*

		dir := "./"

		db, err := scribble.New(dir, nil)
		if err != nil {
			fmt.Println("Error", err)
		}

		// Write a fish to the database
		for _, name := range []string{"onefish", "twofish", "redfish", "bluefish"} {
			db.Write("fish", name, Fish{Name: name})
		}

		// Read a fish from the database (passing fish by reference)
		onefish := Fish{}
		if err := db.Read("fish", "onefish", &onefish); err != nil {
			fmt.Println("Error", err)
		}

		// Read all fish from the database, unmarshaling the response.
		records, err := db.ReadAll("fish")
		if err != nil {
			fmt.Println("Error", err)
		}

		fishies := []Fish{}
		for _, f := range records {
			fishFound := Fish{}
			if err := json.Unmarshal([]byte(f), &fishFound); err != nil {
				fmt.Println("Error", err)
			}
			fishies = append(fishies, fishFound)
		}

		// // Delete a fish from the database
		// if err := db.Delete("fish", "onefish"); err != nil {
		// 	fmt.Println("Error", err)
		// }
		//
		// // Delete all fish from the database
		// if err := db.Delete("fish", ""); err != nil {
		// 	fmt.Println("Error", err)
		// }
	*/
}
