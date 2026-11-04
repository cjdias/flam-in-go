package backoff

import (
	"math"
	"testing"
	"time"
)

// The test lives next to the package, unlike the other tests of flam,
// since an internal package cannot be imported from the tests directory.

func Test_Delay(t *testing.T) {
	for name, tc := range map[string]struct {
		base    time.Duration
		maximum time.Duration
		attempt int
		want    time.Duration
	}{
		"first attempt":              {time.Second, time.Minute, 1, time.Second},
		"attempt before the first":   {time.Second, time.Minute, 0, time.Second},
		"second attempt":             {time.Second, time.Minute, 2, 2 * time.Second},
		"third attempt":              {time.Second, time.Minute, 3, 4 * time.Second},
		"capped":                     {time.Second, 3 * time.Second, 3, 3 * time.Second},
		"doubled to exactly the max": {time.Second, 2 * time.Second, 2, 2 * time.Second},
		"base at the max":            {time.Second, time.Second, 5, time.Second},
		"base over the max":          {2 * time.Second, time.Second, 1, time.Second},
		"many attempts":              {time.Nanosecond, time.Duration(math.MaxInt64), math.MaxInt, time.Duration(math.MaxInt64)},
		"max over the half of int64": {time.Duration(math.MaxInt64/2 + 1), time.Duration(math.MaxInt64), 2, time.Duration(math.MaxInt64)},
	} {
		if got := Delay(tc.base, tc.maximum, tc.attempt); got != tc.want {
			t.Errorf("%s: Delay() = %v, want %v", name, got, tc.want)
		}
	}
}
