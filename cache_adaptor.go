package flam

import "time"

type CacheAdaptor interface {
	Has(obj ...any) ([]bool, error)
	Get(obj ...any) ([]any, error)
	Set(obj ...any) error
	SetEphemeral(ttl time.Duration, obj ...any) error
	Del(obj ...any) error
}
