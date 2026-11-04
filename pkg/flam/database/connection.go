package database

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewConnection returns the [flam.IDatabaseConnection] of the id over db,
// to store in an [flam.IDatabaseConnectionFactory] a connection opened
// elsewhere. It fails with ErrNilDB when db is nil.
//
// The connection owns the pool of db: it implements io.Closer, closing the
// pool, so that the factory closes it when removing it. The connections
// derived from it, by WithConnection, WithPublisher, Apply and
// ApplyFilters, share the pool and do not implement io.Closer.
func NewConnection(
	id string,
	db *gorm.DB,
) (flam.IDatabaseConnection, error) {
	if db == nil {
		return nil, newErrNil(ErrNilDB, connectionNewMessage, id)
	}
	return &ownedConnection{connection: &connection{id: id, db: db}}, nil
}

// connection is the [flam.IDatabaseConnection] returned by the methods
// deriving a connection, sharing the pool of the one it derives from.
type connection struct {
	id string
	db *gorm.DB
}

// ownedConnection is the [flam.IDatabaseConnection] returned by
// [NewConnection], and created by the connection creator, owning its pool.
type ownedConnection struct {
	*connection
}

// compile time checks that *connection and *ownedConnection implement
// IDatabaseConnection
var (
	_ flam.IDatabaseConnection = (*connection)(nil)
	_ flam.IDatabaseConnection = (*ownedConnection)(nil)
)

// Close closes the pool of the connection, which also ends the
// connections derived from it.
func (c *ownedConnection) Close() error {
	db, err := c.db.DB()
	if err != nil {
		return newErrClose(connectionCloseMessage, c.id, err)
	}
	if err := db.Close(); err != nil {
		return newErrClose(connectionCloseMessage, c.id, err)
	}
	return nil
}

func (c *connection) Id() string {
	return c.id
}

func (c *connection) Gorm() *gorm.DB {
	return c.db
}

func (c *connection) WithConnection(
	tx *gorm.DB,
) flam.IDatabaseConnection {
	return c.derive(tx)
}

func (c *connection) WithPublisher(
	publisher flam.IPubSub[string, flam.RecordEvent],
) flam.IDatabaseConnection {
	ctx := c.db.Statement.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return c.derive(c.db.WithContext(context.WithValue(ctx, flam.ContextPublisherKey, publisher)))
}

func (c *connection) GetPublisher() flam.IPubSub[string, flam.RecordEvent] {
	if c.db.Statement.Context == nil {
		return nil
	}
	publisher, _ := c.db.Statement.Context.Value(flam.ContextPublisherKey).(flam.IPubSub[string, flam.RecordEvent])
	return publisher
}

func (c *connection) ApplyFilters(
	request flam.IRequest,
) flam.IDatabaseConnection {
	return c.derive(request.FilterScope(c.db))
}

func (c *connection) Apply(
	request flam.IRequest,
) flam.IDatabaseConnection {
	return c.derive(request.Scope(c.db))
}

func (c *connection) AddError(
	e error,
) error {
	return c.db.AddError(e)
}

func (c *connection) Assign(
	attrs ...any,
) *gorm.DB {
	return c.db.Assign(attrs...)
}

func (c *connection) Association(
	column string,
) *gorm.Association {
	return c.db.Association(column)
}

func (c *connection) Attrs(
	attrs ...any,
) *gorm.DB {
	return c.db.Attrs(attrs...)
}

func (c *connection) AutoMigrate(
	dst ...any,
) error {
	return c.db.AutoMigrate(dst...)
}

func (c *connection) Begin(
	opts ...*sql.TxOptions,
) *gorm.DB {
	return c.db.Begin(opts...)
}

func (c *connection) Clauses(
	conds ...clause.Expression,
) *gorm.DB {
	return c.db.Clauses(conds...)
}

func (c *connection) Commit() *gorm.DB {
	return c.db.Commit()
}

func (c *connection) Connection(
	fc func(*gorm.DB) error,
) error {
	return c.db.Connection(fc)
}

