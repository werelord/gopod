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
			clmock.checkAndResetClose(t)
			// defer after we've opened the db, not on setup
			defer teardown(t, clmock)

			// check result
			testutils.AssertErr(t, tt.wantErr, err)

			testutils.Assert(t, tt.wantErr == (got == nil), fmt.Sprintf("expected (%T == nil) == %v), got %v", got, tt.wantErr, got))

			// check poddb entries
			if got != nil {
				testutils.AssertEquals(t, got.feedColl.name, tt.p.collname)
				testutils.AssertEquals(t, got.FeedCollection().name, tt.p.collname)
				testutils.AssertEquals(t, got.itemDataColl.name, tt.p.collname+"_itemdata")
				testutils.AssertEquals(t, got.ItemDataCollection().name, tt.p.collname+"_itemdata")
				testutils.AssertEquals(t, got.itemXmlColl.name, tt.p.collname+"_itemxml")
				testutils.AssertEquals(t, got.ItemXmlCollection().name, tt.p.collname+"_itemxml")
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
			clmock.checkAndResetClose(t)

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
			clmock.checkAndResetClose(t)

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
			clmock.checkAndResetClose(t)

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

func TestCollection_FetchByEntry(t *testing.T) {

	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	type entrytype struct {
		Hash string
		Foo  string
	}

	var existingEntry = entrytype{Hash: "foo", Foo: "bar"}
	var doc = clover.NewDocumentOf(existingEntry)

	existingId, err := clmock.db.InsertOne(clmock.coll.name, doc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	type params struct {
		clmock       *mockClover
		throwOpenErr bool
		entry        any
	}
	type expected struct {
		id      string
		entry   entrytype
		errStr  string
		wantErr bool
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"no hash error", params{clmock: clmock, entry: struct{ Foo, Bar string }{"meh", "bah"}},
			expected{wantErr: true}},
		{"db open error", params{clmock: clmock, throwOpenErr: true, entry: entrytype{Hash: "foo"}},
			expected{errStr: "failed opening db", wantErr: true}},
		{"doesn't exist", params{clmock: clmock, entry: entrytype{Hash: "bar"}},
			expected{errStr: "hash not found", wantErr: true}},
		{"success", params{clmock: clmock, entry: entrytype{Hash: "foo"}},
			expected{id: existingId, entry: existingEntry}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var id string
			var err error
			// should reset every timne
			tt.p.clmock.SetOpenError(tt.p.throwOpenErr)

			// because using interface fucks with type erasure, which then fucks with unmarshalling,
			// need to hard cast the success from failure entries.. bullshit
			res, ok := tt.p.entry.(entrytype)
			if ok {
				id, err = tt.p.clmock.coll.FetchByEntry(&res)
			} else {
				id, err = tt.p.clmock.coll.FetchByEntry(&tt.p.entry)
			}
			clmock.checkAndResetClose(t)

			testutils.AssertErr(t, tt.e.wantErr, err)
			if tt.e.errStr != "" {
				testutils.AssertErrContains(t, tt.e.errStr, err)
			}
			testutils.AssertEquals(t, tt.e.id, id)
			if err == nil {
				//testutils.Assert(t, ok == true, "type returned not entry; wtf")
				testutils.AssertEquals(t, tt.e.entry, res)
			}

		})
	}
}

func TestCollection_FetchById(t *testing.T) {

	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	type entrytype struct{ Hash, Foo string }

	var existingEntry = entrytype{Hash: "foo", Foo: "bar"}
	var doc = clover.NewDocumentOf(existingEntry)

	existingId, err := clmock.db.InsertOne(clmock.coll.name, doc)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	type params struct {
		clmock       *mockClover
		id           string
		throwOpenErr bool
	}
	type expected struct {
		id     string
		entry  entrytype
		errStr string
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"db open error", params{clmock: clmock, throwOpenErr: true},
			expected{errStr: "failed opening db"}},
		{"doesn't exist", params{clmock: clmock, id: "foobar"},
			expected{errStr: "id not found"}},
		{"success", params{clmock: clmock, id: existingId},
			expected{id: existingId, entry: existingEntry}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var outEntry entrytype

			// should reset every timne
			tt.p.clmock.SetOpenError(tt.p.throwOpenErr)
			tt.p.clmock.checkAndResetClose(t)

			id, err := tt.p.clmock.coll.FetchById(tt.p.id, &outEntry)

			testutils.AssertEquals(t, tt.e.id, id)
			if testutils.AssertErrContains(t, tt.e.errStr, err) {
				testutils.AssertEquals(t, tt.e.entry, outEntry)
			}
		})
	}
}

