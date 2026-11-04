package envelope

import (
	"net/url"
	"strconv"
)

// Pagination describes pagination metadata attached to a response envelope.
type Pagination struct {
	// Search is the search term the page was filtered by, if any.
	Search string `json:"search,omitempty"`
	// Start is the offset of the page's first item.
	Start int64 `json:"start"`
	// Count is the page size asked for.
	Count int64 `json:"count"`
	// Total is the number of items across every page.
	Total int64 `json:"total"`
	// Prev is the link to the previous page. It is empty on the first page,
	// and when Count is not positive.
	Prev string `json:"prev,omitempty"`
	// Next is the link to the next page. It is empty on the last page, and
	// when Count is not positive.
	Next string `json:"next,omitempty"`
}

// newPagination builds the pagination metadata for a page of results.
//
// The path is optional: given one, without a query, the links it produces are
// resolvable on their own, and without one they stay relative to whatever the
// caller requested. A page of no item has no links, as they would lead back to
// the same page.
func newPagination(
	search string,
	start,
	count,
	total int64,
	path ...string,
) *Pagination {
	report := &Pagination{
		Search: search,
		Start:  start,
		Count:  count,
		Total:  total,
	}
	if count <= 0 {
		return report
	}

	target := ""
	if len(path) > 0 {
		target = path[0]
	}

	if start > 0 {
		report.Prev = paginationLink(target, search, max(start-count, 0), count)
	}

	if start+count < total {
		report.Next = paginationLink(target, search, start+count, count)
	}

	return report
}

// paginationLink renders a link to another page of the same collection.
//
// The parameters are encoded rather than interpolated, so a search term holding
// a character with meaning in a query string cannot corrupt the link. An empty
// term is left out entirely instead of being emitted as a bare "search=".
func paginationLink(
	path,
	search string,
	start,
	count int64,
) string {
	query := url.Values{}
	if search != "" {
		query.Set("search", search)
	}
	query.Set("start", strconv.FormatInt(start, 10))
	query.Set("count", strconv.FormatInt(count, 10))

	return path + "?" + query.Encode()
}
