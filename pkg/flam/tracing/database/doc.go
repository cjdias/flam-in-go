// Package database traces the statements of the gorm database connections
// of [github.com/cjdias/flam-in-go/pkg/flam/database] with the
// [go.opentelemetry.io/otel/trace.TracerProvider] of the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing].
//
// The gorm plugin returned by [NewPlugin] runs each statement of the
// connections it is added to within a client span, a child of the span of
// the context of the statement, named by its operation and its table, such
// as "gorm.query users". The bundle returned from [NewBundle] adds it to
// every connection created by the connection factory; it can also be
// added to a connection, or to any gorm.DB, with Use:
//
//	err := conn.Use(database.NewPlugin(provider))
//
// The spans have a parent only when the statement is given the context of
// the caller, such as the context of a request served within the span of
// the REST middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/tracing/middleware]:
//
//	err := conn.WithContext(req.Context()).First(&user, id).Error
//
// The SQL text of a statement is recorded with the placeholders of its
// values, the values themselves being left out, as they may hold private
// data.
package database
