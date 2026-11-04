package flam

import (
	"context"
	"database/sql"
	"reflect"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/cjdias/flam-in-go/pkg/flam/internal/value"
)

// DatabaseConfig is a gorm configuration held by an
// [IDatabaseConfigFactory], under its id, from which database connections
// are opened. It is a value: opening a connection with a copy of its
// Config, as gorm.Open changes the configuration it is given, leaves the
// one held by the factory untouched.
type DatabaseConfig struct {
	gorm.Config

	id string
}

// NewDatabaseConfig returns the DatabaseConfig of the id, holding config,
// for the creators of an [IDatabaseConfigFactory].
func NewDatabaseConfig(
	id string,
	config gorm.Config,
) DatabaseConfig {
	return DatabaseConfig{
		Config: config,
		id:     id,
	}
}

// Id returns the id of the configuration in its factory.
func (c DatabaseConfig) Id() string {
	return c.id
}

// IDatabaseConfigCreator creates the database configurations, for the
// [IDatabaseConfigFactory].
type IDatabaseConfigCreator = IResourceCreator[DatabaseConfig]

// IDatabaseConfigFactory creates and holds the gorm configurations of the
// database connections, one per id.
type IDatabaseConfigFactory = IFactory[DatabaseConfig]

// IDatabaseDialect is a gorm dialector, held by an
// [IDatabaseDialectFactory] under its id.
type IDatabaseDialect interface {
	gorm.Dialector
	IResource

	// Gorm returns a new gorm dialector of the driver, the value a
	// connection is opened with: gorm and its drivers check its type for
	// optional features, such as savepoints and error translation, which a
	// value wrapping it would hide, and change it while opening, so that
	// connections never share one.
	Gorm() gorm.Dialector
}

// IDatabaseDialectCreator creates the dialects of a database driver, for
// the [IDatabaseDialectFactory].
type IDatabaseDialectCreator = IResourceCreator[IDatabaseDialect]

// IDatabaseDialectFactory creates and holds the database dialects, one per
// id.
type IDatabaseDialectFactory = IFactory[IDatabaseDialect]

// RequestSortDirection is the direction an [IRequest] sorts rows in.
type RequestSortDirection string

// The directions of a sort.
const (
	// RequestSortAsc sorts in ascending order.
	RequestSortAsc RequestSortDirection = "asc"

	// RequestSortDesc sorts in descending order.
	RequestSortDesc RequestSortDirection = "desc"
)

// IRequest describes a query, its filters, sorts, relations to preload,
// offset and limit, implemented by
// [github.com/cjdias/flam-in-go/pkg/flam/database.Request], and applied to
// a connection by [IDatabaseConnection] Apply and ApplyFilters.
type IRequest interface {
	// AddFilter adds the filter of the field, comparing it to value with
	// the SQL operator, and returns the request.
	AddFilter(field string, operator string, value any) IRequest

	// AddTextFilter adds the filter of the text field, comparing it to
	// value with the operator, "like" when not given, ignoring case for the
	// like operators, and returns the request.
	AddTextFilter(field string, value string, op ...string) IRequest

	// AddSort adds the sort of the field, in the direction, RequestSortAsc
	// when not given, and returns the request.
	AddSort(field string, direction ...RequestSortDirection) IRequest

	// AddRelation adds the relation to preload, its rows meeting the
	// condition when given, and returns the request.
	AddRelation(relation string, condition ...string) IRequest

	// Scope returns tx narrowed by the whole request: filters, sorts,
	// relations, offset, limit and soft-deleted rows. As its signature is
	// the one of a gorm scope, it can be given to gorm.DB Scopes.
	Scope(tx *gorm.DB) *gorm.DB

	// FilterScope returns tx narrowed by the filters of the request, and
	// its soft-deleted rows, only, as for counting the rows of a page. As
	// its signature is the one of a gorm scope, it can be given to gorm.DB
	// Scopes.
	FilterScope(tx *gorm.DB) *gorm.DB
}

