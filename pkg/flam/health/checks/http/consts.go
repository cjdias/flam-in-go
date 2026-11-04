package http

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam/health"
)

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.health.checks.http"

// Driver is the driver name of the http checks, held by the
// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver] field
// of their configuration.
const Driver = "flam.health.checks.driver.http"

// Fields of an http check configuration, along with the fields shared by
// the drivers, documented in
// [github.com/cjdias/flam-in-go/pkg/flam/health].
const (
	// ConfigFieldURL is the field holding the URL the check requests with
	// GET: an absolute http or https URL, or, with a client, a URL
	// resolved against the base URL of the client, such as a path, empty
	// requesting the base URL itself.
	ConfigFieldURL = "url"

	// ConfigFieldClient is the field holding the id of the client the
	// check requests with, in the client factory, the check requesting
	// with a plain HTTP client unless set.
	ConfigFieldClient = "client"

	// ConfigFieldMinStatus is the field holding the lowest status of a
	// healthy response, an integer from 100 to 599, 200 unless set.
	ConfigFieldMinStatus = "min_status"

	// ConfigFieldMaxStatus is the field holding the highest status of a
	// healthy response, an integer from the lowest one to 599, 299 unless
	// set.
	ConfigFieldMaxStatus = "max_status"

	// ConfigFieldTimeout is the field holding the time the check waits for
	// the response, a duration such as "1s", 0, unless set, waiting as
	// long as the probe does.
	ConfigFieldTimeout = "timeout"
)

// Context fields of the [*flam.Error] values returned by the creator and
// the checks.
const (
	// ErrorFieldID holds the id of the check.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldClient holds the id of the client of the check.
	ErrorFieldClient = "client"

	// ErrorFieldURL holds the URL of the check.
	ErrorFieldURL = "url"

	// ErrorFieldStatus holds the status of an unhealthy response, an int.
	ErrorFieldStatus = "status"
)

// Defaults of the fields of a check configuration.
const (
	defaultKind      = health.KindReadiness
	defaultClient    = ""
	defaultMinStatus = 200
	defaultMaxStatus = 299
	defaultTimeout   = time.Duration(0)
)

// Bounds of the statuses, and the most of a body read before closing it,
// so that the connection may be reused.
const (
	minStatus    = 100
	maxStatus    = 599
	maxBodyDrain = 64 << 10
)
