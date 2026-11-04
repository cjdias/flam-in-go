package outbox

import (
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/config"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	dbstandard "github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/sqlite"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/outbox"
	"github.com/cjdias/flam-in-go/tests/flam/testutil"
)

func Test_Declarations(t *testing.T) {
	for _, pair := range [][2]string{
		{outbox.RelayConfigPath, "flam.outbox.relays"},
		{outbox.ConfigFieldConnection, "connection"},
		{outbox.ConfigFieldBroker, "broker"},
		{outbox.ConfigFieldProcess, "process"},
		{outbox.ConfigFieldPoll, "poll"},
		{outbox.ConfigFieldBatch, "batch"},
		{outbox.ConfigFieldLease, "lease"},
		{outbox.ConfigFieldBackoff, "backoff"},
		{outbox.ConfigFieldMaxBackoff, "max_backoff"},
		{outbox.ConfigFieldDelete, "delete"},
		{outbox.ConfigFieldRetention, "retention"},
		{outbox.ConfigFieldCleanup, "cleanup"},
		{outbox.ConfigFieldMigrate, "migrate"},
		{outbox.DefaultConnectionConfigPath, "flam.outbox.defaults.relays.connection"},
		{outbox.DefaultBrokerConfigPath, "flam.outbox.defaults.relays.broker"},
		{outbox.DefaultPollConfigPath, "flam.outbox.defaults.relays.poll"},
		{outbox.DefaultBatchConfigPath, "flam.outbox.defaults.relays.batch"},
		{outbox.DefaultLeaseConfigPath, "flam.outbox.defaults.relays.lease"},
		{outbox.DefaultBackoffConfigPath, "flam.outbox.defaults.relays.backoff"},
		{outbox.DefaultMaxBackoffConfigPath, "flam.outbox.defaults.relays.max_backoff"},
		{outbox.DefaultDeleteConfigPath, "flam.outbox.defaults.relays.delete"},
		{outbox.DefaultRetentionConfigPath, "flam.outbox.defaults.relays.retention"},
		{outbox.DefaultCleanupConfigPath, "flam.outbox.defaults.relays.cleanup"},
		{outbox.DefaultMigrateConfigPath, "flam.outbox.defaults.relays.migrate"},
		{outbox.Table, "flam_outbox"},
		{outbox.LogChannel, "flam.outbox"},
		{outbox.ErrorFieldID, "id"},
		{outbox.ErrorFieldField, "field"},
		{outbox.ErrorFieldConnection, "connection"},
		{outbox.ErrorFieldBroker, "broker"},
		{outbox.ErrorFieldTopic, "topic"},
		{outbox.ErrorFieldMessage, "message"},
	} {
		if pair[0] != pair[1] {
			t.Errorf("constant = %q, want %q", pair[0], pair[1])
		}
	}
	testutil.Equal(t, "TableName()", outbox.Row{}.TableName(), "flam_outbox")
}

// assertCause fails the test unless err wraps the sentinel and holds the
// context.
func assertCause(t *testing.T, err error, sentinel error, ctx map[string]any) {
	t.Helper()
	var e *flam.Error
	if !errors.Is(err, sentinel) || !errors.As(err, &e) {
		t.Fatalf("error = %v, want it to wrap %v", err, sentinel)
	}
	testutil.Context(t, e, ctx)
}

