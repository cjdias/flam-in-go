// Package database provides, to the Prometheus registry of
// [github.com/cjdias/flam-in-go/pkg/flam/metrics], the collectors of the
// statistics of the pools of the database connections of the
// [github.com/cjdias/flam-in-go/pkg/flam.IDatabaseConnectionFactory] of
// the container, the go_sql_* metrics of the Prometheus client, such as
// go_sql_open_connections, labelled by the id of their connection at
// db_name.
//
// The connections are those listed at [ConnectionsConfigPath], or else
// every connection configured in the factory, taken from it, and so
// opened, when the registry is created:
//
//	flam:
//	  metrics:
//	    collectors:
//	      database:
//	        connections: [main]
package database
