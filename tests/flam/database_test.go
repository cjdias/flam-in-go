package flam

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/utils/tests"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

type published struct {
	channel string
	event   flam.RecordEvent
}

type recordPublisher struct {
	flam.IPubSub[string, flam.RecordEvent]

	err    error
	events []published
}

func (p *recordPublisher) Publish(_ context.Context, channel string, event flam.RecordEvent) error {
	p.events = append(p.events, published{channel, event})

	return p.err
}

func (p *recordPublisher) channels() []string {
	var channels []string
	for _, e := range p.events {
		channels = append(channels, e.channel)
	}

	return channels
}

type plainRecord struct {
	flam.Record
	Name string
}

type valuePrefixed struct {
	flam.Record
	Name string
}

func (valuePrefixed) ChannelPrefix() string { return "app.value" }

type pointerPrefixed struct {
	flam.Record
	Name string
}

func (*pointerPrefixed) ChannelPrefix() string { return "app.pointer" }

func recordDB(t *testing.T, publisher flam.IPubSub[string, flam.RecordEvent]) *gorm.DB {
	t.Helper()

	db, e := gorm.Open(tests.DummyDialector{}, &gorm.Config{DryRun: true})
	testutil.NoError(t, e)

	if publisher == nil {
		return db
	}

	return db.WithContext(context.WithValue(context.Background(), flam.ContextPublisherKey, publisher))
}

func Test_DatabaseConfig(t *testing.T) {
	t.Run("should hold the id and the gorm configuration", func(t *testing.T) {
		c := flam.NewDatabaseConfig("main", gorm.Config{SkipDefaultTransaction: true})

		testutil.Equal(t, "Id()", c.Id(), "main")
		testutil.Equal(t, "SkipDefaultTransaction", c.SkipDefaultTransaction, true)
	})
}

func Test_Record_NilCheck(t *testing.T) {
	now := time.Now()
	text := "x"
	var nilText *string
	var nilMap map[string]any
	var nilSlice []int
	var nilFunc func()
	var nilChan chan int

	cases := []struct {
		name  string
		value any
		want  any
	}{
		{"nil", nil, nil},
		{"valid bool", sql.NullBool{Bool: true, Valid: true}, true},
		{"invalid bool", sql.NullBool{}, nil},
		{"valid byte", sql.NullByte{Byte: 2, Valid: true}, byte(2)},
		{"invalid byte", sql.NullByte{}, nil},
		{"valid int16", sql.NullInt16{Int16: 3, Valid: true}, int16(3)},
		{"invalid int16", sql.NullInt16{}, nil},
		{"valid int32", sql.NullInt32{Int32: 4, Valid: true}, int32(4)},
		{"invalid int32", sql.NullInt32{}, nil},
		{"valid int64", sql.NullInt64{Int64: 5, Valid: true}, int64(5)},
		{"invalid int64", sql.NullInt64{}, nil},
		{"valid float64", sql.NullFloat64{Float64: 1.5, Valid: true}, 1.5},
		{"invalid float64", sql.NullFloat64{}, nil},
		{"valid string", sql.NullString{String: "s", Valid: true}, "s"},
		{"invalid string", sql.NullString{}, nil},
		{"valid time", sql.NullTime{Time: now, Valid: true}, now},
		{"invalid time", sql.NullTime{}, nil},
		{"pointer", &text, "x"},
		{"pointer to a null value", &sql.NullInt64{Int64: 7, Valid: true}, int64(7)},
		{"nil pointer", nilText, nil},
		{"nil map", nilMap, nil},
		{"nil slice", nilSlice, nil},
		{"nil func", nilFunc, nil},
		{"nil chan", nilChan, nil},
		{"slice", []int{1}, []int{1}},
		{"plain", 8, 8},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			testutil.Equal(t, "NilCheck()", flam.Record{}.NilCheck(c.value), c.want)
		})
	}
}

