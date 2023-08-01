package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"gopod/testutils"
	"math/rand"
	"testing"
	"time"

	"github.com/go-test/deep"
	"golang.org/x/exp/slices"

	//"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mostly integration tests.. mocks in poddb_testmock.go

func TestNewDB(t *testing.T) {
	type arg struct {
		pathEmpty bool
		createVer int
		mock      mockGorm
		termErr   []stackType
	}
	type exp struct {
		createVerCalled bool
		dbNil           bool
		errStr          string
		callStack       []stackType
	}
	tests := []struct {
		name string
		p    arg
		e    exp
	}{

		{"empty path", arg{pathEmpty: true, mock: mockGorm{mockdb: &mockGormDB{}}},
			exp{dbNil: true, errStr: "db path cannot be empty"}},
		{"open error", arg{mock: mockGorm{openErr: true, mockdb: &mockGormDB{}}},
			exp{dbNil: true, errStr: "error opening db: foobar", callStack: []stackType{open}}},
		// todo: create ver fail, new unit test on createNewDb()
		// {"create ver fail",	
		// 	arg{mock: mockGorm{mockdb: &mockGormDB{}}, termErr: []stackType{exec, scan}, createVer: -1},
		// 	exp{dbNil: true, createVerCalled: true, errStr: "error finding db version",
		// 		callStack: []stackType{open, raw, scan, exec}}},
		{"model version mismatch", arg{mock: mockGorm{mockdb: &mockGormDB{}}, createVer: 42},
			exp{dbNil: true, errStr: "model doesn't match current", callStack: []stackType{open, raw, scan}}},

		// todo: create version table, new unit test on createNewDb()
		// {"success, create version table", arg{mock: mockGorm{mockdb: &mockGormDB{}}, createVer: -1},
		// 	exp{createVerCalled: true, callStack: []stackType{open, raw, scan, exec}}},

		{"success, matching model versions", arg{mock: mockGorm{mockdb: &mockGormDB{}}},
			exp{callStack: []stackType{open, raw, scan}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var gmock, teardown = setupGormMock(t, &tt.p.mock, true)
			defer teardown(t, gmock)

			resetCallStack()

			// if negative, no create ver called
			// if zero, use default
			// greater than zero, likely used for mismatch
			if tt.p.createVer >= 0 {
				var createVer = tt.p.createVer
				if createVer == 0 {
					createVer = currentModel
				}
				// create version
				if res := gmock.mockdb.DB.
					Exec("CREATE TABLE poddb_model (ID integer); INSERT INTO poddb_model (ID) VALUES (?)",
						createVer); res.Error != nil {
					t.Fatalf("setup version failed: %v", res.Error)
				}
			}
			gmock.mockdb.termErr = tt.p.termErr

			got, err := NewDB(podutils.Tern(tt.p.pathEmpty, "", inMemoryPath))

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, (got == nil) == tt.e.dbNil, fmt.Sprintf("expected db nil, got %v", got))

			var createVerCalled = slices.Contains(callStack, exec)
			testutils.Assert(t, tt.e.createVerCalled == createVerCalled,
				fmt.Sprintf("create version table called expect: %v, got %v", tt.e.createVerCalled, createVerCalled))

			compareCallstack(t, tt.e.callStack)
		})
	}
}

func TestPodDB_IsFeedDeleted(t *testing.T) {

	var gmock, teardown = setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	var (
		feed1, feed2 = generateFeed(false), generateFeed(false)

		defCallStack = []stackType{open, unscoped, model, where, count}
	)

	// insert original
	if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*FeedDBEntry{&feed1, &feed2}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows\n", res.RowsAffected)
	}

	// delete feed 2
	if res := gmock.mockdb.DB.Delete(&feed2); res.Error != nil {
		t.Fatalf("error in delete: %v", res.Error)
	} else {
		fmt.Printf("deleted %v rows\n", res.RowsAffected)
	}

	type args struct {
		emptyPath bool
		openErr   bool
		termErr   stackType
		hash      string
	}
	type exp struct {
		deleted   bool
		errStr    string
		callStack []stackType
	}

	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// error tests, no db changes
		{"empty path", args{emptyPath: true},
			exp{errStr: "poddb is not initialized"}},
		{"empty hash", args{},
			exp{errStr: "hash cannot be empty"}},
		{"open error", args{openErr: true, hash: "foobar"},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"count error", args{termErr: count, hash: "foobar"},
			exp{errStr: "count:foobar", callStack: defCallStack}},

		// success tests
		{"not deleted", args{hash: feed1.Hash}, exp{deleted: false, callStack: defCallStack}},
		{"deleted", args{hash: feed2.Hash}, exp{deleted: true, errStr: "feed deleted", callStack: defCallStack}},
		{"feed does not exist", args{hash: "foobar"}, exp{deleted: false, callStack: defCallStack}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resetCallStack()

			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			isDel, err := poddb.isFeedDeleted(tt.p.hash)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, tt.e.deleted == isDel,
				fmt.Sprintf("expecting deleted == %v, got %v", tt.e.deleted, isDel))
			// make sure if deleted the error returned is the correct type
			if isDel {
				testutils.Assert(t, errors.Is(err, &ErrorFeedDeleted{}) == true,
					fmt.Sprintf("expected error ErrorFeedDeleted, got %T", err))
			}
			compareCallstack(t, tt.e.callStack)

		})
	}
}

