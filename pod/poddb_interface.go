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
	Where(query any, args ...any) gormDBInterface
	Preload(query string, args ...any) gormDBInterface
	Order(value any) gormDBInterface
	Limit(limit int) gormDBInterface
	Session(config *gorm.Session) gormDBInterface
	Debug() gormDBInterface
	Unscoped() gormDBInterface
	Model(value any) gormDBInterface
	Raw(sql string, values ...any) gormDBInterface

	// finisher methods, return gorm.DB directly (no more chaining)
	AutoMigrate(dst ...any) error
	FirstOrCreate(dest any, conds ...any) *gorm.DB
	First(dest any, conds ...any) *gorm.DB
	Find(dest any, conds ...any) *gorm.DB
	Save(value any) *gorm.DB
	Delete(value any, conds ...any) *gorm.DB
	Count(*int64) *gorm.DB
	Scan(dest any) *gorm.DB
	Exec(sql string, values ...any) *gorm.DB
}
type gormDBImpl struct {
	*gorm.DB
}


func (gdbi *gormDBImpl) AutoMigrate(dst ...any) error {
	return gdbi.DB.AutoMigrate(dst...)
}
func (gdbi *gormDBImpl) Where(query any, args ...any) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Where(query, args...)}
}
func (gdbi *gormDBImpl) Preload(query string, args ...any) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Preload(query, args...)}
}
func (gdbi *gormDBImpl) Order(value any) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Order(value)}
}
func (gdbi *gormDBImpl) Limit(limit int) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Limit(limit)}
}
func (gdbi *gormDBImpl) Session(config *gorm.Session) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Session(config)}
}
func (gdbi *gormDBImpl) Debug() gormDBInterface {
	return &gormDBImpl{gdbi.DB.Debug()}
}
func (gdbi *gormDBImpl) Unscoped() gormDBInterface {
	return &gormDBImpl{gdbi.DB.Unscoped()}
}
func (gdbi *gormDBImpl) Model(value any) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Model(value)}
}
func (gdbi *gormDBImpl) Raw(sql string, values ...any) gormDBInterface {
	return &gormDBImpl{gdbi.DB.Raw(sql, values...)}
}

// finisher methods
func (gdbi *gormDBImpl) FirstOrCreate(dest any, conds ...any) *gorm.DB {
	return gdbi.DB.FirstOrCreate(dest, conds...)
}
func (gdbi *gormDBImpl) First(dest any, conds ...any) *gorm.DB {
	return gdbi.DB.First(dest, conds...)
}
func (gdbi *gormDBImpl) Find(dest any, conds ...any) *gorm.DB {
	return gdbi.DB.Find(dest, conds...)
}
func (gdbi *gormDBImpl) Save(value any) *gorm.DB {
	return gdbi.DB.Save(value)
}
func (gdbi *gormDBImpl) Delete(value any, conds ...any) *gorm.DB {
	return gdbi.DB.Delete(value, conds...)
}
func (gdbi *gormDBImpl) Count(count *int64) *gorm.DB {
	return gdbi.DB.Count(count)
}
func (gdbi *gormDBImpl) Scan(dest any) *gorm.DB {
	return gdbi.DB.Scan(dest)
}
func (gdbi *gormDBImpl) Exec(sql string, values ...any) *gorm.DB {
	return gdbi.DB.Exec(sql, values...)
}
