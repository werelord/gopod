package poddb

import (
	"errors"
	"fmt"
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

// common to all instances
var (
	dbpath  string
	options = clover.InMemoryMode(false)
)

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
func parseAndVerifyEntry(entry any) (map[string]any, string, error) {
	var (
		elem reflect.Value
		succ bool

		entryMap = make(map[string]any)
		hash     string
		err      error
	)

	elem = reflect.Indirect(reflect.ValueOf(entry))
	if elem.Kind() != reflect.Struct {
		err = fmt.Errorf("expecting struct, got %v", elem.Kind())
		return nil, "", err
	}

	// need to check exported fields, not just the number of fields
	for i := 0; i < elem.NumField(); i++ {
		// fmt.Printf("name:'%#v' pkgpath:'%#v' isexported:%v\n",
		// 	elem.Type().Field(i).Name, elem.Type().Field(i).PkgPath,
		// 	elem.Type().Field(i).IsExported())
		if elem.Type().Field(i).IsExported() {
			entryMap[elem.Type().Field(i).Name] = elem.Field(i).Interface()
		}
	}

	if len(entryMap) <= 1 {
		err = fmt.Errorf("minimum two exported fields needed, got %v", len(entryMap))
		return nil, "", err
	} else if hashInterface, exists := entryMap["Hash"]; exists == false {
		err = errors.New("entry missing hash field; must be included to insert")
		return nil, "", err
	} else if hash, succ = hashInterface.(string); succ == false {
		err = errors.New("hash should be a string")
		return nil, "", err
	} else if hash == "" {
		err = errors.New("hash cannot be empty")
		return nil, "", err
	}

	return entryMap, hash, nil
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
	doc, err := db.FindById(c.name, id)
	if err != nil {
		return nil, fmt.Errorf("error in query: %w", err)
	} else if doc == nil {
		return nil, ErrorDoesNotExist{"id not found"}
	}

	return doc, nil
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
