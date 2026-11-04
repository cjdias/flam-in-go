package database

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// Request aggregates filters, sorters and relations for a data query. A
// *Request is a [flam.IRequest].
//
// Sorters is a slice rather than a map because their order is what they mean:
// "by kind, then by name" is a different query from "by name, then by kind", and
// a map cannot say which was asked for. Filters and relations carry no such
// precedence, so they stay keyed by field.
type Request struct {
	// Filters are the filters rows must all meet, keyed by their field, so adding
	// a second filter on a field replaces the first.
	Filters map[string]RequestFilter
	// Sorters are the sorts applied, in the order they were added.
	Sorters []RequestSort
	// Relations are the associations to preload, keyed by relation name.
	Relations map[string]RequestRelation
	// Offset is the number of rows to skip. Nil skips none, and a negative value
	// is treated as 0.
	Offset *int
	// Limit is the most rows answered. Nil answers every row, and a value below 1
	// is treated as 1.
	Limit *int
	// SoftDeleted includes soft-deleted rows when true. They are left out by
	// default.
	SoftDeleted bool
}

var _ flam.IRequest = (*Request)(nil)

// RequestFilter defines a single field filter applied to a request.
type RequestFilter struct {
	// Field is the column, or SQL expression such as "lower(name)", to filter on.
	// It is written into the query as given, so it must never come from user input.
	Field string
	// Operator is the SQL comparison, such as "=", "in" or "like". It is written
	// into the query as given; like operators escape with '\', through an
	// escape '\' clause but on MySQL, whose default it is.
	Operator string
	// Value is the operand. It is the only part bound as a query parameter.
	Value any
}

// RequestSort defines a single field sort applied to a request.
type RequestSort struct {
	// Field is the column to sort by. It is written into the query as given, so it
	// must never come from user input.
	Field string
	// Dir is the direction rows are sorted in.
	Dir flam.RequestSortDirection
}

// RequestRelation defines a related resource to include in a request.
type RequestRelation struct {
	// Relation is the name of the association to preload.
	Relation string
	// Condition is an optional SQL condition the preloaded rows must meet, such as
	// "state = 'active'". Empty preloads every related row.
	Condition string
}

// AddFilter adds a field filter to the request and returns the request.
func (r *Request) AddFilter(
	field string,
	operator string,
	value any,
) flam.IRequest {
	if r.Filters == nil {
		r.Filters = make(map[string]RequestFilter)
	}

	r.Filters[field] = RequestFilter{
		Field:    field,
		Operator: operator,
		Value:    value,
	}

	return r
}

// AddTextFilter adds a text filter to the request and returns the request.
func (r *Request) AddTextFilter(
	field string,
	value string,
	op ...string,
) flam.IRequest {
	operator := requestOption(op, "like")

	if !r.folds(operator) {
		return r.AddFilter(field, operator, value)
	}

	return r.AddFilter("lower("+field+")", operator, strings.ToLower(value))
}

// AddSort adds a field sorter to the request and returns the request.
//
// Sorters are applied in the order they are added, so the first one added is the
// one rows are ordered by first. A field added twice keeps the place it was first
// given and takes the direction it was last given, which is what it did when these
// were keyed by field.
func (r *Request) AddSort(
	field string,
	direction ...flam.RequestSortDirection,
) flam.IRequest {
	dir := requestOption(direction, flam.RequestSortAsc)
	if dir != flam.RequestSortDesc {
		dir = flam.RequestSortAsc
	}

	for index, sorter := range r.Sorters {
		if sorter.Field == field {
			r.Sorters[index].Dir = dir

			return r
		}
	}

	r.Sorters = append(r.Sorters, RequestSort{
		Field: field,
		Dir:   dir,
	})

	return r
}

// AddRelation adds a related resource to the request and returns the request.
func (r *Request) AddRelation(
	relation string,
	condition ...string,
) flam.IRequest {
	if r.Relations == nil {
		r.Relations = make(map[string]RequestRelation)
	}

	r.Relations[relation] = RequestRelation{
		Relation:  relation,
		Condition: requestOption(condition, ""),
	}

	return r
}

// Scope returns tx narrowed by the whole request: its filters, then its
// sorters, in the order they were added, its relations, its offset, its
// limit and its soft-deleted rows. The filters and the relations are applied
// in the order of their keys, so that the same request always gives the
// same query.
func (r *Request) Scope(
	tx *gorm.DB,
) *gorm.DB {
	tx = r.FilterScope(tx)

	for _, sorter := range r.Sorters {
		tx = tx.Order(sorter.Field + " " + string(sorter.Dir))
	}

	for _, key := range slices.Sorted(maps.Keys(r.Relations)) {
		relation := r.Relations[key]
		if relation.Condition == "" {
			tx = tx.Preload(relation.Relation)
		} else {
			tx = tx.Preload(relation.Relation, relation.Condition)
		}
	}

	if r.Offset != nil {
		tx = tx.Offset(max(*r.Offset, 0))
	}

	if r.Limit != nil {
		tx = tx.Limit(max(*r.Limit, 1))
	}

	return tx
}

// FilterScope returns tx narrowed by the filters of the request, in the
// order of their keys, and its soft-deleted rows, only, as for counting the
// rows of a page.
func (r *Request) FilterScope(
	tx *gorm.DB,
) *gorm.DB {
	for _, key := range slices.Sorted(maps.Keys(r.Filters)) {
		filter := r.Filters[key]
		tx = tx.Where(r.predicate(tx, filter), filter.Value)
	}

	if r.SoftDeleted {
		tx = tx.Unscoped()
	}

	return tx
}

// predicate returns the SQL condition of the filter, its value bound as a
// parameter, a like operator escaping with '\'. MySQL escapes with '\'
// by default, and reads the '\' literal as an unterminated string, so the
// escape clause is left out for it.
func (r *Request) predicate(
	tx *gorm.DB,
	filter RequestFilter,
) string {
	predicate := fmt.Sprintf("%s %s ?", filter.Field, filter.Operator)
	if r.folds(filter.Operator) && (tx.Dialector == nil || tx.Dialector.Name() != "mysql") {
		predicate += ` escape '\'`
	}

	return predicate
}

// folds reports whether the operator is a like operator, whose text filters
// ignore case.
func (r *Request) folds(
	operator string,
) bool {
	switch strings.ToLower(strings.TrimSpace(operator)) {
	case "like", "not like", "ilike", "not ilike":
		return true
	default:
		return false
	}
}

// requestOption returns the first of the optional values, or def when none
// is given, without appending to the caller's slice.
func requestOption[T any](
	values []T,
	def T,
) T {
	if len(values) == 0 {
		return def
	}

	return values[0]
}
