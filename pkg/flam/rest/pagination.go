package rest

import (
	"net/http"

	"github.com/gorilla/schema"

	"github.com/cjdias/flam-in-go/pkg/flam/envelope"
)

// Bounds of the count of the items of a [Page] decoded by [DecodePage].
const (
	// DefaultPageCount is the count of a page asking for none, or for a
	// count below 1.
	DefaultPageCount = 20

	// MaxPageCount is the most items a page may ask for, a page asking for
	// more being given this count.
	MaxPageCount = 100
)

// Page is the page of a collection asked for by the query of a request,
// such as "?search=jo&start=40&count=20", as decoded by [DecodePage].
type Page struct {
	// Search is the term the items are searched by, empty when not
	// searched. Before filtering by it with a SQL LIKE, escape it with
	// [github.com/cjdias/flam-in-go/pkg/flam/database.SearchTerm].
	Search string `schema:"search"`

	// Start is the offset of the first item of the page.
	Start int `schema:"start"`

	// Count is the most items of the page.
	Count int `schema:"count"`
}

// DecodePage decodes the page of the query of the URL of the request with
// the decoder, as [DecodeQuery] does, failing with ErrDecode, and bounds it
// with [DefaultPageCount] and [MaxPageCount], as [Page.Bound] does.
//
//	page, err := rest.DecodePage(r.decoder, req)
//	if err != nil {
//		_ = rest.WriteEnvelope(w, req, envelope.NewEnvelope().
//			WithNewError(http.StatusBadRequest, err))
//		return
//	}
//	request := &database.Request{}
//	if page.Search != "" {
//		request.AddTextFilter("name", database.SearchTerm(page.Search))
//	}
//	request.Paginate(page.Start, page.Count)
//	... // count the matching rows, then find those of the page
//	_ = rest.WriteEnvelope(w, req, page.Envelope(req, users, total))
func DecodePage(
	decoder *schema.Decoder,
	req *http.Request,
) (Page, error) {
	var page Page
	if err := DecodeQuery(decoder, req, &page); err != nil {
		return Page{}, err
	}
	return page.Bound(DefaultPageCount, MaxPageCount), nil
}

// Bound returns a copy of the page starting at 0 at least, and counting
// defaultCount items when counting none, or less, and maxCount items at
// most, when maxCount is positive.
func (p Page) Bound(
	defaultCount int,
	maxCount int,
) Page {
	p.Start = max(p.Start, 0)
	if p.Count < 1 {
		p.Count = defaultCount
	}
	if maxCount > 0 {
		p.Count = min(p.Count, maxCount)
	}
	return p
}

// Envelope returns the envelope of the data of the page, along with its
// [envelope.Pagination], for the total number of items of every page,
// whose links to the previous and next pages lead to the path of the URL
// of the request, when not nil.
func (p Page) Envelope(
	req *http.Request,
	data any,
	total int64,
) envelope.Envelope {
	var path []string
	if req != nil && req.URL != nil {
		path = append(path, req.URL.EscapedPath())
	}
	return envelope.NewEnvelope(data).
		WithPagination(p.Search, int64(p.Start), int64(p.Count), total, path...)
}
