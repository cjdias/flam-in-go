package sqlite

import (
	"net/url"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/database"
	"github.com/cjdias/flam-in-go/pkg/flam/database/dialects/internal/base"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
)

// newCreator returns the [flam.IDatabaseDialectCreator] of the SQLite
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

// Create returns the dialect of the connection string of the path and the
// parameters.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IDatabaseDialect, error) {
	id := bag.String(factory.ConfigFieldId)

	path, ok := base.NonEmpty(base.Field(c.config, bag, ConfigFieldPath, DefaultPathConfigPath, defaultPath))
	if !ok {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, ConfigFieldPath)
	}
	params, ok := base.Params(base.Field(c.config, bag, ConfigFieldParams, DefaultParamsConfigPath, nil))
	if !ok {
		return nil, base.NewErrInvalidField(ErrInvalidField, creatorCreateMessage, id, ConfigFieldParams)
	}

	dsn := creatorDSN(path, params)
	return database.NewDialect(id, func() gorm.Dialector { return sqlite.Open(dsn) })
}

// creatorDSN returns the connection string of the path and the
// parameters, written as a URL query in name order, so that a value
// holding "&", "=" or a space stays one value.
func creatorDSN(
	path string,
	params map[string]string,
) string {
	if len(params) == 0 {
		return path
	}

	query := url.Values{}
	for key, val := range params {
		query.Set(key, val)
	}
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}
	return path + separator + query.Encode()
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create sqlite database dialect"
