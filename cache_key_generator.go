package flam

type CacheKeyGenerator interface {
	Generate(obj any) (string, error)
}