func (c *connection) Count(
	count *int64,
) *gorm.DB {
	return c.db.Count(count)
}

func (c *connection) Create(
	value any,
) *gorm.DB {
	return c.db.Create(value)
}

func (c *connection) CreateInBatches(
	value any,
	batchSize int,
) *gorm.DB {
	return c.db.CreateInBatches(value, batchSize)
}

func (c *connection) DB() (*sql.DB, error) {
	return c.db.DB()
}

func (c *connection) Debug() *gorm.DB {
	return c.db.Debug()
}

func (c *connection) Delete(
	value any,
	conds ...any,
) *gorm.DB {
	return c.db.Delete(value, conds...)
}

func (c *connection) Distinct(
	args ...any,
) *gorm.DB {
	return c.db.Distinct(args...)
}

func (c *connection) Exec(
	sql string,
	values ...any,
) *gorm.DB {
	return c.db.Exec(sql, values...)
}

func (c *connection) Find(
	dest any,
	conds ...any,
) *gorm.DB {
	return c.db.Find(dest, conds...)
}

func (c *connection) FindInBatches(
	dest any,
	batchSize int,
	fc func(tx *gorm.DB, batch int) error,
) *gorm.DB {
	return c.db.FindInBatches(dest, batchSize, fc)
}

func (c *connection) First(
	dest any,
	conds ...any,
) *gorm.DB {
	return c.db.First(dest, conds...)
}

func (c *connection) FirstOrCreate(
	dest any,
	conds ...any,
) *gorm.DB {
	return c.db.FirstOrCreate(dest, conds...)
}

func (c *connection) FirstOrInit(
	dest any,
	conds ...any,
) *gorm.DB {
	return c.db.FirstOrInit(dest, conds...)
}

func (c *connection) Get(
	key string,
) (any, bool) {
	return c.db.Get(key)
}

func (c *connection) Group(
	name string,
) *gorm.DB {
	return c.db.Group(name)
}

func (c *connection) Having(
	query any,
	args ...any,
) *gorm.DB {
	return c.db.Having(query, args...)
}

func (c *connection) InnerJoins(
	query string,
	args ...any,
) *gorm.DB {
	return c.db.InnerJoins(query, args...)
}

func (c *connection) InstanceGet(
	key string,
) (any, bool) {
	return c.db.InstanceGet(key)
}

func (c *connection) InstanceSet(
	key string,
	value any,
) *gorm.DB {
	return c.db.InstanceSet(key, value)
}

func (c *connection) Joins(
	query string,
	args ...any,
) *gorm.DB {
	return c.db.Joins(query, args...)
}

func (c *connection) Last(
	dest any,
	conds ...any,
) *gorm.DB {
	return c.db.Last(dest, conds...)
}

func (c *connection) Limit(
	limit int,
) *gorm.DB {
	return c.db.Limit(limit)
}

func (c *connection) MapColumns(
	m map[string]string,
) *gorm.DB {
	return c.db.MapColumns(m)
}

func (c *connection) Migrator() gorm.Migrator {
	return c.db.Migrator()
}

func (c *connection) Model(
	value any,
) *gorm.DB {
	return c.db.Model(value)
}

func (c *connection) Name() string {
	return c.db.Name()
}

func (c *connection) Not(
	query any,
	args ...any,
) *gorm.DB {
	return c.db.Not(query, args...)
}

func (c *connection) Offset(
	offset int,
) *gorm.DB {
	return c.db.Offset(offset)
}

func (c *connection) Omit(
	columns ...string,
) *gorm.DB {
	return c.db.Omit(columns...)
}

func (c *connection) Or(
	query any,
	args ...any,
) *gorm.DB {
	return c.db.Or(query, args...)
}

func (c *connection) Order(
	value any,
) *gorm.DB {
	return c.db.Order(value)
}

func (c *connection) Pluck(
	column string,
	dest any,
) *gorm.DB {
	return c.db.Pluck(column, dest)
}