func TestPodDB_loadDBFeed(t *testing.T) {

	var gmock, teardown = setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	// insert shit here, for retrieval
	var (
		entryOne, entryTwo, newEntry = generateFeed(true), generateFeed(true), generateFeed(true)
		entryOneNoXml, entryTwoNoXml FeedDBEntry

		hashQuery, idQuery FeedDBEntry
	)

	if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*FeedDBEntry{&entryOne, &entryTwo}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows, ids: %v %v\n", res.RowsAffected, entryOne.ID, entryTwo.ID)
	}
	hashQuery.Hash = entryOne.Hash
	entryOneNoXml = entryOne
	entryOneNoXml.XmlFeedData = nil

	idQuery.ID = entryTwo.ID
	entryTwoNoXml = entryTwo
	entryTwoNoXml.XmlFeedData = nil

	var (
		defCS        = []stackType{open, where, firstorcreate}
		defCSWithXml = []stackType{open, where, preload, firstorcreate}
	)

	type args struct {
		emptyPath bool
		openErr   bool
		entryNil  bool
		termErr   stackType
		fq        FeedDBEntry
		loadXml   bool
	}
	type exp struct {
		useDB     bool
		fe        FeedDBEntry
		errStr    string
		callStack []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// error results
		{"empty path", args{emptyPath: true, fq: hashQuery}, exp{errStr: "poddb is not initialized"}},
		{"entry nil", args{entryNil: true}, exp{errStr: "feed cannot be nil"}},
		{"missing id+hash", args{}, exp{errStr: "hash or ID has not been set"}},
		{"open error", args{openErr: true, fq: hashQuery},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"firstOrCreate error", args{termErr: firstorcreate, fq: hashQuery},
			exp{errStr: "firstorcreate:foobar", callStack: defCS}},
		// success results
		{"success with hash", args{fq: hashQuery},
			exp{fe: entryOneNoXml, callStack: defCS}},
		{"success with hash, loadXml", args{fq: hashQuery, loadXml: true},
			exp{fe: entryOne, callStack: defCSWithXml}},
		{"success with id", args{fq: idQuery},
			exp{fe: entryTwoNoXml, callStack: defCS}},
		{"success with id, loadXml", args{fq: idQuery, loadXml: true},
			exp{fe: entryTwo, callStack: defCSWithXml}},
		{"create new", args{fq: newEntry, loadXml: true},
			exp{useDB: true, callStack: defCSWithXml}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			resetCallStack()
			var fq = tt.p.fq

			opt := loadOptions{includeXml: tt.p.loadXml}
			err := poddb.loadFeed(podutils.Tern(tt.p.entryNil, nil, &fq), opt)
			if testutils.AssertErrContains(t, tt.e.errStr, err) {
				// general expected structure
				testutils.Assert(t, fq.ID > 0, fmt.Sprintf("Id > 0, got %v", fq.ID))
				testutils.Assert(t, fq.XmlId > 0, fmt.Sprintf("xmlId > 0, got %v", fq.ID))

				// see if xml is loaded

				testutils.Assert(t, (fq.XmlFeedData != nil) == (tt.p.loadXml),
					fmt.Sprintf("expected XmlFeedData(nil) == %v, got %v", !tt.p.loadXml, fq.XmlFeedData))

				var exp = tt.e.fe
				if tt.e.useDB {
					// pull the entry from the db, compare
					var dbEntry FeedDBEntry
					dbEntry.ID = fq.ID
					if res := gmock.mockdb.DB.Preload("XmlFeedData").Find(&dbEntry); res.Error != nil {
						t.Errorf("error in checking: %v", err)
					}
					exp = dbEntry
				}
				testutils.AssertEquals(t, exp, fq)
			}
			// check the call stack

			compareCallstack(t, tt.e.callStack)
		})
	}
}

func TestPodDB_loadDBFeedXml(t *testing.T) {

	gmock, teardown := setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	// insert stuff for retrieval
	var (
		entryOne, entryTwo FeedXmlDBEntry
	)

	entryOne.Title = "foobar"
	entryOne.Author = "meh"

	entryTwo.Title = "barfoo"
	entryTwo.PubDate = time.Now()

	if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*FeedXmlDBEntry{&entryOne, &entryTwo}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows, ids: %v %v\n", res.RowsAffected, entryOne.ID, entryTwo.ID)
	}

	var expCS = []stackType{open, where, first}

	type args struct {
		emptyPath bool
		openErr   bool
		termErr   stackType
		xmlId     uint
	}
	type exp struct {
		fxe       FeedXmlDBEntry
		errStr    string
		callStack []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// error tests
		{"empty path", args{emptyPath: true, xmlId: entryOne.ID}, exp{errStr: "poddb is not initialized"}},
		{"id zero", args{}, exp{errStr: "xml ID cannot be zero"}},
		{"open error", args{openErr: true, xmlId: entryOne.ID},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"generic find error", args{termErr: first, xmlId: entryOne.ID},
			exp{errStr: "first:foobar", callStack: expCS}},

		// not existing tests
		{"id doesn't exist", args{xmlId: 99},
			exp{errStr: "record not found", callStack: expCS}},

		// success
		{"success one", args{xmlId: entryOne.ID}, exp{fxe: entryOne, callStack: expCS}},
		{"success two", args{xmlId: entryTwo.ID}, exp{fxe: entryTwo, callStack: expCS}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			var xmlEntry, err = poddb.loadDBFeedXml(tt.p.xmlId)
			testutils.AssertErrContains(t, tt.e.errStr, err)
			if err != nil {
				testutils.Assert(t, xmlEntry == nil, fmt.Sprintf("expected xmlEntry == nil, got %v", xmlEntry))
			} else {
				testutils.Assert(t, xmlEntry.ID > 0, fmt.Sprintf("Id > 0, got %v", xmlEntry.ID))

				// compare objects
				testutils.AssertEquals(t, tt.e.fxe, *xmlEntry)
			}

			compareCallstack(t, tt.e.callStack)
		})
	}
}

