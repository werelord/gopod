package poddb

import (
	"fmt"
	"gopod/testutils"
	"testing"
)

// more integration tests than unit tests here; however we're replacing the cloverInterface
// definition for testing purposes (stopping deferred close); as each test will run clover in
// memory mode we don't want to close until the test is complete

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
		{"empty db path", params{}, true},
		{"empty collection", params{dbpath: "foo"}, true},
		{"db open error", params{dbpath: "foo", collname: "bar", openError: true}, true},
		{"success", params{dbpath: "foo", collname: "bar"}, false},
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
