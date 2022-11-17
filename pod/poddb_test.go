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

	"github.com/glebarez/sqlite"
	//"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mostly integration tests

// path changed using pure go sqlite driver
// const inMemoryPath = "file::memory:?cache=shared"
const inMemoryPath = ":memory:"

type mockGorm struct {
	// todo: what do we need
	mockdb  *mockGormDB
	openErr bool
}

type mockGormDB struct {
	*gorm.DB

	autoMigrateErr   bool
	autoMigrateTypes []string

	termErr bool
}

func (mg *mockGorm) Open(d gorm.Dialector, opts ...gorm.Option) (gormDBInterface, error) {
	fmt.Print("MockGorm.Open()")
	defer func() { fmt.Print("\n") }()
	appendCallstack(open)

	if mg.openErr {
		fmt.Print(", returning error")
		return nil, errors.New("foobar")
	}
	// make sure call list is reset

	var (
		err error
		db  *gorm.DB
	)
	if mg.mockdb.DB == nil {
		fmt.Print(", opening new instance")
		db, err = gorm.Open(d, opts...)
		mg.mockdb.DB = db
	} else {
		fmt.Print(", returning already opened")
	}

	return mg.mockdb, err
}

func (mgdb *mockGormDB) AutoMigrate(dst ...any) error {
	fmt.Print("MockGormDB.AutoMigrate()\n")
	appendCallstack(automigrate)

	mgdb.autoMigrateTypes = testutils.ListTypes(dst...)

	if mgdb.autoMigrateErr {
		return errors.New("automigrate:foobar")
	} else {
		return nil
	}
}

// terminal method calls
func (mgdb *mockGormDB) FirstOrCreate(dest any, conds ...any) *gorm.DB {
	appendCallstack(firstorcreate)
	if mgdb.termErr {
		return &gorm.DB{Error: errors.New("firstorcreate:foobar")}
	} else {
		return mgdb.DB.FirstOrCreate(dest, conds...)
	}
}
func (mgdb *mockGormDB) First(dest any, conds ...any) *gorm.DB {
	appendCallstack(first)
	if mgdb.termErr {
		return &gorm.DB{Error: errors.New("first:foobar")}
	} else {
		return mgdb.DB.First(dest, conds...)
	}
}
func (mgdb *mockGormDB) Find(dest any, conds ...any) *gorm.DB {
	appendCallstack(find)
	if mgdb.termErr {
		return &gorm.DB{Error: errors.New("find:foobar")}
	} else {
		return mgdb.DB.Find(dest, conds...)
	}
}
func (mgdb *mockGormDB) Save(value any) *gorm.DB {
	appendCallstack(save)
	if mgdb.termErr {
		return &gorm.DB{Error: errors.New("save:foobar")}
	} else {
		return mgdb.DB.Save(value)
	}
}
func (mgdb *mockGormDB) Delete(value any, conds ...any) *gorm.DB {
	appendCallstack(delete)
	if mgdb.termErr {
		return &gorm.DB{Error: errors.New("delete:foobar")}
	} else {
		return mgdb.DB.Delete(value, conds...)
	}
}

// continuaetion method calls
func (mgdb *mockGormDB) Where(query any, args ...any) gormDBInterface {
	appendCallstack(where)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Where(query, args...),
	}
	return &newdb
}
func (mgdb *mockGormDB) Preload(query string, args ...any) gormDBInterface {
	appendCallstack(preload)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Preload(query, args...),
	}
	return &newdb
}
func (mgdb *mockGormDB) Order(value any) gormDBInterface {
	appendCallstack(order)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Order(value),
	}
	return &newdb
}
func (mgdb *mockGormDB) Limit(lim int) gormDBInterface {
	appendCallstack(limit)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Limit(lim),
	}
	return &newdb
}
func (mgdb *mockGormDB) Session(config *gorm.Session) gormDBInterface {
	appendCallstack(session)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Session(config),
	}
	return &newdb
}
func (mgdb *mockGormDB) Debug() gormDBInterface {
	// not logging this in callstack.. fuck that
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Debug(),
	}
	return &newdb
}
func (mgdb *mockGormDB) Unscoped() gormDBInterface {
	appendCallstack(unscoped)
	var newdb = mockGormDB{
		DB: mgdb.DB.Unscoped(),
	}
	return &newdb
}

