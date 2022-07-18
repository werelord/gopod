package main

import (
	"flag"
	"os"
	"strings"
)

//--------------------------------------------------------------------------
type CommandLine struct {
	Reload   bool
	Filename string
}

//--------------------------------------------------------------------------
func (c* CommandLine) initCommandLine(defaultConfig string) (bool) {

	var filename string

	flag.StringVar(&filename, "reload-config", defaultConfig, "usage")

	flag.Parse()

	c.Filename = strings.TrimSpace(filename)

	if c.Filename != "" {
		if _, err := os.Stat(c.Filename); err == nil {
			log.Debug("reloading configuration file: ", c.Filename)
			c.Reload = true
		} else {
			log.Debug(err)
		}
	}

	return c.Reload
}
