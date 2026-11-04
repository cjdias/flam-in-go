package clock

import (
	"time"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// tickerImpl is the [flam.ITicker] of the system clock, wrapping a
// time.Ticker.
type tickerImpl struct {
	ticker *time.Ticker
}

// compile time check that *tickerImpl implements ITicker
var _ flam.ITicker = (*tickerImpl)(nil)

func (t *tickerImpl) C() <-chan time.Time {
	return t.ticker.C
}

func (t *tickerImpl) Stop() {
	t.ticker.Stop()
}

func (t *tickerImpl) Reset(
	d time.Duration,
) {
	t.ticker.Reset(d)
}
