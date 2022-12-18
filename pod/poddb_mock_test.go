package pod

import (
	"errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/go-test/deep"
	"golang.org/x/exp/slices"
	"gorm.io/gorm"
)

// path changed using pure go sqlite driver
// const inMemoryPath = "file::memory:?cache=shared"
const inMemoryPath = ":memory:"

type mockGorm struct {
	mockdb  *mockGormDB
	openErr bool
}

type mockGormDB struct {
	*gorm.DB

	// autoMigrateErr   bool
	// autoMigrateTypes []string

	termErr []stackType
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
	//fmt.Print("MockGormDB.AutoMigrate()\n")
	appendCallstack(automigrate)

	// mgdb.autoMigrateTypes = testutils.ListTypes(dst...)

	// if mgdb.autoMigrateErr {
	return errors.New("automigrate shouldn't be called")
	// } else {
	// return nil
	// }
}

// terminal method calls
func (mgdb *mockGormDB) FirstOrCreate(dest any, conds ...any) *gorm.DB {
	appendCallstack(firstorcreate)
	if slices.Contains(mgdb.termErr, firstorcreate) {
		return &gorm.DB{Error: errors.New("firstorcreate:foobar")}
	} else {
		return mgdb.DB.FirstOrCreate(dest, conds...)
	}
}
func (mgdb *mockGormDB) First(dest any, conds ...any) *gorm.DB {
	appendCallstack(first)
	if slices.Contains(mgdb.termErr, first) {
		return &gorm.DB{Error: errors.New("first:foobar")}
	} else {
		return mgdb.DB.First(dest, conds...)
	}
}
func (mgdb *mockGormDB) Find(dest any, conds ...any) *gorm.DB {
	appendCallstack(find)
	if slices.Contains(mgdb.termErr, find) {
		return &gorm.DB{Error: errors.New("find:foobar")}
	} else {
		return mgdb.DB.Find(dest, conds...)
	}
}
func (mgdb *mockGormDB) Save(value any) *gorm.DB {
	appendCallstack(save)
	if slices.Contains(mgdb.termErr, save) {
		return &gorm.DB{Error: errors.New("save:foobar")}
	} else {
		return mgdb.DB.Save(value)
	}
}
func (mgdb *mockGormDB) Delete(value any, conds ...any) *gorm.DB {
	appendCallstack(delete)
	if slices.Contains(mgdb.termErr, delete) {
		return &gorm.DB{Error: errors.New("delete:foobar")}
	} else {
		return mgdb.DB.Delete(value, conds...)
	}
}
func (mgdb *mockGormDB) Count(c *int64) *gorm.DB {
	appendCallstack(count)
	if slices.Contains(mgdb.termErr, count) {
		return &gorm.DB{Error: errors.New("count:foobar")}
	} else {
		return mgdb.DB.Count(c)
	}
}
func (mgdb *mockGormDB) Scan(dest any) *gorm.DB {
	appendCallstack(scan)
	if slices.Contains(mgdb.termErr, scan) {
		return &gorm.DB{Error: errors.New("scan:foobar")}
	} else {
		return mgdb.DB.Scan(dest)
	}
}
func (mgdb *mockGormDB) Exec(sql string, values ...any) *gorm.DB {
	appendCallstack(exec)
	if slices.Contains(mgdb.termErr, exec) {
		return &gorm.DB{Error: errors.New("exec:foobar")}
	} else {
		return mgdb.DB.Exec(sql, values...)
	}
}

// continuation method calls
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
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Unscoped(),
	}
	return &newdb
}
func (mgdb *mockGormDB) Model(value any) gormDBInterface {
	appendCallstack(model)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Model(value),
	}
	return &newdb
}
func (mgdb *mockGormDB) Raw(sql string, values ...any) gormDBInterface {
	appendCallstack(raw)
	var newdb = mockGormDB{
		termErr: mgdb.termErr,
		DB:      mgdb.DB.Raw(sql, values...),
	}
	return &newdb
}

type stackType string

const (
	open stackType = "g.open"

	// continuation calls
	automigrate stackType = "db.automigrate"
	where       stackType = "db.where"
	preload     stackType = "db.preload"
	order       stackType = "db.order"
	limit       stackType = "db.limit"
	session     stackType = "db.session"
	unscoped    stackType = "db.unscoped"
	count       stackType = "db.count"
	model       stackType = "db.model"
	raw         stackType = "db.raw"

	// terminating calls
	firstorcreate stackType = "db.firstorcreate"
	first         stackType = "db.first"
	find          stackType = "db.find"
	save          stackType = "db.save"
	delete        stackType = "db.delete"
	scan          stackType = "db.scan"
	exec          stackType = "db.exec"
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
		gImpl = oldGorm
	}
}
