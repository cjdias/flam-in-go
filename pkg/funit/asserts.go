package funit

import (
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// WithAssertNoError adds the assertion that the error the act stored in
// e is nil.
func (r *Runner) WithAssertNoError(
	e *error,
) *Runner {
	return r.WithAssert("no error", func() {
		r.t.Helper()
		if *e != nil {
			r.t.Errorf("funit: error = %v, want nil", *e)
		}
	})
}

// WithAssertError adds the assertion of the id that the error the act
// stored in e wraps the expected one, as errors.Is tells, and holds the
// text in its message, when given.
func (r *Runner) WithAssertError(
	id string,
	e *error,
	expected error,
	text ...string,
) *Runner {
	return r.WithAssert(id, func() {
		r.t.Helper()
		switch {
		case *e == nil:
			r.t.Errorf("funit: %s: error = nil, want %v", id, expected)
		case !errors.Is(*e, expected):
			r.t.Errorf("funit: %s: error = %v, want it to wrap %v", id, *e, expected)
		case len(text) > 0 && !strings.Contains((*e).Error(), text[0]):
			r.t.Errorf("funit: %s: error = %v, want it to hold %q", id, *e, text[0])
		}
	})
}

// WithAssertDatabaseError adds the assertion that the error the act stored
// in e is the one of a closed database connection: sql.ErrConnDone, or
// the "database is closed" error of database/sql. Use WithAssertError for
// the other database errors.
func (r *Runner) WithAssertDatabaseError(
	e *error,
) *Runner {
	return r.WithAssert("closed database error", func() {
		r.t.Helper()
		switch {
		case *e == nil:
			r.t.Errorf("funit: error = nil, want a closed database error")
		case !errors.Is(*e, sql.ErrConnDone) && !strings.Contains((*e).Error(), "sql: database is closed"):
			r.t.Errorf("funit: error = %v, want a closed database error", *e)
		}
	})
}

// WithAssertTotal adds the assertion that the number the act stored in
// total is the expected one.
func (r *Runner) WithAssertTotal(
	expected int64,
	total *int64,
) *Runner {
	return r.WithAssert(fmt.Sprintf("total of %d", expected), func() {
		r.t.Helper()
		if *total != expected {
			r.t.Errorf("funit: total = %d, want %d", *total, expected)
		}
	})
}

// WithAssertBool adds the assertion that the bool the act stored in res is
// the expected one.
func (r *Runner) WithAssertBool(
	res *bool,
	expected bool,
) *Runner {
	return r.WithAssert(fmt.Sprintf("response %v", expected), func() {
		r.t.Helper()
		if *res != expected {
			r.t.Errorf("funit: response = %v, want %v", *res, expected)
		}
	})
}

// WithAssertPublished adds the assertion that the messages recorded on the
// topics of WithArrangeSignals were published on the topics, as many times
// as given, waiting up to the published timeout for them to be delivered.
// The order of the messages across topics is not checked, as the pub/sub
// delivers those of each subscription on a goroutine of its own.
func (r *Runner) WithAssertPublished(
	topics ...string,
) *Runner {
	return r.WithAssert("published", func() {
		r.t.Helper()
		got := r.awaitPublished(len(topics))
		want := slices.Sorted(slices.Values(topics))
		if !slices.Equal(got, want) {
			r.t.Errorf("funit: published on %q, want %q", got, want)
		}
	})
}

// awaitPublished returns the topics of the recorded messages, sorted,
// once n of them were recorded, or once the published timeout elapsed,
// waiting then a little more for unexpected ones to show.
func (r *Runner) awaitPublished(
	n int,
) []string {
	deadline := time.Now().Add(r.publishedTimeout)
	for r.recorded() < n && time.Now().Before(deadline) {
		time.Sleep(pollInterval)
	}
	time.Sleep(10 * pollInterval) // an unexpected message after the expected ones
	var topics []string
	for _, p := range r.Published() {
		topics = append(topics, p.Topic)
	}
	slices.Sort(topics)
	return topics
}
