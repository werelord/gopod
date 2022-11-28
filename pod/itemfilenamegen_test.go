package pod

import (
	"fmt"
	"gopod/podconfig"
	"gopod/podutils"
	"gopod/testutils"
	"testing"
	"time"

	"golang.org/x/exp/slices"
)

func TestItem_generateFilename(t *testing.T) {

	var (
		testTime    = time.Now()
		testTimeRep = testTime.AddDate(0, 0, -7)
		defstr      = testTime.Format(podutils.TimeFormatStr)

		regex = "\\((VOY|DS9|Voyager)? ?(S[0-9]E[0-9]+ ?&? ?[0-9]*)\\)"

		noCollFunc   = func(string) bool { return false }
		allCollFunc  = func(string) bool { return true }
		collFooList  = []string{"foo.bar", "fooA.bar", "fooB.bar", "fooC.bar"}
		specificColl = func(s string) bool {
			return slices.Contains(collFooList, s)
		}
	)

	type cfgarg struct {
		shortname string
		regex     string
		parse     string
		xmlNil    bool
		collFunc  func(string) bool
	}
	type itemarg struct {
		url          string
		xLink        string
		epStr        string
		itemcount    int
		sesnStr      string
		title        string
		defaultTime  time.Time
		filenameXtra string
	}
	type exp struct {
		filename       string
		extra          string
		collFuncCalled bool
		errStr         string
	}
	tests := []struct {
		name string
		c    cfgarg
		i    itemarg
		e    exp
	}{
		{"item xml nil", cfgarg{xmlNil: true, shortname: "foo", parse: "foo#shortname#bar"}, itemarg{},
			exp{errStr: "item xml is nil"}},
		{"filenameParse empty", cfgarg{parse: ""}, itemarg{url: "http://foo.bar/meh.mp3"},
			exp{filename: "meh.mp3"}},
		{"shortname", cfgarg{shortname: "foo", parse: "foo#shortname#bar"}, itemarg{},
			exp{filename: "foofoobar"}},
		{"linkFinalpath", cfgarg{parse: "foo#linkfinalpath#bar"}, itemarg{xLink: "https://foo.bar/83"},
			exp{filename: "foo83bar"}},
		{"linkFinalpath missing", cfgarg{parse: "foo#linkfinalpath#bar"}, itemarg{},
			exp{filename: "foo" + defstr + "bar"}},
		{"count", cfgarg{parse: "foo#count#bar"}, itemarg{itemcount: 42},
			exp{filename: "foo042bar"}},
		{"negative count", cfgarg{parse: "foo#count#bar"}, itemarg{itemcount: -1},
			exp{filename: "foo" + defstr + "bar"}},
		{"episode string", cfgarg{parse: "foo#episode#bar"}, itemarg{epStr: "42"},
			exp{filename: "foo042bar"}},
		{"episode string missing", cfgarg{parse: "foo#episode#bar"}, itemarg{},
			exp{filename: "foo" + defstr + "bar"}},
		{"season string", cfgarg{parse: "foo#season#bar"}, itemarg{sesnStr: "13"},
			exp{filename: "foo13bar"}},
		{"season string missing", cfgarg{parse: "foo#season#bar"}, itemarg{},
			exp{filename: "foobar"}},
		{"pubdate", cfgarg{parse: "foo_#date#_bar"}, itemarg{},
			exp{filename: "foo_" + defstr + "_bar"}},
		{"pubdate, replacement", cfgarg{parse: "foo_#date#_bar"}, itemarg{defaultTime: testTimeRep},
			exp{filename: "foo_" + testTimeRep.Format(podutils.TimeFormatStr) + "_bar"}},
		{"regex err", cfgarg{parse: "foo#titleregex:1#bar"}, itemarg{}, exp{errStr: "regex is empty"}},
		{"regex title", cfgarg{parse: "foo#titleregex:1##titleregex:2#bar", regex: regex},
			itemarg{title: "foo (VOY S4E15)"},
			exp{filename: "fooVOYS4E15bar"}},
		{"url filename", cfgarg{parse: "foo#urlfilename#"}, itemarg{url: "http://foo.bar/meh.mp3"},
			exp{filename: "foomeh.mp3"}},

		{"no collisions", cfgarg{collFunc: noCollFunc, parse: "foobar.mp3"}, itemarg{},
			exp{filename: "foobar.mp3", collFuncCalled: true}},
		{"all collisions", cfgarg{collFunc: allCollFunc, parse: "foobar.mp3"}, itemarg{},
			exp{errStr: "still collides", collFuncCalled: true}},
		{"specific collision", cfgarg{parse: "foo.bar", collFunc: specificColl}, itemarg{},
			exp{filename: "fooD.bar", extra: "D", collFuncCalled: true}},
		{"specific collision from url", cfgarg{collFunc: specificColl}, itemarg{url: "http://foo.com/foo.bar"},
			exp{filename: "fooD.bar", extra: "D", collFuncCalled: true}},
		{"extra already defined", cfgarg{collFunc: allCollFunc, parse: "foobar.mp3"}, itemarg{filenameXtra: "Z"},
			exp{filename: "foobarZ.mp3", extra: "Z", collFuncCalled: false}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var item = Item{}
			item.Url = tt.i.url
			item.EpNum = tt.i.itemcount
			if tt.c.xmlNil == false {
				item.XmlData = &ItemXmlDBEntry{}
				item.XmlData.Title = tt.i.title
				item.XmlData.Link = tt.i.xLink
				item.XmlData.EpisodeStr = tt.i.epStr
				item.XmlData.SeasonStr = tt.i.sesnStr

				if tt.i.defaultTime.IsZero() == false {
					var oldTimeNow = timeNow
					timeNow = func() time.Time { return tt.i.defaultTime }
					defer func() { timeNow = oldTimeNow }()
				} else {
					item.XmlData.Pubdate = testTime
				}

			}
			item.FilenameXta = tt.i.filenameXtra

			var cfg = podconfig.FeedToml{}
			cfg.Shortname = tt.c.shortname
			cfg.FilenameParse = tt.c.parse
			cfg.Regex = tt.c.regex

			var collFuncCalled bool
			var collfuncHolder func(string) bool
			if tt.c.collFunc != nil {
				collfuncHolder = func(s string) bool {
					collFuncCalled = true
					return tt.c.collFunc(s)
				}
			}

			filename, extra, err := item.generateFilename(cfg, collfuncHolder)

			// make sure generate filename does not set the filename
			testutils.Assert(t, item.Filename == "", fmt.Sprintf("expecting item.Filename to be blank, got '%v'", item.Filename))

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.filename, filename)
			testutils.AssertEquals(t, tt.e.extra, extra)
			testutils.Assert(t, collFuncCalled == tt.e.collFuncCalled,
				fmt.Sprintf("expected collision func called to be %v, got %v", tt.e.collFuncCalled, collFuncCalled))

		})
	}
}

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

			var item = Item{}
			item.XmlData = &ItemXmlDBEntry{}
			item.XmlData.Link = tt.p.xLink

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

			var item = Item{}
			item.XmlData = &ItemXmlDBEntry{}
			item.XmlData.EpisodeStr = tt.p.epstr

			var cfg = podconfig.FeedToml{EpisodePad: tt.p.padlen}
			got := item.replaceEpisode(tt.p.str, tt.p.defaultRep, cfg)
			testutils.AssertEquals(t, tt.want, got)

		})
	}
}