// IDatabaseConnection is a gorm database connection held by an
// [IDatabaseConnectionFactory] under its id.
//
// Beyond Id, Gorm and the methods following them, each method calls the
// method of the same name of the *gorm.DB returned by Gorm, which the
// gorm documentation describes.
//
// A connection is shared by every consumer taking it from its factory:
// WithConnection, WithPublisher, Apply and ApplyFilters return new
// connections, of the same id and the same pool of database connections,
// leaving the receiver untouched.
type IDatabaseConnection interface {
	IResource

	// Gorm returns the *gorm.DB of the connection.
	Gorm() *gorm.DB

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
	Name() string
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

	// WithConnection returns the connection of tx, such as the
	// transaction given to a Transaction callback, under the id of the
	// receiver.
	WithConnection(tx *gorm.DB) IDatabaseConnection

	// WithPublisher returns the connection publishing on publisher the
	// events of the [Record] values it handles, as described by Record.
	WithPublisher(publisher IPubSub[string, RecordEvent]) IDatabaseConnection

	// GetPublisher returns the publisher of the connection, or nil when it
	// has none.
	GetPublisher() IPubSub[string, RecordEvent]

	// ApplyFilters returns the connection narrowed by the request
	// FilterScope.
	ApplyFilters(request IRequest) IDatabaseConnection

	// Apply returns the connection narrowed by the request Scope.
	Apply(request IRequest) IDatabaseConnection
}

// IDatabaseConnectionCreator creates the database connections, for the
// [IDatabaseConnectionFactory].
type IDatabaseConnectionCreator = IResourceCreator[IDatabaseConnection]

// IDatabaseConnectionFactory creates and holds the database connections,
// one per id.
type IDatabaseConnectionFactory = IFactory[IDatabaseConnection]

// DatabaseContextKey is the type of the keys of the values flam stores in
// the context of a gorm statement, so that they never collide with the
// values other packages store under the same name.
type DatabaseContextKey string

// ContextPublisherKey is the key of the publisher of the [Record] events,
// an IPubSub[string, RecordEvent], in the context of a gorm statement.
const ContextPublisherKey DatabaseContextKey = "flam.database.context.publisher"

// RecordChannelDefaultPrefix is the prefix of the channels of the events of
// the records not implementing [IRecordChannelPrefixer], followed by "."
// and their table name, when known.
const RecordChannelDefaultPrefix = "flam.database.record"

// Suffixes of the channels of the [Record] events, one per gorm hook.
const (
	RecordChannelBeforeSave   = ".before_save"
	RecordChannelBeforeCreate = ".before_create"
	RecordChannelAfterCreate  = ".after_create"
	RecordChannelBeforeUpdate = ".before_update"
	RecordChannelAfterUpdate  = ".after_update"
	RecordChannelAfterSave    = ".after_save"
	RecordChannelBeforeDelete = ".before_delete"
	RecordChannelAfterDelete  = ".after_delete"
)

// ErrorFieldChannel is the context field, of the errors of the [Record]
// hooks, holding the channel of the event that failed to be published.
const ErrorFieldChannel = "channel"

// IRecordChannelPrefixer is implemented by a record embedding [Record] that
// owns the prefix of the channels of its events, such as "app.user", in
// place of [RecordChannelDefaultPrefix] and its table name. Record does not
// implement it, as every record embedding it would then implement it too.
type IRecordChannelPrefixer interface {
	// ChannelPrefix returns the prefix of the channels of the events of the
	// record.
	ChannelPrefix() string
}

// RecordEvent is the message of an event of a [Record], published on the
// channel of the event.
//
// The publisher delivers it once the hook returned, possibly once the
// statement or its transaction ended: Tx is the statement as it was when
// the hook was called, to read, such as its context, rather than to
// run other statements.
type RecordEvent struct {
	// Record is the record the hook was called on, the element handled for
	// a batch statement.
	Record any

	// Tx is the gorm statement that called the hook.
	Tx *gorm.DB
}

// Record is a base model embedding common primary key and timestamp fields.
//
// Its gorm hooks publish the events of the records embedding it, a
// [RecordEvent], with the publisher of the statement, the one of a
// connection returned by [IDatabaseConnection] WithPublisher, stored at
// [ContextPublisherKey] in the statement context, and do nothing without
// one. The channel of an event is the record channel prefix, from
// [IRecordChannelPrefixer] or [RecordChannelDefaultPrefix], followed by
// the suffix of the hook, such as [RecordChannelAfterCreate]. A hook fails,
// failing the statement, when the event can't be published.
type Record struct {
	Id        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NilCheck normalizes nullable and nil values into a plain value or nil,
// unwrapping sql.NullX wrappers and dereferencing pointers, so the result is
// always something sql.NullX.Scan accepts.
func (r Record) NilCheck(value any) any {
	if value == nil {
		return nil
	}

	switch typed := value.(type) {
	case sql.NullBool:
		if typed.Valid {
			return typed.Bool
		}
		return nil
	case sql.NullByte:
		if typed.Valid {
			return typed.Byte
		}
		return nil
	case sql.NullInt16:
		if typed.Valid {
			return typed.Int16
		}
		return nil
	case sql.NullInt32:
		if typed.Valid {
			return typed.Int32
		}
		return nil
	case sql.NullInt64:
		if typed.Valid {
			return typed.Int64
		}
		return nil
	case sql.NullFloat64:
		if typed.Valid {
			return typed.Float64
		}
		return nil
	case sql.NullString:
		if typed.Valid {
			return typed.String
		}
		return nil
	case sql.NullTime:
		if typed.Valid {
			return typed.Time
		}
		return nil
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return nil
		}

		return r.NilCheck(rv.Elem().Interface())
	case reflect.Map,
		reflect.Chan,
		reflect.Slice,
		reflect.Func,
		reflect.Interface:
		if rv.IsNil() {
			return nil
		}
	}

	return value
}

