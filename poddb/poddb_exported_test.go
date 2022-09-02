package poddb

import (
	"fmt"
	"gopod/testutils"
	"testing"

	"github.com/ostafen/clover/v2"
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

			clmock, teardown := setupTest(t, false, "", tt.p.openError)

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

func TestCollection_InsertyByEntry(t *testing.T) {

	type entry struct {
		Hash string
		Foo  string
	}

	type params struct {
		entry any
	}
	type expected struct {
		idEmpty bool
		wantErr bool
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		// simple checks; majority of checks (replace, db errors, etc) on internal function
		{"missing hash field", params{entry: struct{ Foo string }{Foo: "bar"}},
			expected{idEmpty: true, wantErr: true}},
		{"hash empty", params{entry: entry{Foo: "bar"}},
			expected{idEmpty: true, wantErr: true}},
		{"success", params{entry: entry{Hash: "barfoo", Foo: "bar"}},
			expected{idEmpty: false, wantErr: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var collname = "foo"

			clmock, teardown := setupTest(t, true, collname, false)
			defer teardown(t, clmock)

			id, err := clmock.coll.InsertyByEntry(tt.p.entry)

			testutils.Assert(t, (id == "") == tt.e.idEmpty,
				fmt.Sprintf("expecting ID empty == %v, got %v", tt.e.idEmpty, id))
			if testutils.AssertErr(t, tt.e.wantErr, err) {
				// make sure entry exists and matches
				doc, err := clmock.db.FindById(clmock.coll.name, id)
				if testutils.AssertErr(t, false, err) {
					var dbentry entry
					err = doc.Unmarshal(&dbentry)
					testutils.AssertErr(t, false, err)
					testutils.AssertEquals(t, tt.p.entry, dbentry)
				}
			}
		})
	}
}

func TestCollection_InsertyById(t *testing.T) {
	type entry struct {
		Hash string
		Foo  string
	}

	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	// insert existing
	var existing = entry{"foobar", "existing"}
	var newentry = entry{"barfoo", "newentry"}
	doc := clover.NewDocumentOf(existing)
	dbid, err := clmock.db.InsertOne(clmock.coll.name, doc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	type params struct {
		clmock *mockClover
		id     string
		entry  any
	}
	type expected struct {
		origId    string
		origEntry entry
		wantErr   bool
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		// simple checks; majority of checks (replace, db errors, etc) on internal function
		{"id empty", params{clmock: clmock, entry: newentry},
			expected{origId: dbid, origEntry: existing, wantErr: true}},
		{"id not found", params{clmock: clmock, id: "foobar", entry: newentry},
			expected{origId: dbid, origEntry: existing, wantErr: true}},
		{"hash empty", params{clmock: clmock, id: dbid, entry: entry{Foo: "bar"}},
			expected{origId: dbid, origEntry: existing, wantErr: true}},
		{"success", params{clmock: clmock, id: dbid, entry: newentry},
			expected{origId: dbid, origEntry: existing, wantErr: false}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			id, err := tt.p.clmock.coll.InsertyById(tt.p.id, tt.p.entry)

			testutils.AssertEquals(t, tt.p.id, id)
			insertErr := testutils.AssertErr(t, tt.e.wantErr, err) == false

			// get the original entry
			doc, err := clmock.db.FindById(tt.p.clmock.coll.name, tt.e.origId)
			if err != nil {
				t.Fatalf("error: %v", err)
			}
			var dbentry entry
			if err := doc.Unmarshal(&dbentry); err != nil {
				t.Fatalf("error: %v", err)
			}

			if insertErr {
				// make sure db matches the original
				testutils.AssertEquals(t, tt.e.origEntry, dbentry)
				testutils.AssertNotEquals(t, tt.p.entry, dbentry)
			} else {
				// make sure db matches new entry
				testutils.AssertEquals(t, tt.p.entry, dbentry)
				testutils.AssertNotEquals(t, tt.e.origEntry, dbentry)
			}
		})
	}
}

func TestCollection_InsertAll(t *testing.T) {
	// not doing error checks; alot of that handled by test on internal method

	type entry struct {
		Hash string
		Foo  string
	}

	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	var (
		entryOne         = entry{"foo", "entryone"}
		entryOneModified = entry{"foo", "entryonemodified"}
		entryTwo         = entry{"bar", "entryTwo"}
		entryThree       = entry{"meh", "entryThree"}
	)

	// insert pre-existing entries, just one pre-populated
	doc := clover.NewDocumentOf(entryOne)
	id, err := clmock.db.InsertOne(clmock.coll.name, doc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	type params struct {
		clmock    *mockClover
		entryList []*DBEntry
	}
	type expected struct {
		wantErr bool
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"multi insert", params{clmock: clmock,
			entryList: []*DBEntry{
				{ID: &id, Entry: entryOneModified},
				{Entry: entryTwo},
				{Entry: entryThree}}},
			expected{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.clmock.coll.InsertAll(tt.p.entryList)

			if testutils.AssertErr(t, tt.e.wantErr, err) {
				// make sure all ids are set
				for _, origEntry := range tt.p.entryList {
					testutils.Assert(t, origEntry.ID != nil, "ID should not be nil")
					testutils.Assert(t, *(origEntry.ID) != "", "ID should not be empty")

					// make sure db entry matches
					doc, err := clmock.db.FindById(clmock.coll.name, *origEntry.ID)
					testutils.AssertErr(t, false, err)
					var dbentry entry
					err = doc.Unmarshal(&dbentry)
					testutils.AssertErr(t, false, err)
					testutils.AssertEquals(t, origEntry.Entry, dbentry)
				}
			}

		})
	}
}

/*
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

/*
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

/*
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

/*
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


/*
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

/*
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
