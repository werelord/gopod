package pod

import (
	"fmt"
	"gopod/testutils"
	"testing"
)

/*
func Test_createNewItemEntry(t *testing.T) {
	type args struct {
		parentConfig podconfig.FeedToml
		db           *poddb.PodDB
		hash         string
		xmlData      *podutils.XItemData
	}
	tests := []struct {
		name    string
		args    args
		want    *Item
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createNewItemEntry(tt.args.parentConfig, tt.args.db, tt.args.hash, tt.args.xmlData)
			if (err != nil) != tt.wantErr {
				t.Errorf("createNewItemEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createNewItemEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

*/
/*
func Test_createItemDataDBEntry(t *testing.T) {
	tests := []struct {
		name string
		want any
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createItemDataDBEntry(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createItemDataDBEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

*/
/*

func Test_loadFromDBEntry(t *testing.T) {
	type args struct {
		parentCfg podconfig.FeedToml
		db        *poddb.PodDB
		entry     poddb.DBEntry
	}
	tests := []struct {
		name    string
		args    args
		want    *Item
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadFromDBEntry(tt.args.parentCfg, tt.args.db, tt.args.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadFromDBEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loadFromDBEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

*/

func Test_parseUrl(t *testing.T) {
	type args struct {
		encUrl   string
		urlparse string
	}
	type exp struct {
		errStr string
		resUrl string
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"empty url", args{}, exp{errStr: "empty url"}},
		{"url incomplete", args{encUrl: "foo/bar/meh.mp3"}, exp{errStr: "failed url parsing"}},
		{"basic url", args{encUrl: "http://foo.bar/meh.mp3"}, exp{resUrl: "http://foo.bar/meh.mp3"}},
		{"url with query", args{encUrl: "http://foo.bar/meh.mp3?foo=bar"}, exp{resUrl: "http://foo.bar/meh.mp3"}},
		{"url with query and parse",
			args{encUrl: "http://track.me/foo/foo.bar.com/meh.mp3?foo=bar", urlparse: "foo.bar.com"},
			exp{resUrl: "http://foo.bar.com/meh.mp3"}},
		{"url with non-existant parse",
			args{encUrl: "http://track.me/foo/foo.bar.com/meh.mp3?foo=bar", urlparse: "meh.bar.com"},
			exp{resUrl: "http://track.me/foo/foo.bar.com/meh.mp3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			url, err := parseUrl(tt.p.encUrl, tt.p.urlparse)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.resUrl, url)
		})
	}
}

func Test_calcHash(t *testing.T) {
	type args struct {
		guid, url string
	}
	type exp struct {
		errStr    string
		validHash bool
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		//{"", args{}, exp{}},
		{"parse failure", args{guid: "foobar"}, exp{errStr: "failed to calc hash"}},
		{"empty guid", args{url: "http://foo.bar/meh.mp3"}, exp{validHash: true}},
		{"all exists", args{url: "http://foo.bar/meh.mp3", guid: "foobar"}, exp{validHash: true}},
		// todo: tests
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// what the hash results to is outside of scope

			hash, err := calcHash(tt.p.guid, tt.p.url, "")

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, (len(hash) == 28) == tt.e.validHash, fmt.Sprintf("unexpected hash: '%v'", hash))

		})
	}
}



// mocking for os, net

// type mockShit struct {
// 	// todo: shit here
// }



// func setupPodUtilsMock() (mockShit, func(string, mockShit)) {
// 	var ms = mockShit{}
// 	var oldImpl = pnuImpl
// 	pnuImpl = ms


// }

/*

func TestItem_Download(t *testing.T) {
	type args struct {
		mp3path string
	}
	tests := []struct {
		name    string
		i       *Item
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.i.Download(tt.args.mp3path); (err != nil) != tt.wantErr {
				t.Errorf("Item.Download() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/