// BeforeSave publishes the record before it is saved.
func (r Record) BeforeSave(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelBeforeSave)
}

// BeforeCreate publishes the record before it is created.
func (r Record) BeforeCreate(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelBeforeCreate)
}

// AfterCreate publishes the record after it has been created.
func (r Record) AfterCreate(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelAfterCreate)
}

// BeforeUpdate publishes the record before it is updated.
func (r Record) BeforeUpdate(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelBeforeUpdate)
}

// AfterUpdate publishes the record after it has been updated.
func (r Record) AfterUpdate(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelAfterUpdate)
}

// AfterSave publishes the record after it has been saved.
func (r Record) AfterSave(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelAfterSave)
}

// BeforeDelete publishes the record before it is deleted.
func (r Record) BeforeDelete(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelBeforeDelete)
}

// AfterDelete publishes the record after it has been deleted.
func (r Record) AfterDelete(
	tx *gorm.DB,
) error {
	return r.publish(tx, RecordChannelAfterDelete)
}

// publish publishes the event of the hook, on the channel of the record
// the hook has been called on, with the publisher of the statement.
//
// As the hooks are promoted, the receiver is the embedded Record and not
// the record embedding it, so the record is taken from the statement.
func (r Record) publish(
	tx *gorm.DB,
	event string,
) error {
	if tx.Statement.Context == nil {
		return nil
	}

	publisher, ok := tx.Statement.Context.Value(ContextPublisherKey).(IPubSub[string, RecordEvent])
	if !ok || value.IsNil(publisher) {
		return nil
	}

	subject := recordSubject(tx)
	channel := recordChannelPrefix(tx, subject) + event
	msg := RecordEvent{
		Record: tx.Statement.Dest,
		Tx:     tx,
	}
	if subject.IsValid() {
		msg.Record = subject.Interface()
	}

	if e := publisher.Publish(tx.Statement.Context, channel, msg); e != nil {
		return NewWrappedError(e, "unable to publish the record event").
			With(ErrorFieldChannel, channel)
	}

	return nil
}

// recordSubject returns the record the hook has been called on, the
// element handled for a batch statement, or the invalid value when it
// can't be found.
func recordSubject(
	tx *gorm.DB,
) reflect.Value {
	subject := tx.Statement.ReflectValue

	switch subject.Kind() {
	case reflect.Slice, reflect.Array:
		if tx.Statement.CurDestIndex >= subject.Len() {
			return reflect.Value{}
		}

		subject = subject.Index(tx.Statement.CurDestIndex)
	}

	for subject.Kind() == reflect.Pointer || subject.Kind() == reflect.Interface {
		if subject.IsNil() {
			return reflect.Value{}
		}

		subject = subject.Elem()
	}

	return subject
}

// recordChannelPrefix returns the channel prefix of the record, from its
// ChannelPrefix method, of a value or a pointer receiver, or else
// RecordChannelDefaultPrefix followed by its table name, when known.
func recordChannelPrefix(
	tx *gorm.DB,
	subject reflect.Value,
) string {
	if subject.IsValid() {
		if prefixer, ok := subject.Interface().(IRecordChannelPrefixer); ok {
			return prefixer.ChannelPrefix()
		}

		if subject.CanAddr() {
			if prefixer, ok := subject.Addr().Interface().(IRecordChannelPrefixer); ok {
				return prefixer.ChannelPrefix()
			}
		}
	}

	if tx.Statement.Table == "" {
		return RecordChannelDefaultPrefix
	}

	return RecordChannelDefaultPrefix + "." + tx.Statement.Table
}
