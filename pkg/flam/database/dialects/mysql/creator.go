package mysql

import (
	"net"
	"strconv"

	sqldriver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IDatabaseDialectCreator] of the MySQL
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
	invalid := func(name string) error {
		return base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, name)
	}

	protocol, ok := base.NonEmpty(base.Field(c.config, bag, ConfigFieldProtocol, DefaultProtocolConfigPath, defaultProtocol))
	if !ok {
		return nil, invalid(ConfigFieldProtocol)
	}
	server, field := base.ReadServer(c.config, bag, creatorFields)
	if field != "" {
		return nil, invalid(field)
	}

	cfg := sqldriver.NewConfig()
	cfg.ParseTime = true
	cfg.Net = protocol
	cfg.DBName = server.Database
	cfg.User = server.Username
	cfg.Passwd = server.Password
	cfg.Params = server.Params
	cfg.Addr = server.Host
	if cfg.Net != "unix" {
		cfg.Addr = net.JoinHostPort(server.Host, strconv.Itoa(server.Port))
	}
	dsn := cfg.FormatDSN()
	return database.NewDialect(id, func() gorm.Dialector { return gormmysql.Open(dsn) })
}

// creatorFields are the fields of a mysql dialect configuration, its
// schema as the database.
var creatorFields = base.ServerFields{
	Host:     base.ServerField{Name: ConfigFieldHost, DefaultPath: DefaultHostConfigPath, BuiltIn: defaultHost},
	Port:     base.ServerField{Name: ConfigFieldPort, DefaultPath: DefaultPortConfigPath, BuiltIn: defaultPort},
	Database: base.ServerField{Name: ConfigFieldSchema, DefaultPath: DefaultSchemaConfigPath, BuiltIn: defaultSchema},
	Username: base.ServerField{Name: ConfigFieldUsername, DefaultPath: DefaultUsernameConfigPath, BuiltIn: defaultUsername},
	Password: base.ServerField{Name: ConfigFieldPassword, DefaultPath: DefaultPasswordConfigPath, BuiltIn: defaultPassword},
	Params:   base.ServerField{Name: ConfigFieldParams, DefaultPath: DefaultParamsConfigPath},
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create mysql database dialect"
