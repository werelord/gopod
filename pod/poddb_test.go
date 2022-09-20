package pod

import (
	"errors"
	"fmt"
	"gopod/testutils"
	"testing"

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
	autoMigrateCalled bool
	autoMigrateErr    bool

	// checks on autoMigrate types
	feed, feedxml, item, itemxml bool
	unknownList                  string
}

func (mg *mockGorm) Open(d gorm.Dialector, opts ...gorm.Option) (gormDBInterface, error) {
	fmt.Print("MockGorm.Open()")

	if mg.openErr {
		return nil, errors.New("foobar")
	}
	if mg.mockdb.DB == nil {
		db, err := gorm.Open(d, opts...)
		mg.mockdb.DB = db
		return mg.mockdb, err
	}
	return mg.mockdb, nil
}

func (mgdb *mockGormDB) AutoMigrate(dst ...any) error {
	// todo: check dst for expected interface objects
	fmt.Print("MockGormDB.AutoMigrate()")
	mgdb.autoMigrateCalled = true

	for _, model := range dst {
		switch t := model.(type) {
		case *FeedDBEntry:
			mgdb.feed = true
		case *FeedXmlDBEntry:
			mgdb.feedxml = true
		case *ItemDBEntry:
			mgdb.item = true
		case *ItemXmlDBEntry:
			mgdb.itemxml = true
		default:
			mgdb.unknownList += fmt.Sprintf("'%T',", t)
		}
	}
	if mgdb.autoMigrateErr {
		return errors.New("automigrate:foobar")
	} else {
		return nil
	}
}

// todo: replace these
func (mgdb *mockGormDB) Where(query any, args ...any) *gormDBImpl {
	return &gormDBImpl{mgdb.DB.Where(query, args...)}
}
func (mgdb *mockGormDB) Preload(query string, args ...any) *gormDBImpl {
	return &gormDBImpl{mgdb.DB.Preload(query, args...)}
}
func (mgdb *mockGormDB) Order(value any) *gormDBImpl { return &gormDBImpl{mgdb.DB.Order(value)} }
func (mgdb *mockGormDB) Limit(limit int) *gormDBImpl { return &gormDBImpl{mgdb.DB.Limit(limit)} }
func (mgdb *mockGormDB) Session(config *gorm.Session) *gormDBImpl {
	return &gormDBImpl{mgdb.DB.Session(config)}
}

func setupMock(t *testing.T, mock *mockGorm, openDB bool) (*mockGorm, func(*testing.T, *mockGorm)) {

	var oldGorm = gImpl
	gImpl = mock
	fmt.Printf("setupTest(%v)", t.Name())

	if openDB {
		// todo: open db here, for outside test handling
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

			got, err := NewDB(tt.p.path)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, (got == nil) == tt.e.dbNil, fmt.Sprintf("expected db nil, got %v", got))
			testutils.Assert(t, tt.e.autoMigrateCalled == gmock.mockdb.autoMigrateCalled,
				fmt.Sprintf("automigrate called expect: %v, got %v", tt.e.autoMigrateCalled, gmock.mockdb.autoMigrateCalled))
			if gmock.mockdb.autoMigrateCalled {
				// check types on automigrate
				testutils.Assert(t, gmock.mockdb.feed == true, "FeedDBEntry missing from automigrate")
				testutils.Assert(t, gmock.mockdb.feedxml == true, "FeedXmlDBEntry missing from automigrate")
				testutils.Assert(t, gmock.mockdb.item == true, "ItemDBEntry missing from automigrate")
				testutils.Assert(t, gmock.mockdb.itemxml == true, "ItemXmlDBEntry missing from automigrate")
				testutils.Assert(t, gmock.mockdb.unknownList == "",
					fmt.Sprintf("Unknown item added to automigrate: %v", gmock.mockdb.unknownList))
			}

		})
	}
}

/*
func TestPodDB_loadDBFeed(t *testing.T) {
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
			if err := tt.pdb.loadDBFeed(tt.args.feedEntry, tt.args.loadXml); (err != nil) != tt.wantErr {
				t.Errorf("PodDB.loadDBFeed() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/ /*
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
