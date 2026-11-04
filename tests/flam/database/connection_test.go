package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"regexp"
	"sync"
	"testing"

	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/pubsub"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

// item is the model of the tests.
type item struct {
	flam.Record
	Name string
	Size int
}

// openDB returns a gorm database over a new SQLite file, with the item
// table, closed at the end of the test.
func openDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(gormsqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: logger.Discard,
	})
	testutil.NoError(t, err)
	testutil.NoError(t, db.AutoMigrate(&item{}))
	t.Cleanup(func() {
		if pool, err := db.DB(); err == nil {
			_ = pool.Close()
		}
	})
	return db
}

// newConnection returns the connection "main" over a new SQLite database.
func newConnection(t *testing.T) (flam.IDatabaseConnection, *gorm.DB) {
	t.Helper()
	db := openDB(t)
	conn, err := database.NewConnection("main", db)
	testutil.NoError(t, err)
	return conn, db
}

// recordPublisher is a flam.IPubSub recording the published events.
type recordPublisher struct {
	flam.IPubSub[string, flam.RecordEvent]

	channels []string
}

func (p *recordPublisher) Publish(_ context.Context, channel string, _ flam.RecordEvent) error {
	p.channels = append(p.channels, channel)
	return nil
}

func Test_NewConnection(t *testing.T) {
	t.Run("should fail with a nil database", func(t *testing.T) {
		conn, err := database.NewConnection("main", nil)

		testutil.Equal(t, "conn", conn, flam.IDatabaseConnection(nil))
		testutil.Error(t, err, database.ErrNilDB, "unable to create database connection: nil gorm database", map[string]any{
			database.ErrorFieldID: "main",
		})
	})

	t.Run("should hold the id and the database", func(t *testing.T) {
		conn, db := newConnection(t)

		testutil.Equal(t, "Id()", conn.Id(), "main")
		testutil.Equal(t, "Gorm()", conn.Gorm() == db, true)
	})

	t.Run("should close the pool, once", func(t *testing.T) {
		conn, db := newConnection(t)
		closer, ok := conn.(io.Closer)
		testutil.Equal(t, "io.Closer", ok, true)

		testutil.NoError(t, closer.Close())
		pool, _ := db.DB()
		testutil.Equal(t, "Ping()", pool.Ping() != nil, true)
		testutil.NoError(t, closer.Close())
	})

	t.Run("should fail to close a database without pool", func(t *testing.T) {
		db, err := gorm.Open(gormsqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Discard})
		testutil.NoError(t, err)
		pool, _ := db.DB()
		t.Cleanup(func() { _ = pool.Close() })
		db.ConnPool = nil
		db.Statement.ConnPool = nil

		conn, err := database.NewConnection("main", db)
		testutil.NoError(t, err)
		testutil.Error(t, conn.(io.Closer).Close(), gorm.ErrInvalidDB, "unable to close database connection: invalid db", map[string]any{
			database.ErrorFieldID: "main",
		})
	})

	t.Run("should report the error of closing the pool", func(t *testing.T) {
		db, err := gorm.Open(gormsqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: logger.Discard})
		testutil.NoError(t, err)
		pool, _ := db.DB()
		t.Cleanup(func() { _ = pool.Close() })
		db.ConnPool = failingPool{pool}
		db.Statement.ConnPool = db.ConnPool

		conn, err := database.NewConnection("main", db)
		testutil.NoError(t, err)
		testutil.Error(t, conn.(io.Closer).Close(), errBoom, "unable to close database connection: boom", map[string]any{
			database.ErrorFieldID: "main",
		})
	})
}

var errBoom = errors.New("boom")

// failingPool is a gorm connection pool whose *sql.DB fails to close, as
// its connector does.
type failingPool struct {
	*sql.DB
}

func (p failingPool) GetDBConn() (*sql.DB, error) {
	return sql.OpenDB(failingConnector{}), nil
}

// failingConnector is a driver.Connector failing to close.
type failingConnector struct{}

func (failingConnector) Connect(context.Context) (driver.Conn, error) { return nil, errBoom }
func (failingConnector) Driver() driver.Driver                        { return nil }
func (failingConnector) Close() error                                 { return errBoom }

