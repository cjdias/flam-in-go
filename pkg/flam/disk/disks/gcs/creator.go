package gcs

import (
	"context"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/internal/objectfs"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newCreator returns the [flam.IDiskCreator] of the gcs disks, reading the
// defaults of the missing fields from the configuration. It is the
// provider registered by the bundle returned from [NewBundle].
func newCreator(
	config flam.IConfig,
) flam.IDiskCreator {
	return &creator{config: config}
}

// creator is the [flam.IDiskCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the disk over the bucket of the configuration, with a
// new client of its endpoint and credentials. No request is sent to the
// storage.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IDisk, error) {
	id := bag.String(factory.ConfigFieldId)

	cfg, err := c.read(id, bag)
	if err != nil {
		return nil, err
	}

	options := []option.ClientOption{storage.WithJSONReads()}
	if cfg.endpoint != "" {
		options = append(options, option.WithEndpoint(cfg.endpoint))
	}
	switch {
	case cfg.credentialsFile != "":
		options = append(options, option.WithAuthCredentialsFile(cfg.credentialsType, cfg.credentialsFile))
	case cfg.credentialsJSON != "":
		options = append(options, option.WithAuthCredentialsJSON(cfg.credentialsType, []byte(cfg.credentialsJSON)))
	case cfg.anonymous:
		options = append(options, option.WithoutAuthentication())
	}
	client, err := storage.NewClient(context.Background(), options...)
	if err != nil {
		return nil, newErrClient(creatorCreateMessage, id, err)
	}
	return objectfs.NewDisk(id, diskName, cfg.prefix, &store{client: client, bucket: client.Bucket(cfg.bucket)}), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create gcs disk"

// creatorCredentialsTypes are the credentials types of
// ConfigFieldCredentialsType, by their name.
var creatorCredentialsTypes = map[string]option.CredentialsType{
	"service_account":              option.ServiceAccount,
	"authorized_user":              option.AuthorizedUser,
	"impersonated_service_account": option.ImpersonatedServiceAccount,
	"external_account":             option.ExternalAccount,
}

// diskConfig is the configuration of a disk, read by the creator.
type diskConfig struct {
	bucket          string
	prefix          string
	endpoint        string
	credentialsFile string
	credentialsJSON string
	credentialsType option.CredentialsType
	anonymous       bool
}

// read returns the configuration of the disk of the id, from its fields
// or their defaults, failing with ErrInvalidField for an invalid one.
func (c *creator) read(
	id string,
	bag flam.IBag,
) (*diskConfig, error) {
	invalid := func(field string) error {
		return newErrInvalidField(creatorCreateMessage, id, field)
	}

	cfg := &diskConfig{}
	var ok bool
	var credentialsType string
	if cfg.bucket, ok = c.field(bag, ConfigFieldBucket, "", "").(string); !ok || cfg.bucket == "" {
		return nil, invalid(ConfigFieldBucket)
	}
	if cfg.prefix, ok = c.field(bag, ConfigFieldPrefix, "", "").(string); !ok {
		return nil, invalid(ConfigFieldPrefix)
	}
	if cfg.endpoint, ok = c.field(bag, ConfigFieldEndpoint, DefaultEndpointConfigPath, defaultEndpoint).(string); !ok {
		return nil, invalid(ConfigFieldEndpoint)
	}
	if cfg.credentialsFile, ok = c.field(bag, ConfigFieldCredentialsFile, "", "").(string); !ok {
		return nil, invalid(ConfigFieldCredentialsFile)
	}
	if cfg.credentialsJSON, ok = c.field(bag, ConfigFieldCredentialsJSON, "", "").(string); !ok || (cfg.credentialsJSON != "" && cfg.credentialsFile != "") {
		return nil, invalid(ConfigFieldCredentialsJSON)
	}
	if credentialsType, ok = c.field(bag, ConfigFieldCredentialsType, DefaultCredentialsTypeConfigPath, defaultCredentialsType).(string); ok {
		cfg.credentialsType, ok = creatorCredentialsTypes[credentialsType]
	}
	if !ok {
		return nil, invalid(ConfigFieldCredentialsType)
	}
	if cfg.anonymous, ok = c.field(bag, ConfigFieldAnonymous, DefaultAnonymousConfigPath, defaultAnonymous).(bool); !ok || (cfg.anonymous && (cfg.credentialsFile != "" || cfg.credentialsJSON != "")) {
		return nil, invalid(ConfigFieldAnonymous)
	}
	return cfg, nil
}

// field returns the value of the field of the disk configuration, or else
// the one stored in the configuration at the default path, or else the
// built-in default, as the field is optional.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}
