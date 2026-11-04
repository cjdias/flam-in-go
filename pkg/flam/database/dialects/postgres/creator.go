package postgres

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IDatabaseDialectCreator] of the PostgreSQL
// dialects, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned
// from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.IDatabaseDialectCreator {
	return &creator{config: config}
}

// creator is the [flam.IDatabaseDialectCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the dialect of the connection string of the fields.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IDatabaseDialect, error) {
	id := bag.String(factory.ConfigFieldId)

	server, invalid := base.ReadServer(c.config, bag, creatorFields)
	if invalid != "" {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, invalid)
	}

	params := server.Params
	maps.Copy(params, map[string]string{
		"host":     server.Host,
		"port":     strconv.Itoa(server.Port),
		"dbname":   server.Database,
		"user":     server.Username,
		"password": server.Password,
	})
	dsn := creatorDSN(params)
	return database.NewDialect(id, func() gorm.Dialector { return postgres.Open(dsn) })
}

// creatorFields are the fields of a postgres dialect configuration.
var creatorFields = base.ServerFields{
	Host:     base.ServerField{Name: ConfigFieldHost, DefaultPath: DefaultHostConfigPath, BuiltIn: defaultHost},
	Port:     base.ServerField{Name: ConfigFieldPort, DefaultPath: DefaultPortConfigPath, BuiltIn: defaultPort},
	Database: base.ServerField{Name: ConfigFieldDatabase, DefaultPath: DefaultDatabaseConfigPath, BuiltIn: defaultDatabase},
	Username: base.ServerField{Name: ConfigFieldUsername, DefaultPath: DefaultUsernameConfigPath, BuiltIn: defaultUsername},
	Password: base.ServerField{Name: ConfigFieldPassword, DefaultPath: DefaultPasswordConfigPath, BuiltIn: defaultPassword},
	Params:   base.ServerField{Name: ConfigFieldParams, DefaultPath: DefaultParamsConfigPath},
}

// creatorDSN returns the keyword/value connection string of the entries,
// in keyword order, so that the same configuration gives the same string.
func creatorDSN(
	entries map[string]string,
) string {
	parts := make([]string, 0, len(entries))
	for _, key := range slices.Sorted(maps.Keys(entries)) {
		parts = append(parts, key+"="+creatorQuote(entries[key]))
	}
	return strings.Join(parts, " ")
}

// creatorQuote quotes the value of a keyword/value connection string,
// escaping its backslashes and single quotes, so that a value holding a
// space or a quote stays one value.
func creatorQuote(
	val string,
) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(val) + "'"
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create postgres database dialect"
