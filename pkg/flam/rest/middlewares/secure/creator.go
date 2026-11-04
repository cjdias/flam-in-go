package secure

import (
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

// newCreator returns the [flam.IRestMiddlewareCreator] of the secure
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

// Create returns the middleware of the header values, the HSTS directives
// and the HSTS forcing of the configuration, or of their defaults, holding
// the headers to set, the empty ones left out.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	m := &middleware{id: id}
	for _, h := range []struct {
		name    string
		field   string
		path    string
		builtIn string
	}{
		{"X-Content-Type-Options", ConfigFieldContentTypeOptions, DefaultContentTypeOptionsConfigPath, defaultContentTypeOptions},
		{"X-Frame-Options", ConfigFieldFrameOptions, DefaultFrameOptionsConfigPath, defaultFrameOptions},
		{"Referrer-Policy", ConfigFieldReferrerPolicy, DefaultReferrerPolicyConfigPath, defaultReferrerPolicy},
		{"Content-Security-Policy", ConfigFieldContentSecurityPolicy, DefaultContentSecurityPolicyConfigPath, defaultContentSecurityPolicy},
		{"Permissions-Policy", ConfigFieldPermissionsPolicy, DefaultPermissionsPolicyConfigPath, defaultPermissionsPolicy},
		{"Cross-Origin-Opener-Policy", ConfigFieldCrossOriginOpenerPolicy, DefaultCrossOriginOpenerPolicyConfigPath, defaultCrossOriginOpenerPolicy},
		{"Cross-Origin-Embedder-Policy", ConfigFieldCrossOriginEmbedderPolicy, DefaultCrossOriginEmbedderPolicyConfigPath, defaultCrossOriginEmbedderPolicy},
		{"Cross-Origin-Resource-Policy", ConfigFieldCrossOriginResourcePolicy, DefaultCrossOriginResourcePolicyConfigPath, defaultCrossOriginResourcePolicy},
	} {
		value, ok := c.field(bag, h.field, h.path, h.builtIn).(string)
		if !ok || strings.ContainsAny(value, "\r\n\x00") {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, h.field)
		}
		if value != "" {
			m.headers = append(m.headers, header{name: h.name, value: value})
		}
	}

	maxAge, ok := settings.Duration(c.field(bag, ConfigFieldHSTSMaxAge, DefaultHSTSMaxAgeConfigPath, defaultHSTSMaxAge))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldHSTSMaxAge)
	}
	flags := map[string]bool{}
	for field, def := range map[string]struct {
		path    string
		builtIn bool
	}{
		ConfigFieldHSTSIncludeSubdomains: {DefaultHSTSIncludeSubdomainsConfigPath, defaultHSTSIncludeSubdomains},
		ConfigFieldHSTSPreload:           {DefaultHSTSPreloadConfigPath, defaultHSTSPreload},
		ConfigFieldForceHSTS:             {DefaultForceHSTSConfigPath, defaultForceHSTS},
	} {
		flag, ok := c.field(bag, field, def.path, def.builtIn).(bool)
		if !ok {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field)
		}
		flags[field] = flag
	}

	if seconds := int64(maxAge.Seconds()); seconds > 0 {
		m.hsts = "max-age=" + strconv.FormatInt(seconds, 10)
		if flags[ConfigFieldHSTSIncludeSubdomains] {
			m.hsts += "; includeSubDomains"
		}
		if flags[ConfigFieldHSTSPreload] {
			m.hsts += "; preload"
		}
	}
	m.forceHSTS = flags[ConfigFieldForceHSTS]
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

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create secure rest middleware"
