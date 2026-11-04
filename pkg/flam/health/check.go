package health

import (
	"context"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// NewCheck returns the [flam.IHealthCheck] of the id and the kind, calling
// fn to check, such as for the checks the application provides in the
// [CheckGroup] group, or the drivers create. It fails with ErrInvalidID
// for an empty id, ErrInvalidKind for a kind other than
// [flam.HealthLiveness], [flam.HealthReadiness] and [flam.HealthBoth], and
// ErrNilCheck for a nil fn.
func NewCheck(
	id string,
	kind flam.HealthKind,
	fn func(ctx context.Context) error,
) (flam.IHealthCheck, error) {
	if err := validate(checkNewMessage, id, kind); err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, newErr(ErrNilCheck, checkNewMessage, id, nil)
	}
	return &check{id: id, kind: kind, fn: fn}, nil
}

// ParseKind returns the kind named by the value, a [flam.HealthKind] or
// one of the names [KindLiveness], [KindReadiness] and [KindBoth], and
// whether the value is one.
func ParseKind(
	value any,
) (flam.HealthKind, bool) {
	var kind flam.HealthKind
	switch v := value.(type) {
	case flam.HealthKind:
		kind = v
	case string:
		kind = map[string]flam.HealthKind{
			KindLiveness:  flam.HealthLiveness,
			KindReadiness: flam.HealthReadiness,
			KindBoth:      flam.HealthBoth,
		}[v]
	}
	return kind, validKind(kind)
}

// checkNewMessage is the message of the errors returned by NewCheck.
const checkNewMessage = "unable to create health check"

// check is the [flam.IHealthCheck] returned by [NewCheck].
type check struct {
	id   string
	kind flam.HealthKind
	fn   func(ctx context.Context) error
}

// compile time check that *check implements IHealthCheck
var _ flam.IHealthCheck = (*check)(nil)

func (c *check) Id() string {
	return c.id
}

func (c *check) Kind() flam.HealthKind {
	return c.kind
}

func (c *check) Check(
	ctx context.Context,
) error {
	return c.fn(ctx)
}

// validate returns the error, with the message, of an empty id or of an
// invalid kind, or nil.
func validate(
	message string,
	id string,
	kind flam.HealthKind,
) error {
	switch {
	case id == "":
		return newErr(ErrInvalidID, message, id, nil)
	case !validKind(kind):
		return newErr(ErrInvalidKind, message, id, nil).
			With(ErrorFieldKind, int(kind))
	}
	return nil
}

// validKind reports whether the kind is one of HealthLiveness,
// HealthReadiness and HealthBoth.
func validKind(
	kind flam.HealthKind,
) bool {
	return kind >= flam.HealthLiveness && kind <= flam.HealthBoth
}
