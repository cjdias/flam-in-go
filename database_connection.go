package flam

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DatabaseConnection interface {
	AddError(e error) error
	Assign(attrs ...interface{}) *gorm.DB
	Association(column string) *gorm.Association
	Attrs(attrs ...interface{}) *gorm.DB
	AutoMigrate(dst ...interface{}) error
	Begin(opts ...*sql.TxOptions) *gorm.DB
	Clauses(conds ...clause.Expression) *gorm.DB
	Commit() *gorm.DB
	Connection(fc func(*gorm.DB) error) error
	Count(count *int64) *gorm.DB
	Create(value interface{}) *gorm.DB
	CreateInBatches(value interface{}, batchSize int) *gorm.DB
	DB() (*sql.DB, error)
	Debug() *gorm.DB
	Delete(value interface{}, conds ...interface{}) *gorm.DB
	Distinct(args ...interface{}) *gorm.DB
	Exec(sql string, values ...interface{}) *gorm.DB
	Find(dest interface{}, conds ...interface{}) *gorm.DB
	FindInBatches(dest interface{}, batchSize int, fc func(tx *gorm.DB, batch int) error) *gorm.DB
	First(dest interface{}, conds ...interface{}) *gorm.DB
	FirstOrCreate(dest interface{}, conds ...interface{}) *gorm.DB
	FirstOrInit(dest interface{}, conds ...interface{}) *gorm.DB
	Get(key string) (interface{}, bool)
	Group(name string) *gorm.DB
	Having(query interface{}, args ...interface{}) *gorm.DB
	InnerJoins(query string, args ...interface{}) *gorm.DB
	InstanceGet(key string) (interface{}, bool)
	InstanceSet(key string, value interface{}) *gorm.DB
	Joins(query string, args ...interface{}) *gorm.DB
	Last(dest interface{}, conds ...interface{}) *gorm.DB
	Limit(limit int) *gorm.DB
	MapColumns(m map[string]string) *gorm.DB
	Migrator() gorm.Migrator
	Model(value interface{}) *gorm.DB
	Not(query interface{}, args ...interface{}) *gorm.DB
	Offset(offset int) *gorm.DB
	Omit(columns ...string) *gorm.DB
	Or(query interface{}, args ...interface{}) *gorm.DB
	Order(value interface{}) *gorm.DB
	Pluck(column string, dest interface{}) *gorm.DB
	Preload(query string, args ...interface{}) *gorm.DB
	Raw(sql string, values ...interface{}) *gorm.DB
	Rollback() *gorm.DB
	RollbackTo(name string) *gorm.DB
	Row() *sql.Row
	Rows() (*sql.Rows, error)
	Save(value interface{}) *gorm.DB
	SavePoint(name string) *gorm.DB
	Scan(dest interface{}) *gorm.DB
	ScanRows(rows *sql.Rows, dest interface{}) error
	Scopes(funcs ...func(*gorm.DB) *gorm.DB) *gorm.DB
	Select(query interface{}, args ...interface{}) *gorm.DB
	Session(config *gorm.Session) *gorm.DB
	Set(key string, value interface{}) *gorm.DB
	SetupJoinTable(model interface{}, field string, joinTable interface{}) error
	Table(name string, args ...interface{}) *gorm.DB
	Take(dest interface{}, conds ...interface{}) *gorm.DB
	ToSQL(queryFn func(tx *gorm.DB) *gorm.DB) string
	Transaction(fc func(tx *gorm.DB) error, opts ...*sql.TxOptions) error
	Unscoped() *gorm.DB
	Update(column string, value interface{}) *gorm.DB
	UpdateColumn(column string, value interface{}) *gorm.DB
	UpdateColumns(values interface{}) *gorm.DB
	Updates(values interface{}) *gorm.DB
	Use(plugin gorm.Plugin) error
	Where(query interface{}, args ...interface{}) *gorm.DB
	WithContext(ctx context.Context) *gorm.DB
}
