package poddb

import (
	"errors"
	"fmt"
	"gopod/testutils"
	"testing"

	"github.com/ostafen/clover/v2"
)

// more integration tests than unit tests here; however we're replacing the cloverInterface
// definition for testing purposes (stopping deferred close); as each test will run clover in
// memory mode we don't want to close until we

type mockClover struct {
	db        *clover.DB
	openError bool
}

func (mc mockClover) Open(p string, _ ...clover.Option) (*clover.DB, error) {
	fmt.Print("MockClover.Open()")
	var err error
	// db might have been opened by setupTest()
	if mc.openError {
		fmt.Println(", returning open error")
		return nil, errors.New("foobar")
	} else if mc.db == nil {
		fmt.Println(", opening db (inMemoryMode)")
		mc.db, err = clover.Open(p, clover.InMemoryMode(true))
	} else {
		fmt.Println(", reusing already open connection")
	}
	return mc.db, err
}

func (mc mockClover) Close() error {
	fmt.Println("MockClover.Close(), faking closing db")
	return nil // need to explicitly close in teardown function
}

func (mc mockClover) FinalClose(t *testing.T) {

}

func setupTest(t *testing.T, openDB bool, openError bool) (*mockClover, func(*testing.T, *mockClover)) {
	var (
		mock = mockClover{openError: openError}
		err  error
	)
	var oldclover = cimpl
	cimpl = mock
	fmt.Print("SetupTest()")

	if openDB {
		fmt.Print(", opening db (inMemoryMode)")
		mock.db, err = clover.Open(t.Name(), clover.InMemoryMode(true))
		if err != nil {
			t.Fatalf("create db failed: %v", err)
		}
	}
	fmt.Print("\n")

	return &mock, func(t *testing.T, m *mockClover) {
		fmt.Print("Teardown()")
		if m.db != nil {
			fmt.Print(", closing db")
			m.db.Close()
		}
		fmt.Print("\n")
		cimpl = oldclover
	}
}

func TestNewDB(t *testing.T) {

	type params struct {
		dbpath    string
		collname  string
		openError bool
	}

	tests := []struct {
		name    string
		p       params
		wantErr bool
	}{
		{
			"empty db path",
			params{},
			true,
		},
		{
			"empty collection",
			params{
				dbpath: "foo",
			},
			true,
		},
		{
			"db open error",
			params{
				dbpath:    "foo",
				collname:  "bar",
				openError: true,
			},
			true,
		},
		{
			"success",
			params{
				dbpath:   "foo",
				collname: "bar",
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			clmock, teardown := setupTest(t, false, tt.p.openError)

			// mock is ready for use; do insertions, etc
			SetDBPath(tt.p.dbpath)
			got, err := NewDB(tt.p.collname)
			// defer after we've opened the db, not on setup
			defer teardown(t, clmock)

			// check result
			testutils.AssertErr(t, tt.wantErr, err)

			// if tt.wantErr && got != nil {
			// // error
			// } else if tt.wantErr == false && got == nil {
			// 	// error
			// }

			testutils.Assert(t, tt.wantErr == (got == nil), fmt.Sprintf("expected (%T == nil) == %v), got %v", got, tt.wantErr, got))

			// check poddb entries
			if got != nil {
				testutils.AssertEquals(t, got.feedColl.name, tt.p.collname)
				testutils.AssertEquals(t, got.itemDataColl.name, tt.p.collname+"_itemdata")
				testutils.AssertEquals(t, got.itemXmlColl.name, tt.p.collname+"_itemxml")
			}

			// check clover entries
			//testutils.AssertIsNil(t, tt.wantErr, clmock.db)
			if clmock.db != nil {
				// todo check collections exist
				colllist, err := clmock.db.ListCollections()
				testutils.AssertErr(t, false, err)
				testutils.Assert(t, len(colllist) == 3, fmt.Sprintf("Collection list should be 3; got %#v", colllist))
				for _, c := range []string{got.feedColl.name, got.itemDataColl.name, got.itemXmlColl.name} {
					exists, err := clmock.db.HasCollection(c)
					testutils.Assert(t, exists, "Missing collection "+c)
					testutils.AssertErr(t, false, err)
				}
			}
		})
	}
}

