package cors

import (
	"slices"
	"strconv"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the CORS
// middlewares, reading the defaults of the missing fields from the
// configuration. It is the provider registered by the bundle returned from
// [NewBundle].
func newCreator(
	args creatorArgs,
) flam.IRestMiddlewareCreator {
	return &creator{
		config: args.Config,
	}
}

// creator is the [flam.IRestMiddlewareCreator] of the configurations with
// the [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the middleware of the allowed origins, methods and
// headers, the exposed headers, the credentials allowance and the max age
// of the configuration, or of their defaults, failing with
// ErrWildcardCredentials when the credentials are allowed along with a
// wildcard origin or exposed header.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	lists := map[string][]string{}
	for field, def := range map[string]struct {
		path    string
		builtIn []string
	}{
		ConfigFieldAllowedOrigins: {DefaultAllowedOriginsConfigPath, defaultAllowedOrigins()},
		ConfigFieldAllowedMethods: {DefaultAllowedMethodsConfigPath, defaultAllowedMethods()},
		ConfigFieldAllowedHeaders: {DefaultAllowedHeadersConfigPath, defaultAllowedHeaders()},
		ConfigFieldExposedHeaders: {DefaultExposedHeadersConfigPath, defaultExposedHeaders()},
	} {
		list, ok := creatorList(c.field(bag, field, def.path, def.builtIn))
		if !ok {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field)
		}
		lists[field] = list
	}
	credentials, ok := c.field(bag, ConfigFieldAllowCredentials, DefaultAllowCredentialsConfigPath, defaultAllowCredentials).(bool)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldAllowCredentials)
	}
	maxAge, ok := settings.Duration(c.field(bag, ConfigFieldMaxAge, DefaultMaxAgeConfigPath, defaultMaxAge))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldMaxAge)
	}
	if credentials {
		for _, field := range []string{ConfigFieldAllowedOrigins, ConfigFieldExposedHeaders} {
			if slices.Contains(lists[field], Wildcard) {
				return nil, newErrCreate(ErrWildcardCredentials, creatorCreateMessage, id, field)
			}
		}
	}

	m := &middleware{
		id:               id,
		origins:          creatorSet(lists[ConfigFieldAllowedOrigins], strings.ToLower),
		anyOrigin:        slices.Contains(lists[ConfigFieldAllowedOrigins], Wildcard),
		methods:          creatorSet(lists[ConfigFieldAllowedMethods], nil),
		allowMethods:     strings.Join(lists[ConfigFieldAllowedMethods], ", "),
		headers:          creatorSet(lists[ConfigFieldAllowedHeaders], strings.ToLower),
		anyHeader:        slices.Contains(lists[ConfigFieldAllowedHeaders], Wildcard),
		exposeHeaders:    strings.Join(lists[ConfigFieldExposedHeaders], ", "),
		allowCredentials: credentials,
	}
	if seconds := int64(maxAge.Seconds()); seconds > 0 {
		m.maxAge = strconv.FormatInt(seconds, 10)
	}
	return m, nil
}

// field returns the value of the field of the middleware configuration,
// or else the one stored in the configuration at the default path, or
// else the built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorList returns the strings of the value, a []string or a []any of
// strings, trimmed, and whether the value is such a list of non-empty
// strings.
func creatorList(
	value any,
) ([]string, bool) {
	list, ok := settings.Strings(value)
	if !ok {
		return nil, false
	}
	trimmed := make([]string, len(list))
	for i, item := range list {
		if trimmed[i] = strings.TrimSpace(item); trimmed[i] == "" {
			return nil, false
		}
	}
	return trimmed, true
}

// creatorSet returns the set of the strings of the list, each passed
// through normalize when not nil, such as to lower case the origins and
// the headers, matched whatever their case, unlike the methods.
func creatorSet(
	list []string,
	normalize func(string) string,
) map[string]struct{} {
	set := make(map[string]struct{}, len(list))
	for _, item := range list {
		if normalize != nil {
			item = normalize(item)
		}
		set[item] = struct{}{}
	}
	return set
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create cors rest middleware"
