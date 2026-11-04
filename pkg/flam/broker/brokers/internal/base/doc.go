// Package base holds what the broker drivers share: the registry of the
// subscriptions of a broker, running each in a goroutine of its own and
// closing them with the broker, the call of the handlers, recovering
// their panics, the reading of the configuration fields, with the
// defaults taken from the configuration, and the errors of the drivers.
package base
