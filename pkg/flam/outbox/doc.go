// Package outbox is the transactional outbox feature: [Add] and [AddJSON]
// store a message in the [Table] table with the transaction of the caller,
// so that the message is published if, and only if, the changes of the
// transaction commit, and the relays of the bundle returned from
// [NewBundle] publish the stored messages with a broker of
// [github.com/cjdias/flam-in-go/pkg/flam/broker]:
//
//	err := connection.Transaction(func(tx *gorm.DB) error {
//		if err := tx.Create(&order).Error; err != nil {
//			return err
//		}
//		return outbox.AddJSON(tx, "orders.created", order, nil)
//	})
//
// The relays are configured at [RelayConfigPath], by id, with the
// connection holding the table and the broker publishing the messages:
//
//	flam:
//	  outbox:
//	    relays:
//	      main: {connection: main, broker: events, batch: 50}
//
// Each relay is a process of the kennel of
// [github.com/cjdias/flam-in-go/pkg/flam/watchdog], under the id "outbox-"
// followed by the relay id, unless set at [ConfigFieldProcess]. While it
// runs, it claims a batch of up to [ConfigFieldBatch] unsent messages, in
// id order, setting a new claim token and a lease of [ConfigFieldLease] on
// each with a conditional update failing when another relay claimed it
// first, publishes them in that order, then marks them sent, or deletes
// them when [ConfigFieldDelete] is set. It claims the next batch at once
// when the batch was full, and polls every [ConfigFieldPoll] otherwise.
// The sent messages are deleted once [ConfigFieldRetention] elapsed, every
// [ConfigFieldCleanup].
//
// The delivery is at least once: a relay crashing after the broker
// accepted a message, but before marking it sent, leaves it claimed until
// its lease expires, after which a relay publishes it again, so that the
// consumers must be idempotent, such as by the id of the message. A
// message the broker refuses is published again after a backoff of
// [ConfigFieldBackoff], doubled at each attempt up to
// [ConfigFieldMaxBackoff], forever, its last error kept in the table.
//
// The messages of a topic are published in the order they were added, as
// a message is not claimed while an earlier message of its topic is
// unsent and claimed by a relay, or waiting for its retry: a message
// failing holds the later messages of its topic back, while the other
// topics go on. This order holds across the relays sharing the table, as
// long as the leases do not expire while their relays run, but the
// broker, and the redeliveries of the at least once delivery, may change
// it further, as its driver documents. Several relays of the table only
// add availability.
//
// The table is created or updated by the relays when created, with gorm
// AutoMigrate, unless [ConfigFieldMigrate] is false, in which case the
// application creates it, with [Migrate] or the migration of
// [NewMigration], as it must when it adds messages without running a
// relay. The failures of the relays are logged on [LogChannel] when the
// container provides a [github.com/cjdias/flam-in-go/pkg/flam.ILogger].
package outbox