type stackType string

const (
	open          stackType = "g.open"
	automigrate   stackType = "db.automigrate"
	where         stackType = "db.where"
	preload       stackType = "db.preload"
	order         stackType = "db.order"
	limit         stackType = "db.limit"
	session       stackType = "db.session"
	unscoped      stackType = "db.unscoped"
	firstorcreate stackType = "db.firstorcreate"
	first         stackType = "db.first"
	find          stackType = "db.find"
	save          stackType = "db.save"
	delete        stackType = "db.delete"
)

var callStack []stackType

func resetCallStack()             { callStack = make([]stackType, 0) }
func appendCallstack(s stackType) { callStack = append(callStack, s) }
func compareCallstack(tb testing.TB, exp []stackType) {
	tb.Helper()

	// make sure we're not comparing possible empty slice to nil slice
	if exp == nil {
		exp = make([]stackType, 0)
	}

	if diffs := deep.Equal(exp, callStack); diffs != nil {
		str := "\033[31m\nObjects not equal:\033[39m\n"
		for _, d := range diffs {
			str += fmt.Sprintf("\033[31m\t%v\033[39m\n", d)
		}
		tb.Error(str)
	}
}

func setupGormMock(t *testing.T, mock *mockGorm, openDB bool) (*mockGorm, func(*testing.T, *mockGorm)) {
	if mock == nil {
		mock = &mockGorm{}
	}

	var oldGorm = gImpl
	gImpl = mock
	fmt.Printf("setupTest(%v)", t.Name())

	if openDB {
		db, err := gorm.Open(sqlite.Open(inMemoryPath), &defaultConfig)
		if err != nil {
			t.Fatalf("open db failed: %v", err)
		}
		mock.mockdb = &mockGormDB{DB: db}
	}
	fmt.Print("\n")

	return mock, func(t *testing.T, mg *mockGorm) {
		fmt.Printf("\nTeardown(%v)\n", t.Name())
		// don't need to call close here...
		// todo: anything else here??
		gImpl = oldGorm
	}
}

func TestNewDB(t *testing.T) {

	var autoMigrageTypes = testutils.ListTypes(&FeedDBEntry{}, &FeedXmlDBEntry{}, &ItemDBEntry{}, &ItemXmlDBEntry{})

	type arg struct {
		path string
		mock mockGorm
	}
	type exp struct {
		autoMigrateCalled bool
		dbNil             bool
		errStr            string
	}
	tests := []struct {
		name string
		p    arg
		e    exp
	}{
		{"empty path", arg{path: "", mock: mockGorm{mockdb: &mockGormDB{}}},
			exp{autoMigrateCalled: false, dbNil: true, errStr: "db path cannot be empty"}},
		{"open error", arg{path: inMemoryPath, mock: mockGorm{openErr: true, mockdb: &mockGormDB{}}},
			exp{autoMigrateCalled: false, dbNil: true, errStr: "error opening db: foobar"}},
		{"automigrate error", arg{path: inMemoryPath, mock: mockGorm{mockdb: &mockGormDB{autoMigrateErr: true}}},
			exp{autoMigrateCalled: true, dbNil: true, errStr: "automigrate:foobar"}},
		{"success", arg{path: inMemoryPath, mock: mockGorm{mockdb: &mockGormDB{}}},
			exp{autoMigrateCalled: true, dbNil: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var gmock, teardown = setupGormMock(t, &tt.p.mock, false)
			defer teardown(t, gmock)

			resetCallStack()

			got, err := NewDB(tt.p.path)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, (got == nil) == tt.e.dbNil, fmt.Sprintf("expected db nil, got %v", got))

			var amCalled = slices.Contains(callStack, automigrate)
			testutils.Assert(t, tt.e.autoMigrateCalled == amCalled,
				fmt.Sprintf("automigrate called expect: %v, got %v", tt.e.autoMigrateCalled, amCalled))

			if tt.e.autoMigrateCalled {
				testutils.Assert(t, len(autoMigrageTypes) == len(gmock.mockdb.autoMigrateTypes),
					fmt.Sprintf("automigrate types len() exp %v, got %v",
						len(autoMigrageTypes), len(gmock.mockdb.autoMigrateTypes)))

				// check types on automigrate
				testutils.AssertDiff(t, autoMigrageTypes, gmock.mockdb.autoMigrateTypes)
			}
		})
	}
}