func TestPodDB_loadFeedItems(t *testing.T) {

	gmock, teardown := setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	var (
		item1, item2, item3 = generateItem(0, true), generateItem(0, true), generateItem(0, true)
		itemA, itemB, itemC = generateItem(0, true), generateItem(0, true), generateItem(0, true)
	)

	// semi random shit
	var datelist = []string{"2022-03-01", "2022-01-01", "2022-02-01", "2005-05-01", "2005-06-01", "2005-04-01"}
	for idx, item := range []*ItemDBEntry{&item1, &item2, &item3, &itemA, &itemB, &itemC} {
		item.FeedId = uint(podutils.Tern(idx < 3, 1, 2))
		var fu error
		item.PubTimeStamp, fu = time.Parse("2006-01-02", datelist[idx])
		if fu != nil {
			t.Errorf("fuck, %v", fu)
		}
	}

	if err := gmock.mockdb.DB.AutoMigrate(&ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create(
		[]*ItemDBEntry{&item1, &item2, &item3, &itemA, &itemB, &itemC}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows\n", res.RowsAffected)
	}

	var rmXml = func(e ItemDBEntry) *ItemDBEntry {
		return &ItemDBEntry{e.PodDBModel, e.Hash, e.FeedId, e.ItemData, e.XmlId, nil}
	}

	type args struct {
		emptyPath  bool
		openErr    bool
		termErr    stackType
		feedId     uint
		numItems   int
		includeXml bool
		asc        bool
	}
	type exp struct {
		resultList []*ItemDBEntry
		errStr     string
		callStack  []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// order pubdate desc: item1, item3, item2, itemB, itemA, itemC

		// error results
		{"empty path", args{emptyPath: true}, exp{errStr: "poddb is not initialized", resultList: nil}},
		{"feed id zero", args{}, exp{errStr: "feed id cannot be zero"}},
		{"open error", args{openErr: true, feedId: 2},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"find error", args{termErr: find, feedId: 2},
			exp{errStr: "find:foobar", resultList: []*ItemDBEntry{}, callStack: []stackType{open, where, order, find}}},

		// success results
		{"no results", args{feedId: 3},
			exp{resultList: []*ItemDBEntry{}, callStack: []stackType{open, where, order, find}}},
		{"full results", args{feedId: 2},
			exp{resultList: []*ItemDBEntry{rmXml(itemB), rmXml(itemA), rmXml(itemC)},
				callStack: []stackType{open, where, order, find}}},
		{"full results (asc)", args{feedId: 2, asc: true},
			exp{resultList: []*ItemDBEntry{rmXml(itemC), rmXml(itemA), rmXml(itemB)},
				callStack: []stackType{open, where, order, find}}},
		{"limit results", args{feedId: 1, numItems: 2},
			exp{resultList: []*ItemDBEntry{rmXml(item1), rmXml(item3)},
				callStack: []stackType{open, where, order, limit, find}}},
		{"limit results (asc)", args{feedId: 1, numItems: 2, asc: true},
			exp{resultList: []*ItemDBEntry{rmXml(item2), rmXml(item3)},
				callStack: []stackType{open, where, order, limit, find}}},
		{"full include xml", args{feedId: 2, includeXml: true},
			exp{resultList: []*ItemDBEntry{&itemB, &itemA, &itemC},
				callStack: []stackType{open, where, order, preload, find}}},
		{"full include xml (asc)", args{feedId: 2, includeXml: true, asc: true},
			exp{resultList: []*ItemDBEntry{&itemC, &itemA, &itemB},
				callStack: []stackType{open, where, order, preload, find}}},
		{"limit include xml", args{feedId: 1, numItems: 2, includeXml: true},
			exp{resultList: []*ItemDBEntry{&item1, &item3},
				callStack: []stackType{open, where, order, limit, preload, find}}},
		{"limit include xml (asc)", args{feedId: 1, numItems: 2, includeXml: true, asc: true},
			exp{resultList: []*ItemDBEntry{&item2, &item3},
				callStack: []stackType{open, where, order, limit, preload, find}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			var direction = podutils.Tern(tt.p.asc == true, cASC, cDESC)

			var opt = loadOptions{includeXml: tt.p.includeXml, direction: direction}
			res, err := poddb.loadFeedItems(tt.p.feedId, tt.p.numItems, opt)
			testutils.AssertErrContains(t, tt.e.errStr, err)

			// var wtflist = make([]*ItemDBEntry, 0)
			// wtf := gmock.mockdb.DB.Where(&ItemDBEntry{FeedId: tt.p.feedId}).Order(clause.OrderByColumn{Column: clause.Column{Name: "PubTimeStamp"}, Desc: true}).Find(&wtflist)
			// fmt.Printf("rows: %v, err: %v\n", wtf.RowsAffected, wtf.Error)

			// fmt.Printf("wtflist:\n")
			// for _, i := range wtflist {
			// 	fmt.Printf("\t(%v)(f:%v)'%v'\n", i.ID, i.FeedId, i.PubTimeStamp.Format(podutils.TimeFormatStr))
			// }
			// fmt.Printf("want:\n")
			// for _, i := range tt.e.resultList {
			// 	fmt.Printf("\t(%v)(f:%v)'%v'\n", i.ID, i.FeedId, i.PubTimeStamp.Format(podutils.TimeFormatStr))
			// }
			// fmt.Printf("got:\n")
			// for _, i := range res {
			// 	fmt.Printf("\t(%v)(f:%v)'%v'\n", i.ID, i.FeedId, i.PubTimeStamp.Format(podutils.TimeFormatStr))
			// }

			testutils.AssertEquals(t, tt.e.resultList, res)

			compareCallstack(t, tt.e.callStack)
		})
	}
}

func TestPodDB_loadItemXml(t *testing.T) {

	gmock, teardown := setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	var (
		item1, item2 = generateItem(1, true), generateItem(1, true)
		stdCallStack = []stackType{open, where, first}
	)

	item1.XmlData.Pubdate, item2.XmlData.Pubdate = time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, -2)

	if err := gmock.mockdb.DB.AutoMigrate(&ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*ItemDBEntry{&item1, &item2}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows\n", res.RowsAffected)
	}

	type args struct {
		emptyPath bool
		openErr   bool
		termErr   stackType
		xmlId     uint
	}
	type exp struct {
		xmlEntry  *ItemXmlDBEntry
		errStr    string
		callStack []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// error results
		{"empty path", args{emptyPath: true}, exp{errStr: "poddb is not initialized"}},
		{"feed id zero", args{}, exp{errStr: "xml id cannot be zero"}},
		{"open error", args{openErr: true, xmlId: item1.XmlId},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"first error", args{termErr: first, xmlId: item1.XmlId},
			exp{errStr: "first:foobar", callStack: stdCallStack}},
		{"not found", args{xmlId: 42}, exp{errStr: "record not found", callStack: stdCallStack}},

		// success
		{"item1", args{xmlId: item1.XmlId}, exp{xmlEntry: item1.XmlData, callStack: stdCallStack}},
		{"item2", args{xmlId: item2.XmlId}, exp{xmlEntry: item2.XmlData, callStack: stdCallStack}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			res, err := poddb.loadItemXml(tt.p.xmlId)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.AssertEquals(t, tt.e.xmlEntry, res)
			compareCallstack(t, tt.e.callStack)
		})
	}
}