func Test_Connection_Derived(t *testing.T) {
	t.Run("should derive connections sharing the pool, without closing it", func(t *testing.T) {
		conn, db := newConnection(t)
		tx := db.Where("size > ?", 1)

		derived := []flam.IDatabaseConnection{
			conn.WithConnection(tx),
			conn.WithPublisher(&recordPublisher{}),
			conn.Apply(&database.Request{}),
			conn.ApplyFilters(&database.Request{}),
		}
		for i, d := range derived {
			name := fmt.Sprintf("derived %d", i)
			testutil.Equal(t, name+" Id()", d.Id(), "main")
			_, closer := d.(io.Closer)
			testutil.Equal(t, name+" io.Closer", closer, false)
			pool, _ := d.DB()
			own, _ := db.DB()
			testutil.Equal(t, name+" pool", pool == own, true)
		}
		testutil.Equal(t, "WithConnection conditions", derived[0].Gorm().Statement.Clauses["WHERE"], tx.Statement.Clauses["WHERE"])
		testutil.Equal(t, "WithConnection pool", derived[0].Gorm().Statement.ConnPool == tx.Statement.ConnPool, true)
		testutil.Equal(t, "receiver untouched", conn.Gorm() == db, true)
	})

	t.Run("should apply the request scopes", func(t *testing.T) {
		conn, _ := newConnection(t)
		for _, name := range []string{"b", "a", "c"} {
			testutil.NoError(t, conn.Create(&item{Name: name, Size: len(name)}).Error)
		}
		req := &database.Request{Limit: new(int)}
		*req.Limit = 2
		req.AddFilter("name", "<>", "c").AddSort("name", flam.RequestSortDesc)

		var all []item
		testutil.NoError(t, conn.Apply(req).Find(&all).Error)
		testutil.Equal(t, "Apply", []string{all[0].Name, all[1].Name}, []string{"b", "a"})

		var count int64
		testutil.NoError(t, conn.ApplyFilters(req).Model(&item{}).Count(&count).Error)
		testutil.Equal(t, "ApplyFilters", count, int64(2))
	})

	t.Run("should not carry the conditions of a query to the next", func(t *testing.T) {
		conn, _ := newConnection(t)
		for _, name := range []string{"a", "b", "c"} {
			testutil.NoError(t, conn.Create(&item{Name: name}).Error)
		}
		filtered := conn.ApplyFilters((&database.Request{}).AddFilter("name", "<>", "c"))

		var some, all []item
		testutil.NoError(t, filtered.Where("name = ?", "a").Find(&some).Error)
		testutil.NoError(t, filtered.Find(&all).Error)
		testutil.Equal(t, "first query", len(some), 1)
		testutil.Equal(t, "next query", len(all), 2)
	})

	t.Run("should be used concurrently", func(t *testing.T) {
		conn, _ := newConnection(t)
		testutil.NoError(t, conn.Create(&item{Name: "a"}).Error)
		applied := conn.Apply((&database.Request{}).AddFilter("name", "=", "a"))

		var wg sync.WaitGroup
		for range 4 {
			wg.Go(func() {
				var found []item
				if err := applied.Find(&found).Error; err != nil || len(found) != 1 {
					t.Errorf("Find() = %d, %v, want 1 row", len(found), err)
				}
			})
		}
		wg.Wait()
	})

	t.Run("should carry the publisher to the record hooks", func(t *testing.T) {
		conn, _ := newConnection(t)
		publisher := &recordPublisher{}

		testutil.Equal(t, "no publisher", conn.GetPublisher(), flam.IPubSub[string, flam.RecordEvent](nil))
		published := conn.WithPublisher(publisher)
		testutil.Equal(t, "GetPublisher()", published.GetPublisher(), flam.IPubSub[string, flam.RecordEvent](publisher))
		testutil.Equal(t, "Apply keeps it", published.Apply(&database.Request{}).GetPublisher() != nil, true)

		testutil.NoError(t, published.Create(&item{Name: "a"}).Error)
		testutil.Equal(t, "channels", publisher.channels, []string{
			"flam.database.record.items.before_save",
			"flam.database.record.items.before_create",
			"flam.database.record.items.after_create",
			"flam.database.record.items.after_save",
		})
		testutil.NoError(t, conn.Create(&item{Name: "b"}).Error)
		testutil.Equal(t, "receiver without publisher", len(publisher.channels), 4)
	})

	t.Run("should deliver the events through a pubsub", func(t *testing.T) {
		conn, _ := newConnection(t)
		ps := pubsub.New[string, flam.RecordEvent]()
		names := make(chan string, 1)
		testutil.NoError(t, ps.Subscribe("sub", "flam.database.record.items.after_create",
			func(_ context.Context, _ string, e flam.RecordEvent) error {
				names <- e.Record.(item).Name
				return nil
			}, nil))

		testutil.NoError(t, conn.WithPublisher(ps).Create(&item{Name: "a"}).Error)
		testutil.Equal(t, "delivered", <-names, "a")
		testutil.NoError(t, ps.Close())
	})

	t.Run("should use the background context of a statement without one", func(t *testing.T) {
		conn, db := newConnection(t)
		tx := db.Session(&gorm.Session{})
		tx.Statement.Context = nil
		bare := conn.WithConnection(tx)

		testutil.Equal(t, "GetPublisher()", bare.GetPublisher(), flam.IPubSub[string, flam.RecordEvent](nil))
		publisher := &recordPublisher{}
		testutil.Equal(t, "WithPublisher()", bare.WithPublisher(publisher).GetPublisher() != nil, true)
	})
}

