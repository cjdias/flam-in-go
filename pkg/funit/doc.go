// Package funit is a functional testing harness for flam applications: a
// [Runner] takes an application, made of the bundles of the program under
// test, through the stages of a functional test.
//
//   - Arrange: the configuration files and values of the test are added to
//     the configuration of the application, as sources of their own, the
//     watchdog processes the test asks for are activated, the application
//     is booted, then the arrange functions run, such as one beginning the
//     database transaction the test works in, or one subscribing to the
//     pub/sub topics whose messages the test checks.
//   - Run: the application runs on a goroutine of its own, until its
//     processes run, so that the test can call them, such as a REST server
//     answering the requests of the act.
//   - Act: the act function runs.
//   - Assert: the assertions run, failing the test on a mismatch.
//   - Teardown: whatever the stages before it did, the processes are closed,
//     so that the run of the application ends, the teardown functions run,
//     the database transaction is rolled back, the subscriptions are
//     cancelled and the configuration sources are removed.
//
// The stage functions are invoked through the container of the
// application, so that they take its values as arguments:
//
//	var users []User
//	runner, _ := funit.NewRunner(app, t)
//	_ = runner.
//		WithConfig("testdata/config.yaml", 100).
//		WithArrangeDatabase("main").
//		WithArrange("seed", func() error {
//			return runner.Tx().Create(&User{Name: "ann"}).Error
//		}).
//		WithAct(func() error {
//			return runner.Tx().Find(&users).Error
//		}).
//		WithAssert("one user", func() {
//			if len(users) != 1 {
//				t.Errorf("users = %v, want one", users)
//			}
//		}).
//		Run()
//
// A runner runs once, as the run ends the processes of the application.
// The application itself is left to the test, which closes it, typically
// with t.Cleanup.
//
// The application is not safe for concurrent use: while it runs on its
// goroutine, its run waits in the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog] running the processes,
// leaving the container to the stages of the runner.
package funit