func TestPodDB_saveFeed(t *testing.T) {

	gmock, teardown := setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	var (
		of1, of2                 = generateFeed(true), generateFeed(true)
		mf1, mf2                 FeedDBEntry
		oi1, oi2                 = generateItem(0, true), generateItem(0, true)
		ni3, ni4                 = generateItem(0, true), generateItem(0, true)
		mf1ItemList, mf2ItemList []*ItemDBEntry
	)

	of2.ItemList = []*ItemDBEntry{&oi1, &oi2}

	// do insert

	if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{}, &ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*FeedDBEntry{&of1, &of2}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows\n", res.RowsAffected)
	}

	var cloneXml = func(xml *FeedXmlDBEntry) *FeedXmlDBEntry {
		var FeedXmlDBEntry = *xml
		return &FeedXmlDBEntry
	}

	// make modifications.. make sure xml data has new references
	mf1 = of1
	mf1.XmlFeedData = cloneXml(of1.XmlFeedData)
	mf1.XmlFeedData.Description = testutils.RandStringBytes(8)
	mf1ItemList = []*ItemDBEntry{&ni3}

	mf2 = of2
	mf2.XmlFeedData = cloneXml(of2.XmlFeedData)
	mf2.XmlFeedData.Link = testutils.RandStringBytes(8)
	mf2ItemList = []*ItemDBEntry{&ni4}

	type args struct {
		emptyPath   bool
		nilFeed     bool
		modFeed     FeedDBEntry
		modItemList []*ItemDBEntry
		openErr     bool
		termErr     stackType
	}
	type exp struct {
		expFeed     FeedDBEntry
		expItemList []*ItemDBEntry
		errStr      string
		callStack   []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		//{"", args{}, exp{}},
		// error tests, no db changes
		{"empty path", args{emptyPath: true, modFeed: mf1},
			exp{errStr: "poddb is not initialized", expFeed: of1}},
		{"feed nil", args{nilFeed: true}, exp{errStr: "feed cannot be nil"}},
		{"feed id zero", args{modFeed: FeedDBEntry{}}, exp{errStr: "feed id is zero; make sure feed is created/loaded first"}},
		{"feed hash empty", args{modFeed: FeedDBEntry{PodDBModel: PodDBModel{ID: 3}}},
			exp{errStr: "hash cannot be empty"}},
		{"open error", args{openErr: true, modFeed: mf1},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"save error", args{termErr: save, modFeed: mf1},
			exp{errStr: "save:foobar", callStack: []stackType{open, session, save}}},

		// success tests
		{"success 1", args{modFeed: mf1, modItemList: mf1ItemList},
			exp{expFeed: mf1, expItemList: []*ItemDBEntry{&ni3}, callStack: []stackType{open, session, save}}},
		{"success 2", args{modFeed: mf2, modItemList: mf2ItemList},
			exp{expFeed: mf2, expItemList: []*ItemDBEntry{&oi1, &oi2, &ni4}, callStack: []stackType{open, session, save}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			tt.p.modFeed.ItemList = tt.p.modItemList

			err := poddb.saveFeed(podutils.Tern(tt.p.nilFeed, nil, &tt.p.modFeed))

			testutils.AssertErrContains(t, tt.e.errStr, err)
			compareCallstack(t, tt.e.callStack)

			// get the feed & items from the db, compare them to this
			if tt.e.expFeed.ID > 0 {

				var compEntry, dbEntry FeedDBEntry
				compEntry = tt.e.expFeed
				dbEntry.ID = tt.p.modFeed.ID
				res := gmock.mockdb.DB.Preload("XmlFeedData").Where(&dbEntry).First(&dbEntry)
				testutils.AssertErr(t, false, res.Error)
				// ignore model and itemlist..
				feedCompare(t, dbEntry, compEntry)

				// check dbItems
				var dbItems = make([]*ItemDBEntry, 0, len(tt.e.expItemList))
				res = gmock.mockdb.DB.Preload("XmlData").Where(&ItemDBEntry{FeedId: tt.p.modFeed.ID}).Find(&dbItems)
				testutils.AssertErr(t, false, res.Error)
				// direct comparision will fail; supply custom compare and use that
				testutils.AssertDiffFunc(t, tt.e.expItemList, dbItems, itemCompare)
			}
		})
	}
}