func Test_createCollections(t *testing.T) {

	type params struct {
		preinsert []string
		collList  []string
		endCount  int
	}

	tests := []struct {
		name string
		p    params
	}{
		// error is hard to test; the badger transaction would have to be discarded
		// but I don't care about that; just care about the cases I want to handle
		// already existing collection
		{
			"existing collection",
			params{
				preinsert: []string{"foo"},
				collList: []string{"foo", "bar", "arm"},
				endCount: 3,
			},
		},
		{
			"all new collection #1",
			params{
				collList: []string{"foo", "bar", "arm"},
				endCount: 3,
			},
		},
		{
			"all new collection #2",
			params{
				preinsert: []string{"fee","fie","foe","fum"},
				collList: []string{"foo", "bar", "arm"},
				endCount: 7,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			clmock, teardown := setupTest(t, true, false)
			defer teardown(t, clmock)

			// preinsert collections
			for _, name := range tt.p.preinsert {
				if err := clmock.db.CreateCollection(name); err != nil {
					t.Fatal("failed to create preinsert collection: ", err)
				}
			}

			// collection insert
			var list = make([]Collection, 0, len(tt.p.collList))
			for _, c := range tt.p.collList {
				list = append(list, Collection{name: c})
			}
			var err = createCollections(clmock.db, list)

			testutils.AssertErr(t, false, err)

			// make sure collections exist
			if err == nil {
				colllist, err := clmock.db.ListCollections()
				testutils.AssertErr(t, false, err)
				testutils.Assert(t, len(colllist) == tt.p.endCount,
					fmt.Sprintf("Collection list should be %v; got %#v", tt.p.endCount, colllist))
				for _, c := range tt.p.collList {
					exists, err := clmock.db.HasCollection(c)
					testutils.Assert(t, exists, "Missing collection "+c)
					testutils.AssertErr(t, false, err)
				}
			}
		})
	}
}

// before doing inserts/fetches, verify the helper functions

/*
func TestCollection_InsertyByEntry(t *testing.T) {
	type params struct {
		
	}
	tests := []struct {
		name    string
		p params
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.InsertyByEntry(tt.args.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.InsertyByEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Collection.InsertyByEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

/*
func TestCollection_InsertyById(t *testing.T) {
	type args struct {
		id    string
		entry any
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.InsertyById(tt.args.id, tt.args.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.InsertyById() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Collection.InsertyById() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollection_InsertAll(t *testing.T) {
	type args struct {
		entryList []*DBEntry
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.InsertAll(tt.args.entryList); (err != nil) != tt.wantErr {
				t.Errorf("Collection.InsertAll() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollection_insert(t *testing.T) {
	type args struct {
		dbEntryList []*DBEntry
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.insert(tt.args.dbEntryList); (err != nil) != tt.wantErr {
				t.Errorf("Collection.insert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCollection_FetchByEntry(t *testing.T) {
	type args struct {
		value any
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.FetchByEntry(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.FetchByEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Collection.FetchByEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollection_FetchById(t *testing.T) {
	type args struct {
		id    string
		value any
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.FetchById(tt.args.id, tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.FetchById() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Collection.FetchById() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollection_FetchAll(t *testing.T) {
	type args struct {
		fn func() any
	}
	tests := []struct {
		name          string
		c             Collection
		args          args
		wantEntryList []DBEntry
		wantErr       bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEntryList, err := tt.c.FetchAll(tt.args.fn)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.FetchAll() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotEntryList, tt.wantEntryList) {
				t.Errorf("Collection.FetchAll() = %v, want %v", gotEntryList, tt.wantEntryList)
			}
		})
	}
}

func TestCollection_FetchAllWithQuery(t *testing.T) {
	type args struct {
		fn func() any
		q  *clover.Query
	}
	tests := []struct {
		name          string
		c             Collection
		args          args
		wantEntryList []DBEntry
		wantErr       bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEntryList, err := tt.c.FetchAllWithQuery(tt.args.fn, tt.args.q)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.FetchAllWithQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotEntryList, tt.wantEntryList) {
				t.Errorf("Collection.FetchAllWithQuery() = %v, want %v", gotEntryList, tt.wantEntryList)
			}
		})
	}
}

func TestCollection_findDocByHash(t *testing.T) {
	type args struct {
		db   *clover.DB
		hash string
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		want    *clover.Document
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.findDocByHash(tt.args.db, tt.args.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.findDocByHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Collection.findDocByHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollection_findDocById(t *testing.T) {
	type args struct {
		db *clover.DB
		id string
	}
	tests := []struct {
		name    string
		c       Collection
		args    args
		want    *clover.Document
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.c.findDocById(tt.args.db, tt.args.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("Collection.findDocById() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Collection.findDocById() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseAndVerifyEntry(t *testing.T) {
	type args struct {
		entry any
	}
	tests := []struct {
		name         string
		args         args
		wantEntryMap map[string]any
		wantHash     string
		wantErr      bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotEntryMap, gotHash, err := parseAndVerifyEntry(tt.args.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAndVerifyEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotEntryMap, tt.wantEntryMap) {
				t.Errorf("parseAndVerifyEntry() gotEntryMap = %v, want %v", gotEntryMap, tt.wantEntryMap)
			}
			if gotHash != tt.wantHash {
				t.Errorf("parseAndVerifyEntry() gotHash = %v, want %v", gotHash, tt.wantHash)
			}
		})
	}
}

func TestExportAllCollections(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ExportAllCollections(tt.args.path)
		})
	}
}

func TestCollection_DropCollection(t *testing.T) {
	tests := []struct {
		name    string
		c       Collection
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.c.DropCollection(); (err != nil) != tt.wantErr {
				t.Errorf("Collection.DropCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
*/