func Test_Record_Hooks(t *testing.T) {
	t.Run("should publish nothing without a publisher", func(t *testing.T) {
		db := recordDB(t, nil)

		testutil.NoError(t, db.Create(&plainRecord{Name: "a"}).Error)
	})

	t.Run("should publish nothing with a nil publisher", func(t *testing.T) {
		var publisher *recordPublisher
		db := recordDB(t, nil).WithContext(context.WithValue(context.Background(), flam.ContextPublisherKey, publisher))

		testutil.NoError(t, db.Create(&plainRecord{Name: "a"}).Error)
	})

	t.Run("should ignore a value stored under the bare key string", func(t *testing.T) {
		publisher := &recordPublisher{}
		//lint:ignore SA1029 the collision of the bare string key is what is tested
		db := recordDB(t, nil).WithContext(context.WithValue(context.Background(), "flam.database.context.publisher", flam.IPubSub[string, flam.RecordEvent](publisher))) //nolint:staticcheck // as above, for golangci-lint

		testutil.NoError(t, db.Create(&plainRecord{Name: "a"}).Error)
		testutil.Equal(t, "events", len(publisher.events), 0)
	})

	t.Run("should publish the create events on the table channels", func(t *testing.T) {
		publisher := &recordPublisher{}
		db := recordDB(t, publisher)
		rec := &plainRecord{Name: "a"}

		testutil.NoError(t, db.Create(rec).Error)
		testutil.Equal(t, "channels", publisher.channels(), []string{
			"flam.database.record.plain_records.before_save",
			"flam.database.record.plain_records.before_create",
			"flam.database.record.plain_records.after_create",
			"flam.database.record.plain_records.after_save",
		})
		testutil.Equal(t, "Record", publisher.events[0].event.Record.(plainRecord).Name, "a")
		testutil.Equal(t, "after create Id", publisher.events[2].event.Record.(plainRecord).Id, rec.Id)
		testutil.Equal(t, "Tx", publisher.events[0].event.Tx != nil, true)
	})

	t.Run("should publish the update and delete events", func(t *testing.T) {
		publisher := &recordPublisher{}
		db := recordDB(t, publisher)
		rec := &plainRecord{Record: flam.Record{Id: 1}, Name: "a"}

		testutil.NoError(t, db.Model(rec).Update("name", "b").Error)
		testutil.NoError(t, db.Delete(rec).Error)
		testutil.Equal(t, "channels", publisher.channels(), []string{
			"flam.database.record.plain_records.before_save",
			"flam.database.record.plain_records.before_update",
			"flam.database.record.plain_records.after_update",
			"flam.database.record.plain_records.after_save",
			"flam.database.record.plain_records.before_delete",
			"flam.database.record.plain_records.after_delete",
		})
	})

	t.Run("should publish each element of a batch", func(t *testing.T) {
		publisher := &recordPublisher{}
		db := recordDB(t, publisher)
		recs := []*plainRecord{{Name: "a"}, {Name: "b"}}

		testutil.NoError(t, db.Create(&recs).Error)
		var names []string
		for _, e := range publisher.events[:4] {
			names = append(names, e.event.Record.(plainRecord).Name)
		}
		testutil.Equal(t, "names", names, []string{"a", "a", "b", "b"})
	})

	t.Run("should use the prefix of the record, of a value or a pointer receiver", func(t *testing.T) {
		publisher := &recordPublisher{}
		db := recordDB(t, publisher)

		testutil.NoError(t, db.Create(&valuePrefixed{Name: "a"}).Error)
		testutil.NoError(t, db.Create(&pointerPrefixed{Name: "a"}).Error)
		testutil.Equal(t, "value", publisher.events[0].channel, "app.value.before_save")
		testutil.Equal(t, "pointer", publisher.events[4].channel, "app.pointer.before_save")
	})

	t.Run("should fail the statement when the event can't be published", func(t *testing.T) {
		boom := errors.New("boom")
		publisher := &recordPublisher{err: boom}
		db := recordDB(t, publisher)

		testutil.Equal(t, "Create()", errors.Is(db.Create(&plainRecord{Name: "a"}).Error, boom), true)

		tx := db.Session(&gorm.Session{})
		tx.Statement.Table = "items"
		testutil.Error(t, flam.Record{}.BeforeDelete(tx), boom, "unable to publish the record event: boom", map[string]any{
			flam.ErrorFieldChannel: "flam.database.record.items.before_delete",
		})
	})

	t.Run("should call the hooks directly on a statement without context or table", func(t *testing.T) {
		publisher := &recordPublisher{}
		db := recordDB(t, nil)

		testutil.NoError(t, flam.Record{}.BeforeSave(&gorm.DB{Statement: &gorm.Statement{}}))

		tx := db.Session(&gorm.Session{NewDB: true})
		tx.Statement.Context = context.WithValue(context.Background(), flam.ContextPublisherKey, flam.IPubSub[string, flam.RecordEvent](publisher))
		tx.Statement.Dest = "dest"
		testutil.NoError(t, flam.Record{}.AfterSave(tx))
		testutil.Equal(t, "events", publisher.events, []published{{
			channel: "flam.database.record.after_save",
			event:   flam.RecordEvent{Record: "dest", Tx: tx},
		}})
	})

	t.Run("should fall back to the destination for a missing element", func(t *testing.T) {
		publisher := &recordPublisher{}
		tx := recordDB(t, publisher).Session(&gorm.Session{})
		recs := []*plainRecord{nil}

		tx.Statement.Dest = &recs
		tx.Statement.ReflectValue = reflect.ValueOf(recs)
		testutil.NoError(t, flam.Record{}.AfterSave(tx))

		tx.Statement.CurDestIndex = 3
		testutil.NoError(t, flam.Record{}.AfterSave(tx))

		testutil.Equal(t, "nil element", publisher.events[0].event.Record, any(&recs))
		testutil.Equal(t, "out of range", publisher.events[1].event.Record, any(&recs))
	})
}
