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
