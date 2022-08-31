package poddb

import (
	"fmt"
	"gopod/testutils"
	"testing"

	"github.com/ostafen/clover/v2"
)

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
		{"existing collection", params{
			preinsert: []string{"foo"},
			collList:  []string{"foo", "bar", "arm"},
			endCount:  3}},
		{"all new collection #1", params{
			collList: []string{"foo", "bar", "arm"},
			endCount: 3}},
		{"all new collection #2", params{
			preinsert: []string{"fee", "fie", "foe", "fum"},
			collList:  []string{"foo", "bar", "arm"},
			endCount:  7}},
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

func Test_parseAndVerifyEntry(t *testing.T) {
	type params struct {
		entry any
	}
	type expected struct {
		entryMap map[string]any
		hash     string
		errorStr string
	}

	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"not a struct", params{entry: "foobar"},
			expected{errorStr: "expecting struct"}},
		{"no exported fields", params{entry: struct{ foo, bar, meh string }{"bar", "arm", "leg"}},
			expected{errorStr: "minimum two exported fields needed"}},
		{"only one exported field", params{entry: struct{ Foo, bar string }{"bar", "foo"}},
			expected{errorStr: "minimum two exported fields needed"}},
		{"no hash field exported", params{entry: struct{ Foo, Bar, Meh string }{"bar", "foo", "meh"}},
			expected{errorStr: "entry missing hash field"}},
		{"hash not string", params{entry: struct {
			Foo  string
			Hash int
		}{"bar", 42}},
			expected{errorStr: "hash should be a string"}},
		{"hash empty", params{entry: struct{ Foo, Hash string }{Foo: "bar"}},
			expected{errorStr: "hash cannot be empty"}},
		{"success", params{entry: struct{ Foo, Hash string }{Foo: "bar", Hash: "meh"}},
			expected{entryMap: map[string]any{"Foo": "bar", "Hash": "meh"}, hash: "meh"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// no db setup for this

			gotEntryMap, gotHash, err := parseAndVerifyEntry(tt.p.entry)

			testutils.AssertErrContains(t, tt.e.errorStr, err)
			testutils.AssertEquals(t, tt.e.entryMap, gotEntryMap)
			testutils.AssertEquals(t, tt.e.hash, gotHash)

		})
	}
}

func TestCollection_findDocByHash(t *testing.T) {

	clmock, teardown := setupTest(t, true, false)
	defer teardown(t, clmock)

	// insert stuff for test
	var coll = "foo"
	if err := clmock.db.CreateCollection(coll); err != nil {
		t.Fatalf("error: %v", err)
	}

	type itemType struct{ Hash, Val string }

	var items = itemType{"foobar", "testMe"}

	insdoc := clover.NewDocumentOf(items)

	docid, err := clmock.db.InsertOne(coll, insdoc)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	type params struct {
		db       *clover.DB
		collName string
		hash     string
	}
	type expected struct {
		id     string
		errStr string
		items  itemType
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"db is nil", params{},
			expected{errStr: "db is not open"}},
		{"collection doesn't exist", params{db: clmock.db, collName: "bar", hash: "foobar"},
			expected{errStr: "error in query"}},
		{"hash not found", params{db: clmock.db, collName: coll, hash: "barfoo"},
			expected{errStr: "hash not found"}},
		{"success", params{db: clmock.db, collName: coll, hash: "foobar"},
			expected{id: docid, items: items}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var coll = Collection{name: tt.p.collName}
			doc, err := coll.findDocByHash(tt.p.db, tt.p.hash)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, (doc == nil) == (tt.e.errStr != ""),
				fmt.Sprintf("expected nil doc == %v, got %v", (tt.e.errStr != ""), doc))
			if doc != nil {
				testutils.AssertEquals(t, tt.e.id, doc.ObjectId())
				var res = itemType{}
				err := doc.Unmarshal(&res)
				testutils.AssertErr(t, false, err)
				testutils.AssertEquals(t, tt.e.items, res)
			}
		})
	}
}

func TestCollection_findDocById(t *testing.T) {

	clmock, teardown := setupTest(t, true, false)
	defer teardown(t, clmock)

	// insert stuff for test
	var coll = "foo"
	if err := clmock.db.CreateCollection(coll); err != nil {
		t.Fatalf("error: %v", err)
	}

	type itemType struct{ Hash, Val string }

	var items = itemType{"foobar", "testMe"}

	insdoc := clover.NewDocumentOf(items)

	docid, err := clmock.db.InsertOne(coll, insdoc)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	type params struct {
		db       *clover.DB
		collName string
		id       string
	}
	type expected struct {
		id     string
		errStr string
		items  itemType
	}

	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"db is nil", params{},
			expected{errStr: "db is not open"}},
		{"collection doesn't exist", params{db: clmock.db, collName: "bar", id: "foobar"},
			expected{errStr: "error in query"}},
		{"id not found", params{db: clmock.db, collName: coll, id: "barfoo"},
			expected{errStr: "id not found"}},
		{"success", params{db: clmock.db, collName: coll, id: docid},
			expected{id: docid, items: items}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var coll = Collection{name: tt.p.collName}
			doc, err := coll.findDocById(tt.p.db, tt.p.id)

			testutils.AssertErrContains(t, tt.e.errStr, err)
			testutils.Assert(t, (doc == nil) == (tt.e.errStr != ""),
				fmt.Sprintf("expected nil doc == %v, got %v", (tt.e.errStr != ""), doc))
			if doc != nil {
				testutils.AssertEquals(t, tt.e.id, doc.ObjectId())
				var res = itemType{}
				err := doc.Unmarshal(&res)
				testutils.AssertErr(t, false, err)
				testutils.AssertEquals(t, tt.e.items, res)
			}
		})
	}
}

/*
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
*/
