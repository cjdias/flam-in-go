package flam

type CacheSerializer interface {
	Serialize(obj any) (string, error)
	Deserialize(data string, obj any) error
}
