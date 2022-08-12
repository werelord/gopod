package poddb

import (
	"errors"
	"fmt"
	"gopod/podutils"
	"os"
	"path/filepath"
	"reflect"

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
	collection string
}

type Entry struct {
	Id    string
	Name  string
	Entry any
}

// common to all instances
var (
	dbpath string
)

func SetDBPath(path string) {
	dbpath = path
}

// --------------------------------------------------------------------------
func NewDB(coll string) (*PodDB, error) {
	if coll == "" {
		return nil, errors.New("collection name cannot be empty")
	}
	var podDB = PodDB{collection: coll}

	db, err := clover.Open(dbpath)
	if err != nil {
		return nil, fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	// make sure collection exists
	if exists, err := db.HasCollection(podDB.collection); err != nil {
		return nil, fmt.Errorf("failed checking collection exists, wtf: %w", err)
	} else if exists == false {
		if err := db.CreateCollection(podDB.collection); err != nil {
			return nil, err
		}
	}
	// collection should exist at this point

	return &podDB, nil

}

// --------------------------------------------------------------------------
// inserts by entry. Will use struct field name as key; struct field value as the value
func (d PodDB) InsertyByEntry(entry any) (string, error) {
	return d.insert("", entry)
}

// --------------------------------------------------------------------------
// inserts by id, replacing the entry if ID is found
// Will use struct field name as key; struct field value as the value
func (d PodDB) InsertyById(id string, entry any) (string, error) {
	return d.insert(id, entry)
}

// --------------------------------------------------------------------------
// inserts entry, replacing via key if it exists..
// will use ID if exists, otherwise will try to find based on key
// returns ID of inserted item if successful, error otherwise
func (d PodDB) insert(id string, entry any) (string, error) {
	var (
		db  *clover.DB
		err error
		doc *clover.Document

		entryName string
		entryVal  any

	)

	entryName, entryVal, err = parseEntry(entry)
	if (err != nil) {
		return "", err
	}

	db, err = clover.Open(dbpath)
	if err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	if id == "" {
		// find doc by name based on entry
		if doc, err = d.findDocByName(db, entryName); err != nil && errors.As(err, &ErrorDoesNotExist{}) == false {
			log.Warnf("failed to find document: ", err)
		}
	} else {
		if doc, err = d.findDocById(db, id); err != nil {
			log.Warn("failed to find document: ", err)
		}
	}

	// if we didn't find a matching document, create a new one
	if doc == nil {
		doc = clover.NewDocument()
	}

	doc.Set(entryName, entryVal)

	db.Save(d.collection, doc)
	log.Debug("document saved, id: ", doc.ObjectId())

	return doc.ObjectId(), nil
}

// --------------------------------------------------------------------------
func (d PodDB) FetchByEntry(value any) (string, error) {
	var (
		db  *clover.DB
		err error
		doc *clover.Document

		entryName string
	)

	entryName, _, err = parseEntry(value)
	if (err != nil) {
		return "", err
	}

	if db, err = clover.Open(dbpath); err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	if doc, err = d.findDocByName(db, entryName); err != nil {
		return "", fmt.Errorf("find doc error: %v", err)
	}

	if err = doc.Unmarshal(value); err != nil {
		return "", fmt.Errorf("unmarshal error: %v", err)
	}
	return doc.ObjectId(), nil
}

// --------------------------------------------------------------------------
func (d PodDB) FetchById(id string, value any) (string, error) {
	var (
		db  *clover.DB
		err error
		doc *clover.Document
	)

	if db, err = clover.Open(dbpath); err != nil {
		return "", fmt.Errorf("failed opening db: %v", err)
	}
	defer db.Close()

	doc, err = d.findDocById(db, id)
	if err != nil {
		return "", fmt.Errorf("find doc error: %v", err)
	} else if doc == nil {
		return "", ErrorDoesNotExist{"doc returned is nil"}
	}
	// todo: more checks??

	if err = doc.Unmarshal(value); err != nil {
		return "", fmt.Errorf("unmarshal error: %v", err)
	}

	return doc.ObjectId(), nil
}

// --------------------------------------------------------------------------
func (d PodDB) findDocByName(db *clover.DB, name string) (*clover.Document, error) {
	if db == nil {
		return nil, errors.New("db is not open")
	}
	docs, err := db.FindAll(clover.NewQuery(d.collection).Where(clover.Field(name).Exists()))
	if err != nil {
		return nil, fmt.Errorf("error in query: %w", err)
	} else if len(docs) == 0 {
		return nil, ErrorDoesNotExist{"name not found"}
	}

	if len(docs) > 1 {
		log.Warn("more than one document with given name found; len == ", len(docs))
	}
	return docs[0], nil
}

// --------------------------------------------------------------------------
func (d PodDB) findDocById(db *clover.DB, id string) (*clover.Document, error) {
	if db == nil {
		return nil, errors.New("db is not open")
	}

	return db.FindById(d.collection, id)
}

//--------------------------------------------------------------------------
func parseEntry(entry any) (string, any, error) {
	var (
		entryName string
		entryVal any
		elem reflect.Value
	)
	elem = reflect.Indirect(reflect.ValueOf(entry))
	if elem.Kind() != reflect.Struct {
		return "", nil, fmt.Errorf("expecting interface, got %v", elem.Kind())
	} else if elem.NumField() != 1 {
		return "", nil, fmt.Errorf("expecting one field in interface, got %v", elem.NumField())
	}

	entryName = elem.Type().Field(0).Name
	entryVal = elem.Field(0).Interface()

	return entryName, entryVal, nil
}

// --------------------------------------------------------------------------
func DumpCollections(path string) {

	if podutils.FileExists(path) == false {
		os.MkdirAll(path, os.ModePerm)
	}

	db, err := clover.Open(dbpath)
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
