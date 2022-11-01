package main

import "gopod/pod"

//lint:file-ignore U1000 Ignore all unused methods, one-time or testing use only

func migrateCount(feedmap map[string]*pod.Feed) {
	for _, feed := range feedmap {

		feed.MigrateCount()
	}
}
