package poddb

/*
import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"

	"github.com/ostafen/clover/v2"
	log "github.com/sirupsen/logrus"
)

type ErrorDoesNotExist struct {
	msg string
}

func (e ErrorDoesNotExist) Error() string {
	return e.msg
}

// abstract away db structure
type PodDB struct {
	feedColl     Collection
	itemDataColl Collection
	itemXmlColl  Collection

	// todo: dirty flag
	// todo: backup on dump ??
}

type Collection struct {
	name string
}

type DBEntry struct {
	ID *string
	// todo: move hash to explicit entry; must be included (do after tests)
	Entry any
}

func (d PodDB) FeedCollection() Collection {
	return d.feedColl
}
func (d PodDB) ItemDataCollection() Collection {
	return d.itemDataColl
}
func (d PodDB) ItemXmlCollection() Collection {
	return d.itemXmlColl
}

func (c Collection) NewQuery() *clover.Query {
	return clover.NewQuery(c.name)
}

func SetDBPath(path string) {
	dbpath = path
}

// --------------------------------------------------------------------------
func NewDB(coll string) (*PodDB, error) {
	if dbpath == "" {
		return nil, errors.New("db path is empty; set the db path via SetDBPath first")
	}
	if coll == "" {
		return nil, errors.New("collection name cannot be empty")
	}
	var podDB = PodDB{}
	podDB.feedColl.name = coll
	podDB.itemDataColl.name = coll + "_itemdata"
	podDB.itemXmlColl.name = coll + "_itemxml"

	db, err := cimpl.Open(dbpath, options)
	if err != nil {
		return nil, fmt.Errorf("failed opening db: %v", err)
	}
	defer cimpl.Close()

	// make sure collections exists
	err = createCollections(db, []Collection{podDB.feedColl, podDB.itemDataColl, podDB.itemXmlColl})
	if err != nil {
		return nil, err
	}

	// collections should exist at this point

	return &podDB, nil
}

// --------------------------------------------------------------------------
// inserts by entry. Will use struct Hash field as document key;
// struct field value as the value to be inserted..
// will take the first valid
func (c Collection) InsertyByEntry(entry any) (string, error) {

	dbe := DBEntry{
		ID:    new(string),
		Entry: entry,
	}
	err := c.insert([]*DBEntry{&dbe})
	return *dbe.ID, err
}

// --------------------------------------------------------------------------
// inserts by id, replacing the entry if ID is found
// Will use struct field name as key; struct field value as the value
func (c Collection) InsertyById(id string, entry any) (string, error) {
	// make sure we're not referencing the caller's string.. although I don't think it does
	// an extra allocation here won't hurt I guess

	if id == "" {
		return "", errors.New("id cannot be empty")
	}

	dbe := DBEntry{
		ID:    new(string),
		Entry: entry,
	}
	*dbe.ID = id
	err := c.insert([]*DBEntry{&dbe})
	return *dbe.ID, err
}

func (c Collection) InsertAll(entryList []*DBEntry) error {
	return c.insert(entryList)
}

// --------------------------------------------------------------------------
func (c Collection) FetchByEntry(value any) (string, error) {
	var (
		db  *clover.DB
		err error
		doc *clover.Document

		//entryMap map[string]any
		hash string
	)

	_, hash, err = parseAndVerifyEntry(value)
	if err != nil {
		return "", err
	}

	if db, err = cimpl.Open(dbpath, options); err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer cimpl.Close()

	if doc, err = c.findDocByHash(db, hash); err != nil {
		return "", fmt.Errorf("find doc error: %v", err)
	} else if doc == nil {
		return "", ErrorDoesNotExist{"entry not found"}
	}

	if err = doc.Unmarshal(&value); err != nil {
		return "", fmt.Errorf("unmarshal error: %v", err)
	}
	return doc.ObjectId(), nil
}

// --------------------------------------------------------------------------
func (c Collection) FetchById(id string, value any) (string, error) {
	var (
		db  *clover.DB
		err error
		doc *clover.Document
	)

	if db, err = cimpl.Open(dbpath, options); err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer cimpl.Close()

	doc, err = c.findDocById(db, id)
	if err != nil {
		return "", fmt.Errorf("find doc error: %v", err)
	} else if doc == nil {
		return "", ErrorDoesNotExist{"entry not found"}
	}
	// todo: more checks??

	if err = doc.Unmarshal(&value); err != nil {
		return "", fmt.Errorf("unmarshal error: %v", err)
	}

	return doc.ObjectId(), nil
}

// --------------------------------------------------------------------------
func (c Collection) FetchAll(fn func() any) (entryList []DBEntry, err error) {

	return c.FetchAllWithQuery(fn, clover.NewQuery(c.name))
}

// --------------------------------------------------------------------------
func (c Collection) FetchAllWithQuery(fn func() any, q *clover.Query) (entryList []DBEntry, err error) {
	var (
		db   *clover.DB
		docs []*clover.Document
	)
	if db, err = cimpl.Open(dbpath, options); err != nil {
		err = fmt.Errorf("failed opening db: %v", err)
		return
	}
	defer cimpl.Close()

	// fuck if I know what errrors are returned here..
	docs, err = db.FindAll(q)
	if err != nil {
		err = fmt.Errorf("findall failed: %v", err)
		return
	}

	for _, doc := range docs {
		var newEntry = fn()
		// does error continue outside??
		if err = doc.Unmarshal(newEntry); err != nil {
			log.Error("unmarshal failed: ", err)
			continue
		}
		var entry = DBEntry{
			ID:    new(string),
			Entry: newEntry,
		}
		*entry.ID = doc.ObjectId()
		entryList = append(entryList, entry)
	}

	return
}

// --------------------------------------------------------------------------
func ExportAllCollections(path string) {

	// don't check if it exists; MkdirAll will skip if it already does
	if err := podutils.MkdirAll(path); err != nil {
		log.Error("MkdirAll failed: ", err)
		return
	}

	db, err := cimpl.Open(dbpath, options)
	if err != nil {
		log.Error("failed opening db: ", err)
		return
	}
	defer cimpl.Close()

	list, err := db.ListCollections()
	if err != nil {
		log.Error("failed getting collections: ", err)
		return
	}
	for _, coll := range list {
		if err := db.ExportCollection(coll, filepath.Join(path, coll+".json")); err != nil {
			log.Errorf("failed exporting collection '%v': %v", coll, err)
		}
	}
}

// --------------------------------------------------------------------------
func (c Collection) DropCollection() error {

	db, err := cimpl.Open(dbpath, options)
	if err != nil {
		err = fmt.Errorf("failed opening db: %v", err)
		return err
	}
	defer cimpl.Close()

	return db.DropCollection(c.name)
}

// todo: dump collection based on instance
*/