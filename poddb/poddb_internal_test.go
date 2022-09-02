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

			clmock, teardown := setupTest(t, true, "", false)
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

			if testutils.AssertErr(t, false, err) {
				// make sure collections exist
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

	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	type itemType struct{ Hash, Val string }

	var items = itemType{"foobar", "testMe"}

	insdoc := clover.NewDocumentOf(items)

	docid, err := clmock.db.InsertOne(clmock.coll.name, insdoc)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	type params struct {
		db   *clover.DB
		coll Collection
		hash string
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
		{"db is nil", params{coll: clmock.coll},
			expected{errStr: "db is not open"}},
		{"collection doesn't exist", params{db: clmock.db, coll: Collection{"bar"}, hash: "foobar"},
			expected{errStr: "error in query"}},
		{"hash not found", params{db: clmock.db, coll: clmock.coll, hash: "barfoo"},
			expected{errStr: "hash not found"}},
		{"success", params{db: clmock.db, coll: clmock.coll, hash: "foobar"},
			expected{id: docid, items: items}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := tt.p.coll.findDocByHash(tt.p.db, tt.p.hash)

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

	clmock, teardown := setupTest(t, true, "foo", false)
	defer teardown(t, clmock)

	type itemType struct{ Hash, Val string }

	var items = itemType{"foobar", "testMe"}

	insdoc := clover.NewDocumentOf(items)

	docid, err := clmock.db.InsertOne(clmock.coll.name, insdoc)
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}

	type params struct {
		db   *clover.DB
		coll Collection
		id   string
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
		{"db is nil", params{coll: clmock.coll},
			expected{errStr: "db is not open"}},
		{"collection doesn't exist", params{db: clmock.db, coll: Collection{"bar"}, id: "foobar"},
			expected{errStr: "error in query"}},
		{"id not found", params{db: clmock.db, coll: clmock.coll, id: "barfoo"},
			expected{errStr: "id not found"}},
		{"success", params{db: clmock.db, coll: clmock.coll, id: docid},
			expected{id: docid, items: items}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := tt.p.coll.findDocById(tt.p.db, tt.p.id)

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

func TestCollection_insert(t *testing.T) {
	//setupTest(t, true, false)

	type validEntry struct {
		Hash string
		Foo  string
	}

	type preinsert struct {
		replaceId bool
		entry     validEntry
	}

	var cp = func(entry DBEntry) *DBEntry {
		// allocate new, copy entry values, return reference to new
		var cpy = entry
		return &cpy
	}

	var (
		foobar = "foobar"
		// error entries
		emptyEntry  = DBEntry{}
		noHashEntry = DBEntry{Entry: struct{ Foo, Bar string }{Foo: "bar", Bar: "foo"}}
		entryWithId = DBEntry{ID: &foobar, Entry: validEntry{Hash: "foobar", Foo: "meh"}}

		// valid entries
		entryOne   = DBEntry{Entry: validEntry{Hash: "foobar", Foo: "meh"}}
		entryTwo   = DBEntry{Entry: validEntry{Hash: "armleg", Foo: "bar"}}
		entryThree = DBEntry{Entry: validEntry{Hash: "barfoo", Foo: "armleg"}}

		// preinsert entries
		entryOneModified = validEntry{Hash: "foobar", Foo: "bah"}
		entryTwoModified = validEntry{Hash: "armleg", Foo: "legarm"}
	)

	//var oneEntry = []*DBEntry{&emptyEntry}

	type params struct {
		coll    Collection
		openErr bool
		entries []*DBEntry
	}
	type expected struct {
		preInsert      []preinsert
		totalItemCount int
		errStr         string
	}
	tests := []struct {
		name string
		p    params
		e    expected
	}{
		{"no entries,", params{},
			expected{errStr: "insert list is empty"}},
		{"open error", params{openErr: true, entries: []*DBEntry{cp(entryOne)}},
			expected{errStr: "failed opening db"}},
		{"bad entry", params{entries: []*DBEntry{cp(emptyEntry)}},
			expected{errStr: "expecting struct, got"}},
		{"no hash entry", params{entries: []*DBEntry{cp(noHashEntry)}},
			expected{errStr: "entry missing hash field"}},
		{"collection doesn't exist", params{coll: Collection{"bar"}, entries: []*DBEntry{cp(entryOne)}},
			expected{errStr: "collection doesn't exist"}},
		{"insert by id, doesn't exist", params{entries: []*DBEntry{cp(entryWithId)}},
			expected{errStr: "ID set, but failed to find document"}},

		{"insert by hash, new entry", params{entries: []*DBEntry{cp(entryOne)}},
			expected{totalItemCount: 1}},
		{"insert by hash, replace existing", params{entries: []*DBEntry{cp(entryOne)}},
			expected{totalItemCount: 1, preInsert: []preinsert{{entry: entryOneModified}}}},

		{"insert by id, replace existing", params{entries: []*DBEntry{cp(entryOne)}},
			expected{totalItemCount: 1, preInsert: []preinsert{{replaceId: true,
				entry: entryOneModified}}}},

		{"insert all (mix and match)", params{entries: []*DBEntry{cp(entryOne), cp(entryTwo), cp(entryThree)}},
			expected{totalItemCount: 3, preInsert: []preinsert{
				{replaceId: true, entry: entryOneModified}, // replace by id
				{entry: entryTwoModified},                  // replace by hash
			}},
		},

		// multiple entries, mix & match

	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			clmock, teardown := setupTest(t, true, "foo", tt.p.openErr)
			defer teardown(t, clmock)

			var coll = clmock.coll
			if tt.p.coll.name != "" {
				coll = tt.p.coll
			}

			// preinsert; populate the preinsert map
			var preInserMap = make(map[string]preinsert, len(tt.e.preInsert))
			if len(tt.e.preInsert) > 0 {
				for i, e := range tt.e.preInsert {
					if id, err := clmock.db.InsertOne(clmock.coll.name, clover.NewDocumentOf(e.entry)); err != nil {
						t.Fatalf("error: %v", err)
					} else {
						preInserMap[id] = e
						if e.replaceId {
							// assume the index of preinsert matches entries
							tt.p.entries[i].ID = &id
						}

					}
				}
			}

			// finally all the pre-shit is done; do the insert and check results
			// use local coll in case of coll not exist test
			inserterr := coll.insert(tt.p.entries)
			if testutils.AssertErrContains(t, tt.e.errStr, inserterr) {

				// make sure entry count matches
				count, err := clmock.db.Count(clover.NewQuery(clmock.coll.name))
				testutils.AssertErr(t, false, err)
				testutils.Assert(t, count == tt.e.totalItemCount,
					fmt.Sprintf("expecting count %v, got %v entries", tt.e.totalItemCount, count))

				for _, e := range tt.p.entries {
					// make sure entries have IDs set
					testutils.Assert(t, (e.ID != nil) && (*e.ID != ""),
						fmt.Sprintf("id should not be empty: %#v", e))

					if e.ID != nil {
						// able to find the entry in the db

						doc, err := clmock.db.FindById(clmock.coll.name, *e.ID)
						if testutils.AssertErr(t, false, err) {
							// make sure entry matches original
							var insertedEntry validEntry
							doc.Unmarshal(&insertedEntry)
							testutils.AssertEquals(t, e.Entry, insertedEntry)
						}

						// make sure if preinsert changed, that the result doesn't match the original
						if preEntry, exists := preInserMap[*e.ID]; exists {
							// by default, for testing purposes we're assuming the insert modifies the orig entry
							testutils.AssertNotEquals(t, preEntry.entry, e.Entry)
						}
					}
				}
			}
		})
	}
}