func TestCollection_FetchAll(t *testing.T) {
	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	type entrytype struct{ Hash, Foo string }

	var entrymap = make(map[string]entrytype)
	for _, entry := range []entrytype{
		{Hash: "foo", Foo: "one"},
		{Hash: "bar", Foo: "two"},
		{Hash: "arm", Foo: "three"},
		{Hash: "meh", Foo: "four"},
	} {
		var doc = clover.NewDocumentOf(entry)
		id, err := clmock.db.InsertOne(clmock.coll.name, doc)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		entrymap[id] = entry
	}

	type params struct {
		clmock       *mockClover
		throwOpenErr bool
	}
	type expected struct {
		entryMap map[string]entrytype
		errStr   string
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"db open error", params{clmock: clmock, throwOpenErr: true},
			expected{errStr: "failed opening db"}},
		{"all items", params{clmock: clmock},
			expected{entryMap: entrymap}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			tt.p.clmock.SetOpenError(tt.p.throwOpenErr)
			var fn = func() any { var entry entrytype; return &entry }

			resultList, err := tt.p.clmock.coll.FetchAll(fn)
			clmock.checkAndResetClose(t)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, len(tt.e.entryMap) == len(resultList),
				fmt.Sprintf("expected length = %v, got %v", len(tt.e.entryMap), len(resultList)))

			// for the next loop, quick reference
			var resultmap = make(map[string]DBEntry, len(resultList))

			for _, dbentry := range resultList {
				item, exists := tt.e.entryMap[*dbentry.ID]
				testutils.Assert(t, exists == true, fmt.Sprintf("missing %v in entryMap", dbentry.ID))
				if exists {
					e, ok := dbentry.Entry.(*entrytype)
					testutils.Assert(t, ok == true, fmt.Sprintf("entry is incorrect type: %v", dbentry.Entry))
					if ok {
						testutils.AssertEquals(t, item, *e)
					}
				}

				resultmap[*dbentry.ID] = dbentry
			}

			for id, item := range tt.e.entryMap {
				dbentry, exists := resultmap[id]
				testutils.Assert(t, exists == true, fmt.Sprintf("missing %v in result", id))
				if exists {
					e, ok := dbentry.Entry.(*entrytype)
					testutils.Assert(t, ok == true, fmt.Sprintf("entry is incorrect type: %v", dbentry.Entry))
					if ok {
						testutils.AssertEquals(t, item, *e)
					}
				}
			}
		})
	}
}

func TestCollection_FetchAllWithQuery(t *testing.T) {
	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	type entrytype struct {
		Hash, Foo string
		Flag      bool
	}

	var entrymapAll = make(map[string]entrytype)
	var entrymapTrue = make(map[string]entrytype)
	var entryMapFalse = make(map[string]entrytype)

	for _, entry := range []entrytype{
		{Hash: "foo", Foo: "one", Flag: true},
		{Hash: "bar", Foo: "two", Flag: true},
		{Hash: "arm", Foo: "three", Flag: false},
		{Hash: "meh", Foo: "four", Flag: true},
		{Hash: "bah", Foo: "five", Flag: false},
	} {
		var doc = clover.NewDocumentOf(entry)
		id, err := clmock.db.InsertOne(clmock.coll.name, doc)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		entrymapAll[id] = entry
		if entry.Flag {
			entrymapTrue[id] = entry
		} else {
			entryMapFalse[id] = entry
		}
	}

	var allQuery = clmock.coll.NewQuery()
	var trueQuery = clmock.coll.NewQuery().Where(clover.Field("Flag").IsTrue())
	var falseQuery = clmock.coll.NewQuery().Where(clover.Field("Flag").IsFalse())

	type params struct {
		clmock       *mockClover
		query        *clover.Query
		throwOpenErr bool
	}
	type expected struct {
		entryMap map[string]entrytype
		errStr   string
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"db open error", params{clmock: clmock, throwOpenErr: true},
			expected{errStr: "failed opening db"}},
		{"all items", params{clmock: clmock, query: allQuery},
			expected{entryMap: entrymapAll}},
		{"false items", params{clmock: clmock, query: falseQuery},
			expected{entryMap: entryMapFalse}},
		{"true items", params{clmock: clmock, query: trueQuery},
			expected{entryMap: entrymapTrue}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.p.clmock.SetOpenError(tt.p.throwOpenErr)
			var fn = func() any { var entry entrytype; return &entry }

			resultList, err := tt.p.clmock.coll.FetchAllWithQuery(fn, tt.p.query)
			clmock.checkAndResetClose(t)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, len(tt.e.entryMap) == len(resultList),
				fmt.Sprintf("expected length = %v, got %v", len(tt.e.entryMap), len(resultList)))

			// for the next loop, quick reference
			var resultmap = make(map[string]DBEntry, len(resultList))

			for _, dbentry := range resultList {
				item, exists := tt.e.entryMap[*dbentry.ID]
				testutils.Assert(t, exists == true, fmt.Sprintf("missing %v in entryMap", dbentry.ID))
				if exists {
					e, ok := dbentry.Entry.(*entrytype)
					testutils.Assert(t, ok == true, fmt.Sprintf("entry is incorrect type: %v", dbentry.Entry))
					if ok {
						testutils.AssertEquals(t, item, *e)
					}
				}

				resultmap[*dbentry.ID] = dbentry
			}

			for id, item := range tt.e.entryMap {
				dbentry, exists := resultmap[id]
				testutils.Assert(t, exists == true, fmt.Sprintf("missing %v in result", id))
				if exists {
					e, ok := dbentry.Entry.(*entrytype)
					testutils.Assert(t, ok == true, fmt.Sprintf("entry is incorrect type: %v", dbentry.Entry))
					if ok {
						testutils.AssertEquals(t, item, *e)
					}
				}
			}

		})
	}
}
