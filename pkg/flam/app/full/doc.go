// Package full is the opt-in to every feature of flam: [New] returns the
// application of [github.com/cjdias/flam-in-go/pkg/flam/app.New], which
// registers the core bundles only, with the bundle of [NewBundle]
// registering the database, migration, redis, cache, translator,
// validator, secrets, client, broker, tracing, metrics, rest, health and
// watchdog features and their drivers. The command feature of
// [github.com/cjdias/flam-in-go/pkg/flam/command] is left out, as it reads
// the arguments of the program: register its bundles to run commands.
//
// It lives apart from the app package so that a program not importing it
// does not link these features nor their drivers, such as gorm, the sql
// drivers, go-redis and miniredis: the Go linker drops the unused
// functions of an imported package, but keeps its init functions and what
// they reach, such as the sql drivers registering themselves. To link
// some of the features only, register their bundles on the application
// of app.New instead.
//
//	a := full.New(nil)
//	defer a.Close()
//	return a.Run()
package full
