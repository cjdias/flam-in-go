// Package database is the database feature: the bundle returned from
// [NewBundle] provides the
// [github.com/cjdias/flam-in-go/pkg/flam.IDatabaseConfigFactory], the
// [github.com/cjdias/flam-in-go/pkg/flam.IDatabaseDialectFactory] and the
// [github.com/cjdias/flam-in-go/pkg/flam.IDatabaseConnectionFactory] of an
// application, which create the gorm configurations, dialects and
// connections described by the configuration at [ConfigConfigPath],
// [DialectConfigPath] and [ConnectionConfigPath] with the creators
// provided in the [ConfigCreatorGroup], [DialectCreatorGroup] and
// [ConnectionCreatorGroup] container groups.
//
// A connection configuration names a dialect and a gorm configuration by
// their ids, such as:
//
//	flam:
//	  database:
//	    dialects:
//	      main: {driver: flam.database.dialects.driver.sqlite, host: app.db}
//	    connections:
//	      main: {dialect: main}
//
// the gorm configuration taking its default, the standard one of
// [github.com/cjdias/flam-in-go/pkg/flam/database/configs/standard].
//
// A connection may also name the dialects of its read replicas and extra
// write sources, at [ConfigFieldReplicas] and [ConfigFieldSources], chosen
// among by the [ConfigFieldPolicy] policy with the
// [gorm.io/plugin/dbresolver] plugin:
//
//	connections:
//	  main: {dialect: primary, replicas: [replica-1, replica-2], policy: round_robin}
//
// Its queries then run on a replica, which may lag behind the writes: a
// query reading what was just written runs on the sources with the
// dbresolver Write clause, as in conn.Clauses(dbresolver.Write).First(&u),
// as do the statements of a transaction, run on the dialect of the
// connection.
//
// The opening of a connection is retried, when it fails, as many times as
// its [ConfigFieldConnectRetries] field says, none by default, waiting
// with the [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container
// for a delay doubling from its [ConfigFieldConnectRetryDelay]. Once open,
// no statement is retried on its own: [Retry] retries the units of work of
// the application, such as transactions, failing with the transient
// errors [IsTransient] tells, such as serialization failures and
// deadlocks.
//
// [Request] describes a query to narrow a connection with, and
// [NewConnection] and [NewDialect] wrap a *gorm.DB and a gorm dialector
// opened elsewhere.
//
// The drivers live in the sub packages of
// [github.com/cjdias/flam-in-go/pkg/flam/database/configs] and
// [github.com/cjdias/flam-in-go/pkg/flam/database/dialects]. They import
// this package for its paths and groups, so this package never imports
// them.
package database
