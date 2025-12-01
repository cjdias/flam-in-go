package flam

type WatchdogLogger interface {
	LogStart(id string)
	LogError(id string, e error)
	LogDone(id string)
}
