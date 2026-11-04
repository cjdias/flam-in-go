package database

import (
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"

	"github.com/cjdias/flam-in-go/pkg/flam"
)

// openedDialector is the gorm dialector of a replica or an extra source,
// recording the gorm database it opens the pool of, for the connection to
// close it.
type openedDialector struct {
	gorm.Dialector

	pools *[]*gorm.DB
}

func (d *openedDialector) Initialize(
	db *gorm.DB,
) error {
	*d.pools = append(*d.pools, db)
	return d.Dialector.Initialize(db)
}

// pooledDialector is the gorm dialector of the dialect of a connection
// among its sources, giving the pool the connection already opened rather
// than opening another.
type pooledDialector struct {
	gorm.Dialector

	pool gorm.ConnPool
}

func (d *pooledDialector) Initialize(
	db *gorm.DB,
) error {
	db.ConnPool = d.pool
	return nil
}

// connectionCreatorOpened returns the gorm dialectors of the dialects,
// recording in pools the gorm databases they open.
func connectionCreatorOpened(
	dialects []flam.IDatabaseDialect,
	pools *[]*gorm.DB,
) []gorm.Dialector {
	dialectors := make([]gorm.Dialector, 0, len(dialects))
	for _, dialect := range dialects {
		dialectors = append(dialectors, &openedDialector{Dialector: dialect.Gorm(), pools: pools})
	}
	return dialectors
}

// connectionCreatorPolicy returns the dbresolver policy of the name, a
// valid one. The round robin one is the strict one, safe for concurrent
// use.
func connectionCreatorPolicy(
	name string,
) dbresolver.Policy {
	if name == PolicyRoundRobin {
		return dbresolver.StrictRoundRobinPolicy()
	}
	return dbresolver.RandomPolicy{}
}
