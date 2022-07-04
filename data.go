package main

import (
	"github.com/BurntSushi/toml"
)

//--------------------------------------------------------------------------
type Config struct {
	// nothing yet
	Foo bool
}

//--------------------------------------------------------------------------
type feed struct {
	name      string
	shortname string
	url       string
}

//--------------------------------------------------------------------------
type masterType struct {
	config Config
	feed   []feed
}

type FeedList struct {
	feed []feed
}

//--------------------------------------------------------------------------
func loadMaster() (Config, []feed) {

	var conf Config
	//var feedlist []Feed
	var feedls FeedList
	var master masterType

	data, err := toml.DecodeFile("./master.toml", &conf)
	if err != nil {
		log.Error("failed to open master.toml: ", err)
		return Config{}, nil
	}
	log.Debug(data)

	//	data.Unmarshal(&conf)

	//	toml.Unmarshal(data, &conf)
	//  rr = toml.Unmarshal(data, &feedls)

	log.Debug("config", conf)
	log.Debug("feeds: ", feedls)
	//return master.Config, master.Feeds
	return master.config, nil

}
