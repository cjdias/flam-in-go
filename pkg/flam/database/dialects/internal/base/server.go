package base

import (
	"github.com/cjdias/flam-in-go/pkg/flam"
)

// ServerField names a field of a server dialect configuration, the path of
// its default in the configuration, and its built-in default.
type ServerField struct {
	Name        string
	DefaultPath string
	BuiltIn     any
}

// ServerFields names the fields of the configuration of a dialect of a
// database server, such as mysql and postgres.
type ServerFields struct {
	Host     ServerField
	Port     ServerField
	Database ServerField
	Username ServerField
	Password ServerField
	Params   ServerField
}

// Server holds the values of the fields of the configuration of a dialect
// of a database server.
type Server struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	Params   map[string]string
}

// ReadServer returns the values of the fields of the dialect
// configuration, or their defaults, in the order of ServerFields, or the
// name of the first invalid one: a non-empty host, database and username,
// a port, a password string, which may be empty, and parameters.
func ReadServer(
	config flam.IConfig,
	bag flam.IBag,
	fields ServerFields,
) (Server, string) {
	read := func(f ServerField) any {
		return Field(config, bag, f.Name, f.DefaultPath, f.BuiltIn)
	}

	var server Server
	var ok bool
	if server.Host, ok = NonEmpty(read(fields.Host)); !ok {
		return Server{}, fields.Host.Name
	}
	if server.Port, ok = Port(read(fields.Port)); !ok {
		return Server{}, fields.Port.Name
	}
	if server.Database, ok = NonEmpty(read(fields.Database)); !ok {
		return Server{}, fields.Database.Name
	}
	if server.Username, ok = NonEmpty(read(fields.Username)); !ok {
		return Server{}, fields.Username.Name
	}
	if server.Password, ok = Text(read(fields.Password)); !ok {
		return Server{}, fields.Password.Name
	}
	if server.Params, ok = Params(read(fields.Params)); !ok {
		return Server{}, fields.Params.Name
	}
	return server, ""
}
