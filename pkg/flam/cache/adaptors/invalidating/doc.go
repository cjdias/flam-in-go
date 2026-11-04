// Package invalidating is the cache adaptor driver keeping the adaptors
// of several processes consistent: its adaptors wrap another adaptor of
// the [github.com/cjdias/flam-in-go/pkg/flam.ICacheAdaptorFactory],
// typically a memory one, named by its id in their adaptor field, or its
// default, and a broker of the
// [github.com/cjdias/flam-in-go/pkg/flam.IBrokerFactory], named by its id
// in their broker field, or its default, such as:
//
//	flam:
//	  broker:
//	    brokers:
//	      events: {driver: flam.broker.brokers.driver.redis, connection: main}
//	  cache:
//	    adaptors:
//	      local: {driver: flam.cache.adaptors.driver.memory}
//	      users: {driver: flam.cache.adaptors.driver.invalidating, adaptor: local, broker: events}
//
// An adaptor reads and writes the values in the adaptor it wraps, then
// publishes, on the topic of its topic field, the keys it removes, with
// Del and DelKeys, and the keys it stores, with Set and SetEphemeral,
// unless its publish_set field is false. It subscribes to the topic when
// created, and removes from the adaptor it wraps the keys published by the
// adaptors of the other processes, ignoring its own messages, told apart
// by the random id of the adaptor in their [HeaderInstance] header, until
// closed, as when its factory is. [NewAdaptor] creates one from an adaptor
// and a broker taken elsewhere.
//
// The adaptor wrapped must implement
// [github.com/cjdias/flam-in-go/pkg/flam.ICacheKeyedAdaptor], as the
// memory and redis adaptors do, and be another one than the invalidating
// adaptor itself; two invalidating adaptors wrapping each other, directly
// or not, wait for each other forever when created.
//
// The keys are removed as well as the broker delivers them: a message lost
// by a broker delivering at most once, as Redis Pub/Sub does, or published
// while a process is not subscribed, leaves its values in that process
// until they expire or are evicted. An adaptor failing to publish the keys
// returns an error wrapping [ErrPublish], the values being removed or
// stored in the adaptor it wraps already. A message whose payload is not a
// JSON array of keys, or whose keys cannot be removed, fails its handler,
// as the broker driver handles it. Once the subscription ends, as when the
// broker is closed, the adaptor no longer removes the keys of the others.
package invalidating
