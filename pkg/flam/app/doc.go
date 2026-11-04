// Package app holds the implementation of the
// [github.com/cjdias/flam-in-go/pkg/flam.IApp] contract: [New] returns an
// application wiring the bundles of a program into a dependency injection
// container.
//
// The container provides, from the start, the application, and New
// registers the core bundles of flam, with their drivers: the clock,
// pubsub, trigger, disk, configuration and log ones, as listed by New.
// Register only the other bundles, and decorate, rather than provide
// again, a value a core bundle provides, such as the clock in a test. The
// other features of flam, database, migration, redis, cache, translator,
// validator, secrets, client, broker, tracing, metrics, rest, health,
// command and watchdog, are opt-in: register their bundles, or use the
// application of [github.com/cjdias/flam-in-go/pkg/flam/app/full.New],
// which registers them all but command. They live in packages of their own, so that a
// program links only the features it registers.
//
// The configuration is provided by the bundle of
// [github.com/cjdias/flam-in-go/pkg/flam/config], decorated by the
// application to add its defaults in this order, the first added winning:
// the values given to New, then the defaults of the bundles implementing
// [github.com/cjdias/flam-in-go/pkg/flam.IConfigurableBundle], in
// registration order, the core bundles first. The sources, added when the
// configuration bundle boots, and the values stored with Set override them
// all. As the application decorates the configuration, the container
// refuses another decorator of it, except in a child scope.
//
//	a := app.New(container.Bag{"flam": container.Bag{"config": ...}})
//	defer a.Close()
//	if err := a.RegisterBundle(mybundle.NewBundle()); err != nil {
//		return err
//	}
//	return a.Run()
//
// The bundles may call the application back from their Config, Register,
// Boot, Run and Close methods: a bundle registering other bundles from its
// Register method is the usual case, and a bundle closing its values takes
// them from the container with Invoke, which keeps working while the
// application closes its bundles. Calling Boot or Run from a Boot method
// fails with ErrBooting, as the boot is under way, and Run from a Run
// method with ErrRunning.
package app
