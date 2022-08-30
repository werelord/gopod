package poddb

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"path/filepath"
	"reflect"

	"github.com/ostafen/clover/v2"
	log "github.com/sirupsen/logrus"
)

type cloverInterface interface {
	Open(string, ...clover.Option) (*clover.DB, error)
	Close() error
}

type cloverImpl struct {
	db *clover.DB
}

func (c cloverImpl) Open(p string, o ...clover.Option) (*clover.DB, error) {
	var err error
	c.db, err = clover.Open(p, o...)
	return c.db, err
}

func (c cloverImpl) Close() error {
	return c.db.Close()
}

var cimpl cloverInterface = cloverImpl{}

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
	ID    *string
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

// common to all instances
var (
	dbpath  string
	options = clover.InMemoryMode(false)
)

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

func createCollections(db *clover.DB, colllist []Collection) error {
	for _, coll := range colllist {
		if exists, err := db.HasCollection(coll.name); err != nil {
			return fmt.Errorf("failed checking collection '%v' exists, wtf: %w", coll, err)
		} else if exists == false {
			if err := db.CreateCollection(coll.name); err != nil {
				return fmt.Errorf("failed creating collection: %v", err)
			}
		}
	}
	return nil
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
// inserts entry, replacing via key if it exists..
// will use ID if exists, otherwise will try to find based on hash key
// returns ID of inserted item if successful, error otherwise
func (c Collection) insert(dbEntryList []*DBEntry) error {
	// todo: move this to array
	var (
		db  *clover.DB
		err error
		doc *clover.Document

		entryMap map[string]any

		hash string
	)
	db, err = clover.Open(dbpath, options)
	if err != nil {
		return fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	// collect all the documents, set the values
	// todo: can we run this loop concurrently??
	for _, entry := range dbEntryList {
		// todo: can we run the parse concurrently??
		entryMap, hash, err = parseAndVerifyEntry(entry.Entry)
		if err != nil {
			return err
		}

		// todo: move this if into InsertBy* methods (???)
		if entry.ID == nil || *entry.ID == "" {
			// find doc by name based on entry
			if doc, err = c.findDocByHash(db, hash); err != nil && errors.As(err, &ErrorDoesNotExist{}) == false {
				log.Warn("failed to find document: ", err)
			}
		} else {
			if doc, err = c.findDocById(db, *entry.ID); err != nil {
				log.Warn("failed to find document: ", err)
			}
		}

		// if we didn't find a matching document, create a new one
		if doc == nil {
			doc = clover.NewDocument()
		}

		for k, v := range entryMap {
			doc.Set(k, v)
		}
		if err = db.Save(c.name, doc); err != nil {
			return err
		}
		//log.Debug("document saved, id: ", doc.ObjectId())
		// make sure the id is saved in the entry
		if entry.ID == nil {
			entry.ID = new(string)
		}
		*entry.ID = doc.ObjectId()
	}
	return nil
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

	if db, err = clover.Open(dbpath, options); err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	if doc, err = c.findDocByHash(db, hash); err != nil {
		return "", fmt.Errorf("find doc error: %v", err)
	} else if doc == nil {
		return "", ErrorDoesNotExist{"doc is nil"}
	}

	if err = doc.Unmarshal(value); err != nil {
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

	if db, err = clover.Open(dbpath, options); err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	doc, err = c.findDocById(db, id)
	if err != nil {
		return "", fmt.Errorf("find doc error: %v", err)
	} else if doc == nil {
		return "", ErrorDoesNotExist{"doc is nil"}
	}
	// todo: more checks??

	if err = doc.Unmarshal(value); err != nil {
		return "", fmt.Errorf("unmarshal error: %v", err)
	}

	return doc.ObjectId(), nil
}

func (c Collection) FetchAll(fn func() any) (entryList []DBEntry, err error) {

	return c.FetchAllWithQuery(fn, clover.NewQuery(c.name))
}

// --------------------------------------------------------------------------
func (c Collection) FetchAllWithQuery(fn func() any, q *clover.Query) (entryList []DBEntry, err error) {
	var (
		db   *clover.DB
		docs []*clover.Document
	)
	if db, err = clover.Open(dbpath, options); err != nil {
		err = fmt.Errorf("failed opening db: %v", err)
		return
	}
	defer db.Close()

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
func (c Collection) findDocByHash(db *clover.DB, hash string) (*clover.Document, error) {
	if db == nil {
		return nil, errors.New("db is not open")
	}
	doc, err := db.FindFirst(clover.NewQuery(c.name).Where(clover.Field("Hash").Eq(hash)))
	if err != nil {
		return nil, fmt.Errorf("error in query: %w", err)
	} else if doc == nil {
		return nil, ErrorDoesNotExist{"hash not found"}
	}

	return doc, nil
}

// --------------------------------------------------------------------------
func (c Collection) findDocById(db *clover.DB, id string) (*clover.Document, error) {
	if db == nil {
		return nil, errors.New("db is not open")
	}

	return db.FindById(c.name, id)
}

// --------------------------------------------------------------------------
func parseAndVerifyEntry(entry any) (entryMap map[string]any, hash string, err error) {
	var (
		elem reflect.Value
		succ bool
	)
	entryMap = make(map[string]any)

	elem = reflect.Indirect(reflect.ValueOf(entry))
	if elem.Kind() != reflect.Struct {
		err = fmt.Errorf("expecting struct, got %v", elem.Kind())
		return
	} else if elem.NumField() <= 1 {
		err = fmt.Errorf("expecting at least two fields in interface, got %v", elem.NumField())
		return
	}

	for i := 0; i < elem.NumField(); i++ {
		entryMap[elem.Type().Field(i).Name] = elem.Field(i).Interface()
	}

	if hashInterface, exists := entryMap["Hash"]; exists == false {
		err = errors.New("entry missing hash field; must be included to insert")
		return
	} else if hash, succ = hashInterface.(string); succ == false {
		err = errors.New("hash should be a string")
		return
	}

	return entryMap, hash, nil
}

// --------------------------------------------------------------------------
func ExportAllCollections(path string) {

	// don't check if it exists; MkdirAll will skip if it already does
	if err := podutils.MkdirAll(path); err != nil {
		log.Error("MkdirAll failed: ", err)
		return
	}

	db, err := clover.Open(dbpath, options)
	if err != nil {
		log.Error("failed opening db: ", err)
		return
	}
	defer db.Close()

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

	db, err := clover.Open(dbpath, options)
	if err != nil {
		err = fmt.Errorf("failed opening db: %v", err)
		return err
	}
	defer db.Close()

	return db.DropCollection(c.name)
}

// todo: dump collection based on instance