func TestItem_replaceCount(t *testing.T) {
	type args struct {
		str        string
		itemcount  int
		defaultRep string
		padlen     int
	}
	tests := []struct {
		name string
		p    args
		want string
	}{
		{"empty string", args{itemcount: 42, defaultRep: "defstr"}, ""},
		{"no replacement", args{str: "foobar", itemcount: 42, defaultRep: "defstr"}, "foobar"},
		{"negative count", args{itemcount: -1, str: "foo#count#bar", defaultRep: "defstr"}, "foodefstrbar"},
		{"normal pad length", args{str: "foo#count#bar", itemcount: 42, defaultRep: "defstr"}, "foo042bar"},
		{"diff pad length", args{str: "foo#count#bar", itemcount: 42, defaultRep: "defstr", padlen: 6}, "foo000042bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var item = Item{}
			item.EpNum = tt.p.itemcount

			var cfg = podconfig.FeedToml{EpisodePad: tt.p.padlen}
			got := item.replaceCount(tt.p.str, tt.p.defaultRep, cfg)
			testutils.AssertEquals(t, tt.want, got)

		})
	}
}

func TestItem_replaceTitleRegex(t *testing.T) {

	var (
		title1 = "Friend to the Brass Plaque (VOY S4E15)"
		parse1 = "tgg.epfoo.VOYS4E15.mp3"
		title2 = "My Neck, My Back, My Reproductive Sack (VOY S2E4)"
		parse2 = "tgg.epfoo.VOYS2E4.mp3"
		title3 = "Casa de Tain  (DS9 S7E24)"
		parse3 = "tgg.epfoo.DS9S7E24.mp3"

		titlebroken1 = "The Prophet Goodbye (DS9 S7E25 & 26)"
		parsebroken1 = "tgg.epfoo.DS9S7E25_&_26.mp3"
		titlebroken2 = "Deep Deep Dimp (Crimson Tide - Bonus Episode)"

		regex = "\\((VOY|DS9|Voyager)? ?(S[0-9]E[0-9]+ ?&? ?[0-9]*)\\)"

		filenameParse = "tgg.epfoo.#titleregex:1##titleregex:2#.mp3"
	)

	type args struct {
		str   string
		regex string
		title string
	}
	type exp struct {
		want   string
		errStr string
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		{"empty string", args{str: "", regex: regex, title: title1},
			exp{}},
		{"no replacement", args{str: "foobar", regex: regex, title: title1},
			exp{want: "foobar"}},
		{"regex empty", args{str: filenameParse, regex: "", title: title1},
			exp{errStr: "regex is empty"}},
		{"regex compile err", args{str: filenameParse, regex: "[].*", title: title1},
			exp{errStr: "error parsing regexp"}},
		{"matches don't fit", args{str: filenameParse, regex: regex, title: titlebroken2},
			exp{want: "tgg.epfoo.mp3"}},

		{"replace success one", args{str: filenameParse, regex: regex, title: title1},
			exp{want: parse1}},
		{"replace success two", args{str: filenameParse, regex: regex, title: title2},
			exp{want: parse2}},
		{"replace success three", args{str: filenameParse, regex: regex, title: title3},
			exp{want: parse3}},

		{"wierd 1", args{str: filenameParse, regex: regex, title: titlebroken1},
			exp{want: parsebroken1}},
		{"wierd 2", args{str: filenameParse, regex: regex, title: titlebroken2},
			exp{want: "tgg.epfoo.mp3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item = Item{}
			item.XmlData = &ItemXmlDBEntry{}
			item.XmlData.Title = tt.p.title

			got, err := item.replaceTitleRegex(tt.p.str, tt.p.regex)
			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.want, got)
		})
	}
}

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

			var item = Item{}
			item.Url = tt.p.url

			var cfg = podconfig.FeedToml{SkipFileTrim: tt.p.skiptrim}
			got := item.replaceUrlFilename(tt.p.str, cfg)
			testutils.AssertEquals(t, tt.e.want, got)
			testutils.Assert(t, cleanCalled == tt.e.cleanCalled, "clean called incorrect")

		})
	}
}

func TestItem_checkFilenameCollisions(t *testing.T) {

	var (
		noCollFunc   = func(string) bool { return false }
		allCollFunc  = func(string) bool { return true }
		collFooList  = []string{"foo.bar", "fooA.bar", "fooB.bar", "fooC.bar"}
		specificColl = func(s string) bool {
			return slices.Contains(collFooList, s)
		}
	)

	type arg struct {
		collFunc     func(string) bool
		filenameXtra string
	}
	type exp struct {
		filename string
		extra    string
		errStr   string
	}
	tests := []struct {
		name string
		p    arg
		e    exp
	}{
		{"no collisions", arg{collFunc: noCollFunc}, exp{filename: "foo.bar"}},
		{"all collisions", arg{collFunc: allCollFunc}, exp{errStr: "still collides"}},
		{"specific collision", arg{collFunc: specificColl}, exp{filename: "fooD.bar", extra: "D"}},
		{"collision func nil", arg{}, exp{filename: "foo.bar"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var item = Item{}
			item.FilenameXta = tt.p.filenameXtra

			filename, extra, err := item.checkFilenameCollisions("foo.bar", tt.p.collFunc)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.filename, filename)
			testutils.AssertEquals(t, tt.e.extra, extra)
		})
	}
}