func TestPodDB_saveItems(t *testing.T) {

	type args struct {
		emptyPath   bool
		openErr     bool
		termErr     stackType
		missingFeed bool
		missingHash bool
		missingGuid bool
		modOrig     bool
		numInsert   int
	}
	type exp struct {
		errStr    string
		callStack []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		//{"", args{}, exp{}},
		// error tests, no db changes
		{"empty path", args{emptyPath: true, numInsert: rand.Intn(4) + 1},
			exp{errStr: "poddb is not initialized"}},
		{"open error", args{openErr: true, numInsert: rand.Intn(4) + 1},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"save error", args{termErr: save, numInsert: rand.Intn(4) + 1},
			exp{errStr: "save:foobar", callStack: []stackType{open, session, save}}},

		// list errors
		{"empty itemlist", args{numInsert: 0}, exp{errStr: "list is empty"}},
		{"missing feed id", args{missingFeed: true, numInsert: rand.Intn(4) + 1}, exp{errStr: "feed id is zero"}},
		{"missing hash", args{missingHash: true, numInsert: rand.Intn(4) + 1}, exp{errStr: "hash is empty"}},
		{"missing guid", args{missingGuid: true, numInsert: rand.Intn(4) + 1}, exp{errStr: "guid is empty"}},

		// success tests
		{"success single, new insert", args{numInsert: 1},
			exp{callStack: []stackType{open, session, save}}},
		{"success multiple, new insert", args{numInsert: 3},
			exp{callStack: []stackType{open, session, save}}},
		{"success only modify", args{modOrig: true, numInsert: 0},
			exp{callStack: []stackType{open, session, save}}},
		{"success modify and new", args{modOrig: true, numInsert: 1},
			exp{callStack: []stackType{open, session, save}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var (
				insertList, checkList []*ItemDBEntry
			)

			// setup db
			gmock, teardown := setupGormMock(t, nil, true)
			defer teardown(t, gmock)

			// insert original
			var orig1, orig2 = generateItem(1, true), generateItem(1, true)
			if err := gmock.mockdb.DB.AutoMigrate(&ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
				t.Fatalf("error in automigrate: %v", err)
			} else if res := gmock.mockdb.DB.Create([]*ItemDBEntry{&orig1, &orig2}); res.Error != nil {
				t.Fatalf("error in insert: %v", res.Error)
			} else {
				// fmt.Printf("inserted %v rows\n", res.RowsAffected)
				checkList = append(checkList, &orig1, &orig2)
			}

			// generate new inserts
			insertList = make([]*ItemDBEntry, 0, tt.p.numInsert)

			if tt.p.modOrig {
				// modify original, insert it into insert list
				var modIndex = rand.Intn(len(checkList))
				var mod = checkList[modIndex]

				mod.Filename = testutils.RandStringBytes(8)
				mod.Downloaded = podutils.Tern(rand.Intn(2) == 1, true, false)
				mod.XmlData.Title = testutils.RandStringBytes(8)
				insertList = append(insertList, mod)
			}

			for x := 0; x < tt.p.numInsert; x++ {
				var it = generateItem(1, true)

				if x == (tt.p.numInsert - 1) {
					if tt.p.missingFeed {
						it.FeedId = 0
					} else if tt.p.missingGuid {
						it.Guid = ""
					} else if tt.p.missingHash {
						it.Hash = ""
					}
				}

				insertList = append(insertList, &it)
				checkList = append(checkList, &it)
			}

			resetCallStack()

			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			err := poddb.saveItems(insertList...)

			if testutils.AssertErrContains(t, tt.e.errStr, err) {

				var dbItems = make([]*ItemDBEntry, 0, len(checkList))
				var res = gmock.mockdb.DB. /*Debug().*/ Preload("XmlData").Where(&ItemDBEntry{FeedId: 1}).Find(&dbItems)
				testutils.AssertErr(t, false, res.Error)
				// comparing including gorm model will fail; use compare method
				testutils.AssertDiffFunc(t, checkList, dbItems, itemCompare)
			}
			compareCallstack(t, tt.e.callStack)

		})
	}
}

