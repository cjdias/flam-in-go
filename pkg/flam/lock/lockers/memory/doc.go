// Package memory is the in-process locker driver, for the tests and the
// applications running a single instance: each locker holds its locks in
// a map of the process, lost when it exits, and not shared with the other
// lockers, each holding its own, so that its locks only exclude the
// goroutines sharing the locker.
//
// As the redis lockers do, a locker holds each lock under a random token,
// unique to the acquisition, for its ttl, measured on the
// [github.com/cjdias/flam-in-go/pkg/flam.IClock] of the container, which
// its waits and automatic refreshes also use, so that the tests control
// them with a fake clock. The fence of a key increases with each of its
// acquisitions, for the life of the locker.
package memory
