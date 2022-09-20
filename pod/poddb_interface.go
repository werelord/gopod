package pod

import (
	"gorm.io/gorm"
)

// functions that will be overwritten by tests (only)
// all other references should be handled directly by db returned
type gormInterface interface {
	Open(gorm.Dialector, ...gorm.Option) (gormDBInterface, error)
}

// type matching interface
type gormImpl struct{}

func (gi *gormImpl) Open(d gorm.Dialector, opt ...gorm.Option) (gormDBInterface, error) {
	db, err := gorm.Open(d, opt...)
	return &gormDBImpl{db}, err
}

// concrete reference for use
var gImpl gormInterface = &gormImpl{}

// interface for db
type gormDBInterface interface {
	// chain methods
	Where(query any, args ...any) *gormDBImpl
	Preload(query string, args ...any) *gormDBImpl
	Order(value any) *gormDBImpl
	Limit(limit int) *gormDBImpl
	Session(config *gorm.Session) *gormDBImpl

	// finisher methods
	AutoMigrate(dst ...any) error
	// Find(dest any, conds ...any) *gormDBImpl
	// FirstOrCreate(any, ...any) gormDBImpl
	// First(any, ...any) gormDBImpl
	// Save(any) gormDBImpl
}
type gormDBImpl struct {
	*gorm.DB
}

//	func (gdbi *gormDBImpl) AutoMigrate(dst ...any) error {
//		return gdbi.AutoMigrate(dst...)
//	}
func (gdbi *gormDBImpl) Where(query any, args ...any) *gormDBImpl { return &gormDBImpl{gdbi.DB.Where(query, args...)} }
func (gdbi *gormDBImpl) Preload(query string, args ...any) *gormDBImpl { return &gormDBImpl{gdbi.DB.Preload(query, args...)} }
func (gdbi *gormDBImpl) Order(value any) *gormDBImpl { return &gormDBImpl{gdbi.DB.Order(value)} }
func (gdbi *gormDBImpl) Limit(limit int) *gormDBImpl { return &gormDBImpl{gdbi.DB.Limit(limit)} }
func (gdbi *gormDBImpl) Session(config *gorm.Session) *gormDBImpl { return &gormDBImpl{gdbi.DB.Session(config)} }

// func (gdbi *gormDBImpl) Find(dest any, conds ...any) *gormDBImpl {
// 	return &gormDBImpl{gdbi.DB.Find(dest, conds...)}
// }
