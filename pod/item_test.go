package pod

import (
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			url, err := parseUrl(tt.p.encUrl, tt.p.urlparse)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.resUrl, url)
		})
	}
}

/*
func Test_calcHash(t *testing.T) {
	// todo: this
}

/*
func TestItem_getItemXmlDBEntry(t *testing.T) {

	var xml = podutils.XItemData{
		Title: "foo",
		Pubdate: time.Now(),
		EpisodeStr: "42",
		Guid: "gooid",
		Link: "link",
		Author: "fu",
		Description: "meh",
		Enclosure: struct{Length uint; TypeStr string; Url string}{42, "meh", "url"},
	}



	tests := []struct {
		name string
		i    *Item
		want *poddb.DBEntry
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.i.getItemXmlDBEntry(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Item.getItemXmlDBEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestItem_getItemDataDBEntry(t *testing.T) {
	tests := []struct {
		name string
		i    *Item
		want *poddb.DBEntry
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.i.getItemDataDBEntry(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Item.getItemDataDBEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*

func TestItem_createProgressBar(t *testing.T) {
	tests := []struct {
		name string
		i    Item
		want *progressbar.ProgressBar
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.i.createProgressBar(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Item.createProgressBar() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/
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
