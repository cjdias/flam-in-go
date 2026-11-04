//go:build !windows && !plan9

package syslog

import (
	"log/syslog"
	"strings"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/log/streams/internal/base"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config      flam.IConfig
	Serializers flam.ILogSerializerFactory
}

// newCreator returns the [flam.ILogStreamCreator] of the syslog streams,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ILogStreamCreator {
	return &creator{
		config:      args.Config,
		serializers: args.Serializers,
	}
}

// creator is the [flam.ILogStreamCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config      flam.IConfig
	serializers flam.ILogSerializerFactory
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns a stream connected to the daemon, and closing the
// connection when closed.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ILogStream, error) {
	id := bag.String(factory.ConfigFieldId)

	settings, invalid := base.Read(c.config, bag, base.Defaults{
		LevelPath:      DefaultLevelConfigPath,
		Level:          defaultLevel,
		SerializerPath: DefaultSerializerConfigPath,
		Serializer:     defaultSerializer,
	})
	if invalid != "" {
		return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, invalid)
	}
	texts := map[string]string{}
	for _, field := range []struct{ name, path string }{
		{ConfigFieldNetwork, DefaultNetworkConfigPath},
		{ConfigFieldAddress, DefaultAddressConfigPath},
		{ConfigFieldTag, DefaultTagConfigPath},
	} {
		text, ok := base.Field(c.config, bag, field.name, field.path, "").(string)
		if !ok {
			return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, field.name)
		}
		texts[field.name] = text
	}
	name, _ := base.Field(c.config, bag, ConfigFieldFacility, DefaultFacilityConfigPath, defaultFacility).(string)
	facility, ok := creatorFacilities[strings.ToLower(name)]
	if !ok {
		return nil, base.NewErrInvalidField(sentinels, creatorCreateMessage, id, ConfigFieldFacility)
	}

	serializer, err := base.Serializer(c.serializers, sentinels, creatorCreateMessage, id, settings.Serializer)
	if err != nil {
		return nil, err
	}
	network, address := texts[ConfigFieldNetwork], texts[ConfigFieldAddress]
	conn, err := syslog.Dial(network, address, facility|syslog.LOG_INFO, texts[ConfigFieldTag])
	if err != nil {
		return nil, newErrDial(creatorCreateMessage, id, network, address, err)
	}
	w := &writer{conn: conn}
	return base.New(id, settings.Level, settings.Channels, leveled{serializer, w}, w, w), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create syslog log stream"

// creatorFacilities are the facilities, by name.
var creatorFacilities = map[string]syslog.Priority{
	"kern":     syslog.LOG_KERN,
	"user":     syslog.LOG_USER,
	"mail":     syslog.LOG_MAIL,
	"daemon":   syslog.LOG_DAEMON,
	"auth":     syslog.LOG_AUTH,
	"syslog":   syslog.LOG_SYSLOG,
	"lpr":      syslog.LOG_LPR,
	"news":     syslog.LOG_NEWS,
	"uucp":     syslog.LOG_UUCP,
	"cron":     syslog.LOG_CRON,
	"authpriv": syslog.LOG_AUTHPRIV,
	"ftp":      syslog.LOG_FTP,
	"local0":   syslog.LOG_LOCAL0,
	"local1":   syslog.LOG_LOCAL1,
	"local2":   syslog.LOG_LOCAL2,
	"local3":   syslog.LOG_LOCAL3,
	"local4":   syslog.LOG_LOCAL4,
	"local5":   syslog.LOG_LOCAL5,
	"local6":   syslog.LOG_LOCAL6,
	"local7":   syslog.LOG_LOCAL7,
}