// passthroughs are the methods of flam.IDatabaseConnection calling the
// *gorm.DB method of the same name.
func passthroughs() []string {
	own := map[string]bool{
		"Id": true, "Gorm": true, "WithConnection": true, "WithPublisher": true,
		"GetPublisher": true, "ApplyFilters": true, "Apply": true,
	}
	var names []string
	typ := reflect.TypeFor[flam.IDatabaseConnection]()
	for i := range typ.NumMethod() {
		if name := typ.Method(i).Name; !own[name] {
			names = append(names, name)
		}
	}
	return names
}

// argsOf returns the arguments of a call of the method: a callback
// returning its zero results, or its *gorm.DB argument, a new value for a
// pointer, an empty item slice for an any, and the zero value of any other
// parameter, no variadic one.
func argsOf(method reflect.Type) []reflect.Value {
	n := method.NumIn()
	if method.IsVariadic() {
		n--
	}
	args := make([]reflect.Value, 0, n)
	for i := range n {
		in := method.In(i)
		switch {
		case in.Kind() == reflect.Pointer:
			args = append(args, reflect.New(in.Elem()))
			continue
		case in.Kind() == reflect.Interface && in.NumMethod() == 0:
			args = append(args, reflect.ValueOf(&[]item{}))
			continue
		case in.Kind() != reflect.Func:
			args = append(args, reflect.Zero(in))
			continue
		}
		args = append(args, reflect.MakeFunc(in, func(params []reflect.Value) []reflect.Value {
			results := make([]reflect.Value, in.NumOut())
			for j := range results {
				results[j] = reflect.Zero(in.Out(j))
				if in.Out(j) == params[0].Type() {
					results[j] = params[0]
				}
			}
			return results
		}))
	}
	return args
}

// address matches the pointer addresses of the error texts, which differ
// between two databases.
var address = regexp.MustCompile(`0x[0-9a-f]+`)

// summary describes a call result for comparison: the error and SQL of a
// *gorm.DB, the text of an error, its addresses masked, the type of a
// value bound to its database, or the value.
func summary(v reflect.Value) any {
	if !v.IsValid() || (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) && v.IsNil() {
		return nil
	}
	switch r := v.Interface().(type) {
	case *gorm.DB:
		return address.ReplaceAllString(fmt.Sprintf("%v | %s | %d", r.Error, r.Statement.SQL.String(), len(r.Statement.Clauses)), "0x")
	case error:
		return address.ReplaceAllString(r.Error(), "0x")
	case *sql.DB, *sql.Row, *sql.Rows, *gorm.Association, gorm.Migrator:
		return reflect.TypeOf(r).String()
	}
	return v.Interface()
}

// call calls the method and returns the summaries of its results, or of
// its panic.
func call(method reflect.Value, args []reflect.Value) (out []any) {
	defer func() {
		if r := recover(); r != nil {
			out = []any{fmt.Sprintf("panic: %v", r)}
		}
	}()
	for _, result := range method.Call(args) {
		out = append(out, summary(result))
	}
	return out
}

func Test_Connection_Passthrough(t *testing.T) {
	names := passthroughs()
	testutil.Equal(t, "count", len(names), 68)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			conn, _ := newConnection(t)
			db := openDB(t)
			got := reflect.ValueOf(conn).MethodByName(name)
			want := reflect.ValueOf(db).MethodByName(name)
			testutil.Equal(t, "gorm method", want.IsValid(), true)

			gotOut := call(got, argsOf(got.Type()))
			wantOut := call(want, argsOf(want.Type()))
			testutil.Equal(t, "results", gotOut, wantOut)
		})
	}

	t.Run("should run statements on the database", func(t *testing.T) {
		conn, _ := newConnection(t)
		testutil.NoError(t, conn.Create(&item{Name: "a", Size: 1}).Error)
		testutil.NoError(t, conn.Model(&item{}).Where("name = ?", "a").Update("size", 5).Error)

		var got item
		testutil.NoError(t, conn.First(&got).Error)
		testutil.Equal(t, "Size", got.Size, 5)
		testutil.NoError(t, conn.Transaction(func(tx *gorm.DB) error {
			return conn.WithConnection(tx).Delete(&got).Error
		}))
		var count int64
		testutil.NoError(t, conn.Model(&item{}).Count(&count).Error)
		testutil.Equal(t, "Count", count, int64(0))
	})
}