func TestPodDB_loadDBFeed(t *testing.T) {

	var gmock, teardown = setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	// insert shit here, for retrieval
	var (
		entryOne, entryTwo, newEntry FeedDBEntry
		entryOneNoXml, entryTwoNoXml FeedDBEntry

		hashQuery, idQuery FeedDBEntry
	)

	entryOne.Hash = "foobar"
	entryOne.XmlFeedData.Title = "barFoo"
	entryTwo.Hash = "armleg"
	entryTwo.XmlFeedData.Title = "armleg"
	newEntry.Hash = "bahmeh"
	newEntry.XmlFeedData.Link = "https://foo.bar/example"

	if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*FeedDBEntry{&entryOne, &entryTwo}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows, ids: %v %v\n", res.RowsAffected, entryOne.ID, entryTwo.ID)
	}
	hashQuery.Hash = entryOne.Hash
	entryOneNoXml = entryOne
	entryOneNoXml.XmlFeedData = FeedXmlDBEntry{}

	idQuery.ID = entryTwo.ID
	entryTwoNoXml = entryTwo
	entryTwoNoXml.XmlFeedData = FeedXmlDBEntry{}

	var (
		defCS        = []stackType{open, where, firstorcreate}
		defCSWithXml = []stackType{open, where, preload, firstorcreate}
	)

	type args struct {
		emptyPath bool
		openErr   bool
		entryNil  bool
		termErr   bool
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
		{"firstOrCreate error", args{termErr: true, fq: hashQuery},
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
			gmock.mockdb.termErr = tt.p.termErr

			resetCallStack()
			var fq = tt.p.fq

			err := poddb.loadDBFeed(podutils.Tern(tt.p.entryNil, nil, &fq), tt.p.loadXml)
			if testutils.AssertErrContains(t, tt.e.errStr, err) {
				// general expected structure
				testutils.Assert(t, fq.ID > 0, fmt.Sprintf("Id > 0, got %v", fq.ID))
				if tt.p.loadXml {
					testutils.Assert(t, fq.XmlFeedData.ID > 0,
						fmt.Sprintf("Xml ID > 0, got %v", fq.XmlFeedData.ID))
					testutils.Assert(t, fq.XmlFeedData.FeedId == fq.ID,
						fmt.Sprintf("Xml Feed Id should be %v, got %v", fq.ID, fq.XmlFeedData.ID))
				} else {
					testutils.Assert(t, fq.XmlFeedData.ID == 0,
						fmt.Sprintf("Xml ID == 0, got %v", fq.XmlFeedData.ID))
				}

				var exp = tt.e.fe
				if tt.e.useDB {
					// pull the entry from the db, compare
					var dbEntry FeedDBEntry
					dbEntry.ID = fq.ID
					if res := gmock.mockdb.DB.Preload("XmlFeedData").Find(&dbEntry); res.Error != nil {
						t.Error("error in checking: ", err)
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
		entryOne, entryTwo         FeedXmlDBEntry
		qWithId, qWithFeedId       FeedXmlDBEntry
		notFoundId, notfoundFeedId FeedXmlDBEntry
	)

	entryOne.Title = "foobar"
	entryOne.Author = "meh"

	entryTwo.Title = "barfoo"
	entryTwo.FeedId = 42
	entryTwo.PubDate = time.Now()

	if err := gmock.mockdb.DB.AutoMigrate(&FeedDBEntry{}, &FeedXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*FeedXmlDBEntry{&entryOne, &entryTwo}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows, ids: %v %v\n", res.RowsAffected, entryOne.ID, entryTwo.ID)
	}

	qWithId.ID = entryOne.ID
	qWithFeedId.FeedId = entryTwo.FeedId
	notFoundId.ID = 99
	notfoundFeedId.FeedId = 13

	var expCS = []stackType{open, where, first}

	type args struct {
		emptyPath bool
		openErr   bool
		entryNil  bool
		termErr   bool
		fxq       FeedXmlDBEntry
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
		{"empty path", args{emptyPath: true, fxq: qWithId}, exp{errStr: "poddb is not initialized"}},
		{"entry nil", args{entryNil: true}, exp{errStr: "feedxml cannot be nil"}},
		{"missing id+hash", args{}, exp{errStr: "xmlID or feedID cannot be zero"}},
		{"open error", args{openErr: true, fxq: qWithId},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"generic find error", args{termErr: true, fxq: qWithId},
			exp{errStr: "first:foobar", callStack: expCS}},

		// not existing tests
		{"id doesn't exist", args{fxq: notFoundId},
			exp{errStr: "record not found", callStack: expCS}},
		{"feedid doesn't exist", args{fxq: notfoundFeedId},
			exp{errStr: "record not found", callStack: expCS}},

		// success
		{"success by id", args{fxq: qWithId}, exp{fxe: entryOne, callStack: expCS}},
		{"success by feedid", args{fxq: qWithFeedId}, exp{fxe: entryTwo, callStack: expCS}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = tt.p.termErr

			var fxq = tt.p.fxq

			var err = poddb.loadDBFeedXml(podutils.Tern(tt.p.entryNil, nil, &fxq))
			if testutils.AssertErrContains(t, tt.e.errStr, err) {
				testutils.Assert(t, fxq.ID > 0, fmt.Sprintf("Id > 0, got %v", fxq.ID))

				// compare objects
				testutils.AssertEquals(t, tt.e.fxe, fxq)
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
		return &ItemDBEntry{e.PodDBModel, e.Hash, e.FeedId, e.ItemData, ItemXmlDBEntry{}}
	}

	type args struct {
		emptyPath  bool
		openErr    bool
		termErr    bool
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
		{"find error", args{termErr: true, feedId: 2},
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
			gmock.mockdb.termErr = tt.p.termErr

			var direction = podutils.Tern(tt.p.asc == true, cASC, cDESC)

			res, err := poddb.loadFeedItems(tt.p.feedId, tt.p.numItems, tt.p.includeXml, direction)
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
	} else if res := gmock.mockdb.DB.Create(
		[]*ItemDBEntry{&item1, &item2}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows\n", res.RowsAffected)
	}

	type args struct {
		emptyPath bool
		openErr   bool
		termErr   bool
		itemId    uint
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
		{"feed id zero", args{}, exp{errStr: "feed id cannot be zero"}},
		{"open error", args{openErr: true, itemId: item1.ID},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"first error", args{termErr: true, itemId: item1.ID},
			exp{errStr: "first:foobar", callStack: stdCallStack}},
		{"not found", args{itemId: 42}, exp{errStr: "record not found", callStack: stdCallStack}},

		// success
		{"item1", args{itemId: item1.ID}, exp{xmlEntry: &item1.XmlData, callStack: stdCallStack}},
		{"item2", args{itemId: item2.ID}, exp{xmlEntry: &item2.XmlData, callStack: stdCallStack}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = tt.p.termErr

			res, err := poddb.loadItemXml(tt.p.itemId)

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

	// make modifications
	mf1 = of1
	mf1.XmlFeedData.Description = testutils.RandStringBytes(8)
	mf1ItemList = []*ItemDBEntry{&ni3}
	mf2 = of2
	mf2.XmlFeedData.Link = testutils.RandStringBytes(8)
	mf2ItemList = []*ItemDBEntry{&ni4}

	type args struct {
		emptyPath   bool
		nilFeed     bool
		modFeed     FeedDBEntry
		modItemList []*ItemDBEntry
		openErr     bool
		termErr     bool
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
		{"save error", args{termErr: true, modFeed: mf1},
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
			gmock.mockdb.termErr = tt.p.termErr

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
				res = gmock.mockdb.DB.Debug().Preload("XmlData").Where(&ItemDBEntry{FeedId: tt.p.modFeed.ID}).Find(&dbItems)
				testutils.AssertErr(t, false, res.Error)
				// direct comparision will fail; supply custom compare and use that
				testutils.AssertDiffFunc(t, tt.e.expItemList, dbItems, itemCompare)
			}
		})
	}
}

func TestPodDB_saveItems(t *testing.T) {

	gmock, teardown := setupGormMock(t, nil, true)
	defer teardown(t, gmock)

	var (
		orig1, orig2 = generateItem(1, true), generateItem(1, true)
		item1, item2 = generateItem(1, true), generateItem(1, true)

		missingFeedId = generateItem(0, true)
		missingHash   = generateItem(1, true)
		missingGuid   = generateItem(1, true)

		defaultInsert = []*ItemDBEntry{&item1, &item2}
	)

	missingHash.Hash = ""
	missingGuid.Guid = ""

	// insert original
	if err := gmock.mockdb.DB.AutoMigrate(&ItemDBEntry{}, &ItemXmlDBEntry{}); err != nil {
		t.Fatalf("error in automigrate: %v", err)
	} else if res := gmock.mockdb.DB.Create([]*ItemDBEntry{&orig1, &orig2}); res.Error != nil {
		t.Fatalf("error in insert: %v", res.Error)
	} else {
		fmt.Printf("inserted %v rows\n", res.RowsAffected)
	}

	type args struct {
		emptyPath  bool
		openErr    bool
		termErr    bool
		insertList []*ItemDBEntry
	}
	type exp struct {
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
		{"empty path", args{emptyPath: true, insertList: defaultInsert},
			exp{errStr: "poddb is not initialized"}},
		{"open error", args{openErr: true, insertList: defaultInsert},
			exp{errStr: "error opening db", callStack: []stackType{open}}},
		{"save error", args{termErr: true, insertList: defaultInsert},
			exp{errStr: "save:foobar", callStack: []stackType{open, session, save}}},

		// list errors
		{"nil itemlist", args{insertList: nil}, exp{errStr: "list is empty"}},
		{"empty itemlist", args{insertList: []*ItemDBEntry{}}, exp{errStr: "list is empty"}},
		{"first missing feed id", args{insertList: []*ItemDBEntry{&missingFeedId, &item1}}, exp{errStr: "feed id is zero"}},
		{"second missing feed id", args{insertList: []*ItemDBEntry{&item1, &missingFeedId}}, exp{errStr: "feed id is zero"}},
		{"first missing hash", args{insertList: []*ItemDBEntry{&missingHash, &item1}}, exp{errStr: "hash is empty"}},
		{"second missing hash", args{insertList: []*ItemDBEntry{&item1, &missingHash}}, exp{errStr: "hash is empty"}},
		{"first missing guid", args{insertList: []*ItemDBEntry{&missingGuid, &item1}}, exp{errStr: "guid is empty"}},
		{"second missing guid", args{insertList: []*ItemDBEntry{&item1, &missingGuid}}, exp{errStr: "guid is empty"}},

		// success tests
		{"success", args{insertList: defaultInsert},
			exp{expItemList: []*ItemDBEntry{&orig1, &orig2, &item1, &item2}, callStack: []stackType{open, session, save}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetCallStack()
			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = tt.p.termErr

			err := poddb.saveItems(tt.p.insertList)

			if testutils.AssertErrContains(t, tt.e.errStr, err) {

				var dbItems = make([]*ItemDBEntry, 0, len(tt.e.expItemList))
				var res = gmock.mockdb.DB. /*Debug().*/ Preload("XmlData").Where(&ItemDBEntry{FeedId: 1}).Find(&dbItems)
				testutils.AssertErr(t, false, res.Error)
				// comparing including gorm model will fail; use compare method
				testutils.AssertDiffFunc(t, tt.e.expItemList, dbItems, itemCompare)
			}
			compareCallstack(t, tt.e.callStack)
		})
	}
}

func TestPodDB_deleteItems(t *testing.T) {

	var (
		item1, item2 = generateItem(1, true), generateItem(1, true)
		item3, item4 = generateItem(1, true), generateItem(1, true)

		missingId    = item1
		defCallStack = []stackType{open, delete, where, delete}

		dupCallStack = func(num int) []stackType {
			var ret = make([]stackType, 0, num*len(defCallStack))
			// first, including open
			ret = append(ret, defCallStack...)
			// append extra calls based on list
			for x := 1; x < num; x++ {
				ret = append(ret, delete, where, delete)
			}
			return ret
		}
	)

	missingId.ID = 0

	var cp = testutils.Cp[ItemDBEntry]

	type args struct {
		emptyPath   bool
		openErr     bool
		termErr     bool
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
		{"delete error", args{termErr: true, delIndex: []int{1}},
			exp{errStr: "delete:foobar", callStack: []stackType{open, delete}}},

		// list errors
		{"missing id", args{missingId: true, delIndex: []int{1}}, exp{errStr: "item missing ID"}},
		{"delete non-existant item", args{notInserted: true, delIndex: []int{}},
			exp{errStr: "might not exist", callStack: []stackType{open, delete}}},

		// success tests
		{"delete one", args{delIndex: []int{2}},
			exp{expDelIndex: []int{2}, callStack: defCallStack}},
		{"delete two", args{delIndex: []int{1, 3}},
			exp{expDelIndex: []int{1, 3}, callStack: dupCallStack(2)}},
		{"delete all", args{delIndex: []int{0, 1, 2, 3}},
			exp{expDelIndex: []int{0, 1, 2, 3}, callStack: dupCallStack(4)}},
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
				deletelist = append(deletelist, newitem)
			}

			resetCallStack()

			var poddb = PodDB{path: podutils.Tern(tt.p.emptyPath, "", inMemoryPath)}
			gmock.openErr = tt.p.openErr
			gmock.mockdb.termErr = tt.p.termErr

			err := poddb.deleteItems(deletelist)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			compareCallstack(t, tt.e.callStack)

			// check items in memory
			for _, item := range resExistList {
				testutils.Assert(t, item.DeletedAt.Time.IsZero(),
					fmt.Sprintf("exist item has non-zero deletedAt entry:\n\t%v\n\t%v", item, item.DeletedAt))
			}
			for _, item := range resDelList {
				testutils.Assert(t, item.DeletedAt.Time.IsZero() == false,
					fmt.Sprintf("deleted item has zero deletedAt entry:\n\t%v\n\t%v", item, item.DeletedAt))
			}

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
				gmock.mockdb.DB.Unscoped().Where("ItemId = ? AND DeletedAt not NULL", item.ID).First(&item.XmlData)
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
	ret = append(ret, deep.Equal(one.XmlFeedData.FeedId, two.XmlFeedData.FeedId)...)
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
		diff = deep.Equal(l.XmlData.ItemId, r.XmlData.ItemId)
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