func Test_Bundle(t *testing.T) {
	t.Run("should register the processes", func(t *testing.T) {
		b := outbox.NewBundle()
		testutil.Equal(t, "Id()", b.Id(), "flam.outbox")

		a := testutil.NewApp()
		testutil.NoError(t, a.RegisterBundle(b))
		testutil.Equal(t, "calls", a.Calls(), []string{"bundle:flam.outbox", "provider/1"})
	})

	t.Run("should provide no relay without configuration", func(t *testing.T) {
		e := newEnv(t, options{})
		got, err := processes(e.app)
		testutil.NoError(t, err)
		testutil.Equal(t, "processes", len(got), 0)
	})

	t.Run("should provide the configured relays", func(t *testing.T) {
		e := newEnv(t, options{values: map[string]any{
			outbox.RelayConfigPath + ".b": container.Bag{},
			outbox.RelayConfigPath + ".a": container.Bag{outbox.ConfigFieldProcess: "custom"},
		}})
		got, err := processes(e.app)
		testutil.NoError(t, err)
		ids := map[string]bool{}
		for _, p := range got {
			ids[p.Id()] = true
		}
		testutil.Equal(t, "ids", ids, map[string]bool{"custom": true, "outbox-b": true})
		testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(outbox.Table), true)
	})

	t.Run("should refuse a relay configuration that is not a bag", func(t *testing.T) {
		e := newEnv(t, options{values: map[string]any{outbox.RelayConfigPath + ".main": "text"}})
		_, err := processes(e.app)
		assertCause(t, err, outbox.ErrInvalidField, map[string]any{"id": "main", "field": "flam.outbox.relays.main"})
	})

	t.Run("should refuse the invalid fields", func(t *testing.T) {
		for field, value := range map[string]any{
			outbox.ConfigFieldConnection: "",
			outbox.ConfigFieldBroker:     "",
			outbox.ConfigFieldProcess:    1,
			outbox.ConfigFieldPoll:       "0s",
			outbox.ConfigFieldBatch:      0,
			outbox.ConfigFieldLease:      "x",
			outbox.ConfigFieldBackoff:    "-1s",
			outbox.ConfigFieldMaxBackoff: "1ms",
			outbox.ConfigFieldDelete:     "yes",
			outbox.ConfigFieldRetention:  "-1s",
			outbox.ConfigFieldCleanup:    "0s",
			outbox.ConfigFieldMigrate:    1,
		} {
			e := newEnv(t, options{relay: container.Bag{field: value}})
			_, err := processes(e.app)
			assertCause(t, err, outbox.ErrInvalidField, map[string]any{"id": "main", "field": field})
			if !strings.HasSuffix(err.Error(), "unable to create outbox relay: invalid outbox relay field") {
				t.Errorf("error = %v", err)
			}
		}
	})

	t.Run("should fail with an unknown connection or broker", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldConnection: "missing"}})
		_, err := processes(e.app)
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("error = %v, want ErrUnknownID", err)
		}
		assertCause(t, err, outbox.ErrConnection, map[string]any{"id": "main", "connection": "missing"})

		e = newEnv(t, options{relay: container.Bag{outbox.ConfigFieldBroker: "missing"}})
		_, err = processes(e.app)
		if !errors.Is(err, factory.ErrUnknownID) {
			t.Fatalf("error = %v, want ErrUnknownID", err)
		}
		assertCause(t, err, outbox.ErrBroker, map[string]any{"id": "main", "broker": "missing"})
	})

	t.Run("should fail without the database bundle", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}, bundles: []flam.IBundle{config.NewBundle(), outbox.NewBundle()}})
		_, err := processes(e.app)
		assertCause(t, err, outbox.ErrConnection, map[string]any{"id": "main", "connection": "main"})
		if !strings.Contains(err.Error(), "no database connection factory, register the database bundle") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("should fail without the broker bundle", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}, bundles: []flam.IBundle{
			config.NewBundle(), database.NewBundle(), dbstandard.NewBundle(), sqlite.NewBundle(), outbox.NewBundle(),
		}})
		_, err := processes(e.app)
		assertCause(t, err, outbox.ErrBroker, map[string]any{"id": "main", "broker": "fake"})
		if !strings.Contains(err.Error(), "no broker factory, register the broker bundle") {
			t.Errorf("error = %v", err)
		}
	})

	t.Run("should fail when the table cannot be migrated", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{}})
		testutil.NoError(t, e.conn.Exec("CREATE VIEW flam_outbox AS SELECT 1 AS id").Error)
		_, err := processes(e.app)
		assertCause(t, err, outbox.ErrMigrate, map[string]any{"id": "main", "connection": "main"})
	})

	t.Run("should not migrate the table when told not to", func(t *testing.T) {
		e := newEnv(t, options{relay: container.Bag{outbox.ConfigFieldMigrate: false}})
		_, err := processes(e.app)
		testutil.NoError(t, err)
		testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(outbox.Table), false)
	})
}

