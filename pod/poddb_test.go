package pod

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"gopod/testutils"
	"testing"

	"github.com/go-test/deep"
	"golang.org/x/exp/slices"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mostly integration tests

const inMemoryPath = "file::memory:?cache=shared"

type mockGorm struct {
	// todo: what do we need
	mockdb  *mockGormDB
	openErr bool
}

type mockGormDB struct {
	*gorm.DB

	autoMigrateErr   bool
	autoMigrateTypes []string
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

func (mgdb *mockGormDB) FirstOrCreate(dest any, conds ...any) *gorm.DB {
	appendCallstack(firstorcreate)
	return mgdb.DB.FirstOrCreate(dest, conds...)
}

// todo: replace these
func (mgdb *mockGormDB) Where(query any, args ...any) gormDBInterface {
	appendCallstack(where)
	var newdb = mockGormDB{
		DB: mgdb.DB.Where(query, args...),
	}
	return &newdb
}
func (mgdb *mockGormDB) Preload(query string, args ...any) gormDBInterface {
	appendCallstack(preload)
	var newdb = mockGormDB{
		DB: mgdb.DB.Preload(query, args...),
	}
	return &newdb
}
func (mgdb *mockGormDB) Order(value any) gormDBInterface {
	appendCallstack(order)
	var newdb = mockGormDB{
		DB: mgdb.DB.Order(value),
	}
	return &newdb
}
func (mgdb *mockGormDB) Limit(lim int) gormDBInterface {
	appendCallstack(limit)
	var newdb = mockGormDB{
		DB: mgdb.DB.Limit(lim),
	}
	return &newdb
}
func (mgdb *mockGormDB) Session(config *gorm.Session) gormDBInterface {
	appendCallstack(session)
	var newdb = mockGormDB{
		DB: mgdb.DB.Session(config),
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
	firstorcreate stackType = "db.firstorcreate"
)

var callStack []stackType

func resetCallStack()             { callStack = make([]stackType, 0) }
func appendCallstack(s stackType) { callStack = append(callStack, s) }
func compareCallstack(tb testing.TB, exp []stackType) {
	tb.Helper()
	if diffs := deep.Equal(exp, callStack); diffs != nil {
		str := "\033[31m\nObjects not equal:\033[39m\n"
		for _, d := range diffs {
			str += fmt.Sprintf("\033[31m\t%v\033[39m\n", d)
		}
		tb.Error(str)
	}
}

func setupMock(t *testing.T, mock *mockGorm, openDB bool) (*mockGorm, func(*testing.T, *mockGorm)) {
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

			var gmock, teardown = setupMock(t, &tt.p.mock, false)
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
				var missing, extra = testutils.ListDiff(autoMigrageTypes, gmock.mockdb.autoMigrateTypes)
				testutils.Assert(t, len(missing) == 0, fmt.Sprintf("Missing types in automigrate: %v", missing))
				testutils.Assert(t, len(extra) == 0, fmt.Sprintf("Extra types in automigrate: %v", extra))
			}
		})
	}
}

func TestPodDB_loadDBFeed(t *testing.T) {

	var gmock, teardown = setupMock(t, nil, true)
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
	//gmock.mockdb.DB.
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

			resetCallStack()
			var fq = tt.p.fq

			err := poddb.loadDBFeed(podutils.Tern(tt.p.entryNil, nil, &fq), tt.p.loadXml)
			if testutils.AssertErrContains(t, tt.e.errStr, err) {
				// general insert structure
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
			var expCallStack = podutils.Tern(tt.e.callStack != nil, tt.e.callStack, make([]stackType, 0))
			compareCallstack(t, expCallStack)
		})
	}
}

/*
func TestPodDB_loadDBFeedXml(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pdb.loadDBFeedXml(tt.args.feedXml); (err != nil) != tt.wantErr {
				t.Errorf("PodDB.loadDBFeedXml() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/ /*
func TestPodDB_loadFeedItems(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.pdb.loadFeedItems(tt.args.feedId, tt.args.numItems, tt.args.includeXml)
			if (err != nil) != tt.wantErr {
				t.Errorf("PodDB.loadFeedItems() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PodDB.loadFeedItems() = %v, want %v", got, tt.want)
			}
		})
	}
}
*/ /*
func TestPodDB_saveFeed(t *testing.T) {
	type args struct {
	}
	type exp struct {
	}
	tests := []struct {
		name string
		p    args
		e    exp
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.pdb.saveFeed(tt.args.feed); (err != nil) != tt.wantErr {
				t.Errorf("PodDB.saveFeed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/