func TestPodDB_deleteFeed(t *testing.T) {

	var defCallStack, noItemCallStack = []stackType{}, []stackType{}
	// item finding
	defCallStack = append(defCallStack, open, where, order, find)
	noItemCallStack = append(noItemCallStack, open, where, order, find)

	// item deletion (xml & item) (chunks will be 1)
	defCallStack = append(defCallStack, open, delete, delete)

	// feed xml & feed
	defCallStack = append(defCallStack, delete, delete)
	noItemCallStack = append(noItemCallStack, delete, delete)

	type args struct {
		emptyPath bool
		openErr   bool
		nilfeed   bool
		zeroId    bool
		termErr   stackType

		delIndex int
		noItems  bool
	}
	type exp struct {
		expDelIndex int
		errStr      string
		callStack   []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// error tests, no db changes
		{"empty path", args{emptyPath: true},
			exp{errStr: "poddb is not initialized", expDelIndex: -1}},
		{"open error", args{openErr: true},
			exp{errStr: "error opening db", expDelIndex: -1, callStack: []stackType{open}}},
		{"delete error", args{termErr: delete},
			exp{errStr: "delete:foobar", expDelIndex: -1, callStack: []stackType{open, where, order, find, open, delete}}},

		// list errors
		{"nil feed", args{nilfeed: true},
			exp{errStr: "feed cannot be nil", expDelIndex: -1}},
		{"zero id", args{zeroId: true},
			exp{errStr: "feed id cannot be zero", expDelIndex: -1}},

		// success tests
		{"delete first", args{delIndex: 0},
			exp{expDelIndex: 0, callStack: defCallStack}},
		{"delete second", args{delIndex: 1, noItems: true},
			exp{expDelIndex: 1, callStack: noItemCallStack}},
		{"delete third", args{delIndex: 2},
			exp{expDelIndex: 2, callStack: defCallStack}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			gmock, teardown := setupGormMock(t, nil, true)
			defer teardown(t, gmock)
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}
			resetCallStack()

			var feedList = []*FeedDBEntry{ptr(generateFeed(true)), ptr(generateFeed(true)), ptr(generateFeed(true))}
			for idx, feed := range feedList {
				if tt.p.noItems == false || idx != tt.p.delIndex {
					// generate items
					var numItems = rand.Intn(5) + 1
					feed.ItemList = make([]*ItemDBEntry, 0, numItems)
					for i := 0; i < numItems; i++ {
						feed.ItemList = append(feed.ItemList, ptr(generateItem(0, true)))
					}
				}
			}

			// insert shit, full assoc
			if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{},
				&ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
				t.Fatalf("error in automigrate: %v", err)
			} else if res := gmock.mockdb.DB. /*Debug().*/
								Session(&gorm.Session{FullSaveAssociations: true}).
								Create(feedList); res.Error != nil {
				t.Fatalf("error in insert: %v", res.Error)
			} else {
				fmt.Printf("inserted %v rows\n", res.RowsAffected)
			}

			// figure out which feed we're deleting, with error handling
			var deletefeed *FeedDBEntry
			if tt.p.nilfeed {
				deletefeed = nil
			} else if tt.p.zeroId {
				deletefeed = &FeedDBEntry{}
			} else {
				deletefeed = feedList[tt.p.delIndex]
			}

			// finally, delete
			err := poddb.deleteFeed(deletefeed)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			compareCallstack(t, tt.e.callStack)

			// check items in memory.. xml and feed should have DeletedAt entries
			for idx, feed := range feedList {
				var isDeleted = idx == tt.e.expDelIndex

				testutils.Assert(t, feed.DeletedAt.Time.IsZero() != isDeleted,
					fmt.Sprintf("DeletedAt.IsZero should be %v, got %v", isDeleted,
						feed.DeletedAt.Time.Format(podutils.TimeFormatStr)))
			}

			// check DB structure.. grab the ID of the expected deleted feed
			var (
				dbFeedList = make([]*FeedDBEntry, 0)
				expDelID   uint
				dbq        = gmock.mockdb.DB /*.Debug()*/
			)
			if tt.e.expDelIndex >= 0 {
				expDelID = feedList[tt.e.expDelIndex].ID
			} else {
				expDelID = 0
			}

			if res := dbq.Unscoped().Order("ID").Find(&dbFeedList); res.Error != nil {
				t.Fatalf("error in find: %v", res.Error)
			}

			for _, dbfeed := range dbFeedList {
				// grab xml, items, items xml

				var notDeleted = dbfeed.ID != expDelID
				dbfeed.ItemList = make([]*ItemDBEntry, 0)

				// check feed
				testutils.Assert(t, dbfeed.DeletedAt.Time.IsZero() == notDeleted,
					fmt.Sprintf("feed.DeletedAt.IsZero should be %v, got %v", notDeleted,
						dbfeed.DeletedAt.Time.Format(podutils.TimeFormatStr)))

				// check feed xml
				var feedXml FeedXmlDBEntry
				if res := dbq.Unscoped().First(&feedXml, dbfeed.XmlId); res.Error != nil {
					t.Fatalf("error in finding xml: %v", res.Error)
				} else {
					testutils.Assert(t, feedXml.DeletedAt.Time.IsZero() == notDeleted,
						fmt.Sprintf("xml.DeletedAt.IsZero should be %v, got %v", notDeleted,
							feedXml.DeletedAt.Time.Format(podutils.TimeFormatStr)))
				}

				// items
				var itemList = make([]*ItemDBEntry, 0)
				if res := dbq.Unscoped().Where("FeedId = ?", dbfeed.ID).Order("ID").Find(&itemList); res.Error != nil {
					t.Fatalf("error in finding items: %v", res.Error)
				} else {
					for _, item := range itemList {
						testutils.Assert(t, item.DeletedAt.Time.IsZero() == notDeleted,
							fmt.Sprintf("item.DeletedAt.IsZero should be %v, got %v", notDeleted,
								item.DeletedAt.Time.Format(podutils.TimeFormatStr)))

						// grab item xml
						var itemXml ItemXmlDBEntry
						if res := dbq.Unscoped().First(&itemXml, item.XmlId); res.Error != nil {
							t.Fatalf("error in finding items: %v", res.Error)
						} else {
							testutils.Assert(t, itemXml.DeletedAt.Time.IsZero() == notDeleted,
								fmt.Sprintf("item.DeletedAt.IsZero should be %v, got %v", notDeleted,
									itemXml.DeletedAt.Time.Format(podutils.TimeFormatStr)))
						}
					}
				}
			}

		})
	}
}