func (c *connection) Preload(
	query string,
	args ...any,
) *gorm.DB {
	return c.db.Preload(query, args...)
}

func (c *connection) Raw(
	sql string,
	values ...any,
) *gorm.DB {
	return c.db.Raw(sql, values...)
}

func (c *connection) Rollback() *gorm.DB {
	return c.db.Rollback()
}

func (c *connection) RollbackTo(
	name string,
) *gorm.DB {
	return c.db.RollbackTo(name)
}

func (c *connection) Row() *sql.Row {
	return c.db.Row()
}

func (c *connection) Rows() (*sql.Rows, error) {
	return c.db.Rows()
}

func (c *connection) Save(
	value any,
) *gorm.DB {
	return c.db.Save(value)
}

func (c *connection) SavePoint(
	name string,
) *gorm.DB {
	return c.db.SavePoint(name)
}

func (c *connection) Scan(
	dest any,
) *gorm.DB {
	return c.db.Scan(dest)
}

func (c *connection) ScanRows(
	rows *sql.Rows,
	dest any,
) error {
	return c.db.ScanRows(rows, dest)
}

func (c *connection) Scopes(
	funcs ...func(*gorm.DB) *gorm.DB,
) *gorm.DB {
	return c.db.Scopes(funcs...)
}

func (c *connection) Select(
	query any,
	args ...any,
) *gorm.DB {
	return c.db.Select(query, args...)
}

func (c *connection) Session(
	config *gorm.Session,
) *gorm.DB {
	return c.db.Session(config)
}

func (c *connection) Set(
	key string,
	value any,
) *gorm.DB {
	return c.db.Set(key, value)
}

func (c *connection) SetupJoinTable(
	model any,
	field string,
	joinTable any,
) error {
	return c.db.SetupJoinTable(model, field, joinTable)
}

func (c *connection) Table(
	name string,
	args ...any,
) *gorm.DB {
	return c.db.Table(name, args...)
}

func (c *connection) Take(
	dest any,
	conds ...any,
) *gorm.DB {
	return c.db.Take(dest, conds...)
}

func (c *connection) ToSQL(
	queryFn func(tx *gorm.DB) *gorm.DB,
) string {
	return c.db.ToSQL(queryFn)
}

func (c *connection) Transaction(
	fc func(tx *gorm.DB) error,
	opts ...*sql.TxOptions,
) error {
	return c.db.Transaction(fc, opts...)
}

func (c *connection) Unscoped() *gorm.DB {
	return c.db.Unscoped()
}

func (c *connection) Update(
	column string,
	value any,
) *gorm.DB {
	return c.db.Update(column, value)
}

func (c *connection) UpdateColumn(
	column string,
	value any,
) *gorm.DB {
	return c.db.UpdateColumn(column, value)
}

func (c *connection) UpdateColumns(
	values any,
) *gorm.DB {
	return c.db.UpdateColumns(values)
}

func (c *connection) Updates(
	values any,
) *gorm.DB {
	return c.db.Updates(values)
}

func (c *connection) Use(
	plugin gorm.Plugin,
) error {
	return c.db.Use(plugin)
}

func (c *connection) Where(
	query any,
	args ...any,
) *gorm.DB {
	return c.db.Where(query, args...)
}

func (c *connection) WithContext(
	ctx context.Context,
) *gorm.DB {
	return c.db.WithContext(ctx)
}

// derive returns the connection of tx, sharing the pool and the id of c.
// tx is made a new session, so that each query chained from the connection
// starts from its conditions, instead of adding to the ones of the
// previous queries, and the connection can be used concurrently.
func (c *connection) derive(
	tx *gorm.DB,
) flam.IDatabaseConnection {
	return &connection{id: c.id, db: tx.Session(&gorm.Session{})}
}

// Messages of the errors of the connections.
const (
	connectionNewMessage   = "unable to create database connection"
	connectionCloseMessage = "unable to close database connection"
)
