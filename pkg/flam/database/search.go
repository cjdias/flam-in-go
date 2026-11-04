package database

import (
	"strings"
)

// SearchTerm returns the pattern of the SQL LIKE filters matching the
// values holding the term, such as "%50\%%" for "50%": the term is
// wrapped in '%' wildcards, its '\', '%' and '_' characters escaped with
// '\', the escape character of the like filters of a [Request], so that
// they match themselves only. An empty term gives "%%", matching every
// value: leave the filter out for it instead.
//
//	request.AddTextFilter("name", database.SearchTerm(search))
func SearchTerm(
	term string,
) string {
	return "%" + searchTermEscaper.Replace(term) + "%"
}

// Paginate sets the offset and the limit of the request to the start and
// the count of a page, such as the one of
// [github.com/cjdias/flam-in-go/pkg/flam/rest.Page], and returns the
// request. As for Offset and Limit, a negative start skips no row, and a
// count below 1 answers one row.
func (r *Request) Paginate(
	start int,
	count int,
) *Request {
	r.Offset = &start
	r.Limit = &count

	return r
}

// searchTermEscaper escapes the characters of a like pattern in a single
// pass, so that the escapes it adds are never escaped again.
var searchTermEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