func TestPodDB_deleteItems(t *testing.T) {

	var (
		item1, item2 = generateItem(1, true), generateItem(1, true)
		item3, item4 = generateItem(1, true), generateItem(1, true)

		missingId    = item1
		defCallStack = []stackType{open, delete, delete}
	)

	missingId.ID = 0

	var cp = testutils.Cp[ItemDBEntry]

	type args struct {
		emptyPath   bool
		openErr     bool
		termErr     stackType
		missingId   bool
		notInserted bool
		delIndex    []int
	}
	type exp struct {
		expDelIndex []int
		errStr      string
		callStack   []stackType
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// error tests, no db changes
		{"empty path", args{emptyPath: true, delIndex: []int{1}},
			exp{errStr: "poddb is not initialized"}},
		{"open error", args{openErr: true, delIndex: []int{1}},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"delete error", args{termErr: delete, delIndex: []int{1}},
			exp{errStr: "delete:foobar", callStack: []stackType{open, delete}}},

		// list errors
		{"missing id", args{missingId: true, delIndex: []int{1}}, exp{errStr: "item missing ID"}},
		{"delete non-existant item", args{notInserted: true, delIndex: []int{}},
			exp{errStr: "", callStack: []stackType{open, delete, delete}}}, // will only warn in logs

		// success tests
		{"delete empty list", args{delIndex: []int{}}, exp{expDelIndex: []int{}}},
		{"delete one", args{delIndex: []int{2}},
			exp{expDelIndex: []int{2}, callStack: defCallStack}},
		{"delete two", args{delIndex: []int{1, 3}},
			exp{expDelIndex: []int{1, 3}, callStack: defCallStack}},
		{"delete all", args{delIndex: []int{0, 1, 2, 3}},
			exp{expDelIndex: []int{0, 1, 2, 3}, callStack: defCallStack}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gmock, teardown := setupGormMock(t, nil, true)
			defer teardown(t, gmock)

			// construct insert array
			var (
				insertlist = []*ItemDBEntry{cp(item1), cp(item2), cp(item3), cp(item4)}
				deletelist = make([]*ItemDBEntry, 0, len(tt.p.delIndex))

				resExistList = make([]*ItemDBEntry, 0, len(insertlist)-len(tt.e.expDelIndex))
				resDelList   = make([]*ItemDBEntry, 0, len(tt.e.expDelIndex))
			)

			if err := gmock.mockdb.DB.AutoMigrate(&ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
				t.Fatalf("error in automigrate: %v", err)
			} else if res := gmock.mockdb.DB.
				Session(&gorm.Session{FullSaveAssociations: true}).
				Create(insertlist); res.Error != nil {
				t.Fatalf("error in insert: %v", res.Error)
			} else {
				fmt.Printf("inserted %v rows\n", res.RowsAffected)
			}

			// populate delete list and end result list
			for idx, item := range insertlist {
				if slices.Contains(tt.p.delIndex, idx) {
					deletelist = append(deletelist, item)
				}

				if slices.Contains(tt.e.expDelIndex, idx) {
					resDelList = append(resDelList, item)
				} else {
					resExistList = append(resExistList, item)
				}

			}

			if tt.p.missingId {
				deletelist = append(deletelist, cp(missingId))
			} else if tt.p.notInserted {
				var newitem = cp(item4)
				newitem.ID = 42
				newitem.XmlId = 42
				deletelist = append(deletelist, newitem)
			}

			resetCallStack()

			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = []stackType{tt.p.termErr}

			err := poddb.deleteItems(deletelist)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			compareCallstack(t, tt.e.callStack)

			// check items in memory
			for _, item := range resExistList {
				testutils.Assert(t, item.DeletedAt.Time.IsZero(),
					fmt.Sprintf("exist item has non-zero deletedAt entry:\n\t%v\n\t%v", item, item.DeletedAt))
			}
			// future: deletedAt is not propegated with deleting based on IDs (as currently handled)
			// for _, item := range resDelList {
			// testutils.Assert(t, item.DeletedAt.Time.IsZero() == false,
			// 	fmt.Sprintf("deleted item has zero deletedAt entry:\n\t%v\n\t%v", item, item.DeletedAt))
			// }

			// check DB structure

			// check non-deleted
			var dbItems = make([]*ItemDBEntry, 0, len(resExistList))
			var res = gmock.mockdb.DB.Preload("XmlData").Find(&dbItems)
			testutils.AssertErr(t, false, res.Error)
			testutils.AssertDiffFunc(t, resExistList, dbItems, itemCompare)

			// check deleted
			var dbDelItems = make([]*ItemDBEntry, 0, len(deletelist))
			res = gmock.mockdb.DB.Unscoped().Where("DeletedAt not NULL").Find(&dbDelItems)
			testutils.AssertErr(t, false, res.Error)
			// because fucking preload doesn't work with unscoped
			for _, item := range dbDelItems {
				gmock.mockdb.DB.Unscoped().Where("ID = ? AND DeletedAt not NULL", item.XmlId).First(&item.XmlData)
			}
			testutils.AssertDiffFunc(t, resDelList, dbDelItems, itemCompare)

		})
	}
}