func Test_Add(t *testing.T) {
	t.Run("should store the messages with the transaction", func(t *testing.T) {
		e := newEnv(t, options{})
		testutil.NoError(t, outbox.Migrate(e.conn.Gorm()))

		boom := errors.New("boom")
		err := e.conn.Transaction(func(tx *gorm.DB) error {
			testutil.NoError(t, outbox.Add(tx, "orders", []byte("lost"), nil))
			return boom
		})
		testutil.Equal(t, "rolled back", err, boom)
		testutil.Equal(t, "rows", len(e.rows(t)), 0)

		testutil.NoError(t, e.conn.Transaction(func(tx *gorm.DB) error {
			testutil.NoError(t, outbox.Add(tx, "orders", []byte("one"), map[string]string{"h": "v"}))
			testutil.NoError(t, outbox.Add(tx, "orders", nil, map[string]string{}))
			return outbox.AddJSON(tx, "users", map[string]int{"id": 7}, nil)
		}))
		rows := e.rows(t)
		testutil.Equal(t, "rows", len(rows), 3)
		for i, want := range []outbox.Row{
			{Topic: "orders", Payload: []byte("one"), Headers: `{"h":"v"}`},
			{Topic: "orders", Payload: []byte{}},
			{Topic: "users", Payload: []byte(`{"id":7}`)},
		} {
			got := rows[i]
			testutil.Equal(t, "Topic", got.Topic, want.Topic)
			testutil.Equal(t, "Payload", got.Payload, want.Payload)
			testutil.Equal(t, "Headers", got.Headers, want.Headers)
			testutil.Equal(t, "SentAt", got.SentAt, (*time.Time)(nil))
			testutil.Equal(t, "Attempts", got.Attempts, 0)
		}
		testutil.Equal(t, "ids", rows[0].ID < rows[1].ID && rows[1].ID < rows[2].ID, true)
	})

	t.Run("should refuse the invalid arguments", func(t *testing.T) {
		e := newEnv(t, options{})
		testutil.Error(t, outbox.Add(nil, "orders", nil, nil), outbox.ErrNilTx,
			"unable to add outbox message: nil outbox transaction", map[string]any{"topic": "orders"})
		testutil.Error(t, outbox.Add(e.conn.Gorm(), "", nil, nil), outbox.ErrInvalidTopic,
			"unable to add outbox message: invalid outbox topic", map[string]any{"topic": ""})
		testutil.Error(t, outbox.AddJSON(e.conn.Gorm(), "orders", make(chan int), nil), outbox.ErrEncode,
			"unable to add outbox message: unable to encode outbox message: json: unsupported type: chan int",
			map[string]any{"topic": "orders"})
	})

	t.Run("should fail when the message cannot be stored", func(t *testing.T) {
		e := newEnv(t, options{})
		err := outbox.Add(e.conn.Gorm(), "orders", nil, nil)
		assertCause(t, err, outbox.ErrAdd, map[string]any{"topic": "orders"})
	})
}

func Test_Migrate(t *testing.T) {
	t.Run("should create the table", func(t *testing.T) {
		e := newEnv(t, options{})
		testutil.NoError(t, outbox.Migrate(e.conn.Gorm()))
		testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(outbox.Table), true)
	})

	t.Run("should fail when the table cannot be created", func(t *testing.T) {
		e := newEnv(t, options{})
		testutil.NoError(t, e.conn.Exec("CREATE VIEW flam_outbox AS SELECT 1 AS id").Error)
		assertCause(t, outbox.Migrate(e.conn.Gorm()), outbox.ErrMigrate, map[string]any{})
	})
}

func Test_NewMigration(t *testing.T) {
	e := newEnv(t, options{})
	m := outbox.NewMigration("001", "main")
	testutil.Equal(t, "Id()", m.Id(), "001")
	testutil.Equal(t, "Group()", m.Group(), "main")
	testutil.Equal(t, "Dependencies()", m.Dependencies(), []string(nil))
	testutil.Equal(t, "Description()", m.Description(), "create the flam_outbox table")
	testutil.NoError(t, m.Up(e.conn))
	testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(outbox.Table), true)
	testutil.NoError(t, m.Down(e.conn))
	testutil.Equal(t, "HasTable()", e.conn.Migrator().HasTable(outbox.Table), false)
}
