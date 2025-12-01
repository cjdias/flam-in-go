package flam

type WatchdogLoggerCreator interface {
	Accept(config Bag) bool
	Create(config Bag) (WatchdogLogger, error)
}
