package flam

import "io"

type DatabaseConnectionFactory interface {
	io.Closer

	Available() []string
	Stored() []string
	Has(id string) bool
	Get(id string) (DatabaseConnection, error)
	Store(id string, connection DatabaseConnection) error
	Remove(id string) error
	RemoveAll() error
}

func newDatabaseConnectionFactory(
	connectionCreator *databaseConnectionCreator,
	factoryConfig FactoryConfig,
) (DatabaseConnectionFactory, error) {
	creators := []FactoryResourceCreator[DatabaseConnection]{connectionCreator}

	return NewFactory(
		creators,
		factoryConfig,
		nil,
		PathDatabaseConnections)
}
