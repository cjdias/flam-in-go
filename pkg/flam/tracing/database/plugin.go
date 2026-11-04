package database

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"
	"go.opentelemetry.io/otel/trace"
	"gorm.io/gorm"
)

// NewPlugin returns the gorm plugin tracing the statements of the
// connections it is added to, with gorm.DB Use, with the tracer of
// [ScopeName] of the provider, or of the global provider of OpenTelemetry
// when nil, read when the plugin is created.
//
// Each statement runs within a client span, a child of the span of the
// context of the statement, given with gorm.DB WithContext, named by its
// operation and its table, such as "gorm.query users", and holding the
// attributes of the database semantic conventions of OpenTelemetry: the
// name of the dialector, such as "sqlite", the operation, the table, the
// SQL text, with the placeholders of its values, which are left out as
// they may hold private data, and the number of rows affected, at
// [AttributeRowsAffected]. A statement
// failing, other than with gorm.ErrRecordNotFound, records its error and
// sets the status of the span to an error.
func NewPlugin(
	provider trace.TracerProvider,
) gorm.Plugin {
	if provider == nil {
		provider = otel.GetTracerProvider()
	}
	return &plugin{tracer: provider.Tracer(ScopeName)}
}

// plugin is the gorm plugin returned by NewPlugin.
type plugin struct {
	tracer trace.Tracer
}

// compile time check that *plugin implements Plugin
var _ gorm.Plugin = (*plugin)(nil)

func (p *plugin) Name() string {
	return PluginName
}

// Initialize registers the callbacks starting a span before the gorm
// callback of each processor, and ending it after.
func (p *plugin) Initialize(
	db *gorm.DB,
) error {
	callbacks := db.Callback()
	return errors.Join(
		callbacks.Create().Before("gorm:create").Register(p.name("before", OperationCreate), p.before(OperationCreate)),
		callbacks.Create().After("gorm:create").Register(p.name("after", OperationCreate), p.after),
		callbacks.Query().Before("gorm:query").Register(p.name("before", OperationQuery), p.before(OperationQuery)),
		callbacks.Query().After("gorm:query").Register(p.name("after", OperationQuery), p.after),
		callbacks.Update().Before("gorm:update").Register(p.name("before", OperationUpdate), p.before(OperationUpdate)),
		callbacks.Update().After("gorm:update").Register(p.name("after", OperationUpdate), p.after),
		callbacks.Delete().Before("gorm:delete").Register(p.name("before", OperationDelete), p.before(OperationDelete)),
		callbacks.Delete().After("gorm:delete").Register(p.name("after", OperationDelete), p.after),
		callbacks.Row().Before("gorm:row").Register(p.name("before", OperationRow), p.before(OperationRow)),
		callbacks.Row().After("gorm:row").Register(p.name("after", OperationRow), p.after),
		callbacks.Raw().Before("gorm:raw").Register(p.name("before", OperationRaw), p.before(OperationRaw)),
		callbacks.Raw().After("gorm:raw").Register(p.name("after", OperationRaw), p.after),
	)
}

// name returns the name of the callback of the operation run at the stage,
// before or after the gorm callback of the operation, such as
// "flam.tracing:before_query".
func (p *plugin) name(
	stage string,
	operation string,
) string {
	return PluginName + ":" + stage + "_" + operation
}

// before returns the callback starting the span of a statement of the
// operation, keeping it in the context of the statement, along with the
// context it replaces.
func (p *plugin) before(
	operation string,
) func(*gorm.DB) {
	return func(db *gorm.DB) {
		stmt := db.Statement
		name := SpanNamePrefix + operation
		if stmt.Table != "" {
			name += " " + stmt.Table
		}
		ctx, span := p.tracer.Start(stmt.Context, name,
			trace.WithSpanKind(trace.SpanKindClient),
			trace.WithAttributes(
				semconv.DBSystemNameKey.String(db.Name()),
				semconv.DBOperationName(operation),
			),
		)
		stmt.Context = context.WithValue(ctx, pluginKey{}, &pluginState{span: span, parent: stmt.Context})
	}
}

// after ends the span the before callback started for the statement, if
// any, recording its table, its SQL text, the rows it affected and its
// error, and gives the statement its context back.
func (p *plugin) after(
	db *gorm.DB,
) {
	stmt := db.Statement
	state, ok := stmt.Context.Value(pluginKey{}).(*pluginState)
	if !ok {
		return
	}
	stmt.Context = state.parent
	span := state.span
	defer span.End()

	if stmt.Table != "" {
		span.SetAttributes(semconv.DBCollectionName(stmt.Table))
	}
	if query := stmt.SQL.String(); query != "" {
		span.SetAttributes(semconv.DBQueryText(query))
	}
	span.SetAttributes(attribute.Int64(AttributeRowsAffected, db.RowsAffected))
	if db.Error != nil && !errors.Is(db.Error, gorm.ErrRecordNotFound) {
		span.RecordError(db.Error)
		span.SetStatus(codes.Error, db.Error.Error())
	}
}

// pluginKey is the key of the state of the span of a statement in its
// context.
type pluginKey struct{}

// pluginState is the span of a statement, and the context of the statement
// its context replaced.
type pluginState struct {
	span   trace.Span
	parent context.Context
}