// helper functions
// --------------------------------------------------------------------------
func generateFeed(withXml bool) FeedDBEntry {
	var f FeedDBEntry
	f.Hash = testutils.RandStringBytes(8)
	if withXml {
		f.XmlFeedData = &FeedXmlDBEntry{}
		f.XmlFeedData.Title = testutils.RandStringBytes(8)
	}
	return f
}

// --------------------------------------------------------------------------
func generateItem(feedId uint, withXml bool) ItemDBEntry {
	var (
		i ItemDBEntry
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	)

	i.FeedId = feedId
	i.Hash = testutils.RandStringBytes(8)
	i.Filename = testutils.RandStringBytes(8)
	i.Guid = testutils.RandStringBytes(8)
	i.Downloaded = podutils.Tern(r.Intn(2) == 1, true, false)
	if withXml {
		i.XmlData = &ItemXmlDBEntry{}
		i.XmlData.Title = testutils.RandStringBytes(8)
		i.XmlData.Enclosure.Url = testutils.RandStringBytes(8)
		i.XmlData.Guid = i.Guid
	}
	return i
}

// --------------------------------------------------------------------------
func feedCompare(tb testing.TB, one, two FeedDBEntry) {
	tb.Helper()
	var ret = make([]string, 0)
	ret = append(ret, deep.Equal(one.Hash, two.Hash)...)
	ret = append(ret, deep.Equal(one.XmlId, two.XmlId)...)
	ret = append(ret, deep.Equal(one.XmlFeedData.ID, two.XmlFeedData.ID)...)
	ret = append(ret, deep.Equal(one.XmlFeedData.XChannelData, two.XmlFeedData.XChannelData)...)

	testutils.Assert(tb, len(ret) == 0, fmt.Sprintf("feed difference: %v", ret))
}

// --------------------------------------------------------------------------
func itemCompare(l, r *ItemDBEntry) bool {
	if l.Hash != r.Hash {
		return false
	} else if l.FeedId != r.FeedId {
		return false
	} else {
		var diff = deep.Equal(l.ItemData, r.ItemData)
		if len(diff) > 0 {
			return false
		}
		diff = deep.Equal(l.XmlId, r.XmlId)
		if len(diff) > 0 {
			return false
		}
		diff = deep.Equal(l.XmlData.XItemData, r.XmlData.XItemData)
		if len(diff) > 0 {
			return false
		}
	}

	return true
}

func ptr[T any](v T) *T {
	return &v
}
