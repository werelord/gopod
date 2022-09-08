package pod

import (
	"gopod/podconfig"
	"gopod/podutils"
	"gopod/testutils"
	"testing"
)

/*
	func TestItem_generateFilename(t *testing.T) {
		type args struct {
			cfg podconfig.FeedToml
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
				if err := tt.i.generateFilename(tt.args.cfg); (err != nil) != tt.wantErr {
					t.Errorf("Item.generateFilename() error = %v, wantErr %v", err, tt.wantErr)
				}
			})
		}
	}
*/
func TestItem_replaceLinkFinalPath(t *testing.T) {
	type arg struct {
		repstr    string
		xLink     string
		defstring string
	}
	tests := []struct {
		name string
		p    arg
		want string
	}{
		{"empty string", arg{xLink: "https://foo.bar/83", defstring: "defstring"}, ""},
		{"empty link", arg{repstr: "test_#linkfinalpath#_bar", defstring: "defstring"}, "test_defstring_bar"},
		{"no replacement", arg{repstr: "foobar", xLink: "https://foo.bar/83", defstring: "defstring"}, "foobar"},
		{"parse error", arg{repstr: "test_#linkfinalpath#_bar", xLink: "foo\tbar", defstring: "defstring"}, "test_defstring_bar"},
		{"success", arg{repstr: "test_#linkfinalpath#_bar", xLink: "https://foo.bar/83", defstring: "def"}, "test_83_bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var item = Item{xmlData: &podutils.XItemData{Link: tt.p.xLink}}

			got := item.replaceLinkFinalPath(tt.p.repstr, tt.p.defstring)
			testutils.AssertEquals(t, tt.want, got)
		})
	}
}

func TestItem_replaceEpisode(t *testing.T) {
	type args struct {
		str        string
		epstr      string
		defaultRep string
		padlen     int
	}
	tests := []struct {
		name string
		p    args
		want string
	}{
		{"empty string", args{epstr: "42", defaultRep: "defstr"}, ""},
		{"no replacement", args{str: "foobar", epstr: "42", defaultRep: "defstr"}, "foobar"},
		{"empty episode", args{str: "foo#episode#bar", defaultRep: "defstr"}, "foodefstrbar"},
		{"normal pad length", args{str: "foo#episode#bar", epstr: "42", defaultRep: "defstr"}, "foo042bar"},
		{"diff pad length", args{str: "foo#episode#bar", epstr: "42", defaultRep: "defstr", padlen: 6}, "foo000042bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var item = Item{xmlData: &podutils.XItemData{EpisodeStr: tt.p.epstr}}
			var cfg = podconfig.FeedToml{EpisodePad: tt.p.padlen}
			got := item.replaceEpisode(tt.p.str, tt.p.defaultRep, cfg)
			testutils.AssertEquals(t, tt.want, got)

		})
	}
}

/*
	func TestItem_replaceTitleRegex(t *testing.T) {
		type args struct {
			dststr string
			regex  string
			title  string
		}
		tests := []struct {
			name    string
			i       Item
			args    args
			want    string
			wantErr bool
		}{
			// TODO: Add test cases.
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := tt.i.replaceTitleRegex(tt.args.dststr, tt.args.regex, tt.args.title)
				if (err != nil) != tt.wantErr {
					t.Errorf("Item.replaceTitleRegex() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("Item.replaceTitleRegex() = %v, want %v", got, tt.want)
				}
			})
		}
	}
*/
func TestItem_replaceUrlFilename(t *testing.T) {
	type args struct {
		str      string
		url      string
		skiptrim bool
	}
	type exp struct {
		want        string
		cleanCalled bool
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"empty string", args{url: ""}, exp{want: ""}},
		{"no replacement", args{str: "foobar", url: ""}, exp{want: "foobar"}},
		{"normal rep", args{str: "foo#urlfilename#bar", url: "http://foo.bar/meh/filename.mp3"},
			exp{want: "foofilename.mp3bar", cleanCalled: true}},
		{"skip trim", args{str: "foo#urlfilename#bar", url: "http://foo.bar/meh/filename.mp3", skiptrim: true},
			exp{want: "foofilename.mp3bar", cleanCalled: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var cleanCalled bool
			var oldClean = cleanFilename
			cleanFilename = func(s string) string { cleanCalled = true; return s }
			defer func() { cleanFilename = oldClean }()

			var item = Item{ItemData: ItemData{Url: tt.p.url}}
			var cfg = podconfig.FeedToml{SkipFileTrim: tt.p.skiptrim}
			got := item.replaceUrlFilename(tt.p.str, cfg)
			testutils.AssertEquals(t, tt.e.want, got)
			testutils.Assert(t, cleanCalled == tt.e.cleanCalled, "clean called incorrect")

		})
	}
}
