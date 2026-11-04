package s3

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/disk/disks/internal/objectfs"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// newCreator returns the [flam.IDiskCreator] of the s3 disks, reading the
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
// client of the AWS configuration loaded with its region and credentials.
// No request is sent to the storage.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IDisk, error) {
	id := bag.String(factory.ConfigFieldId)

	cfg, err := c.read(id, bag)
	if err != nil {
		return nil, err
	}

	var options []func(*awsconfig.LoadOptions) error
	if cfg.region != "" {
		options = append(options, awsconfig.WithRegion(cfg.region))
	}
	if cfg.accessKey != "" {
		options = append(options, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.accessKey, cfg.secretKey, cfg.sessionToken)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), options...)
	if err != nil {
		return nil, newErrConfig(creatorCreateMessage, id, err)
	}

	client := awss3.NewFromConfig(awsCfg, func(o *awss3.Options) {
		if cfg.endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.endpoint)
		}
		o.UsePathStyle = cfg.pathStyle
		// the S3 compatible storages may answer without checksums
		o.DisableLogOutputChecksumValidationSkipped = true
	})
	return objectfs.NewDisk(id, diskName, cfg.prefix, &store{client: client, bucket: cfg.bucket}), nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create s3 disk"

// diskConfig is the configuration of a disk, read by the creator.
type diskConfig struct {
	bucket       string
	prefix       string
	region       string
	endpoint     string
	pathStyle    bool
	accessKey    string
	secretKey    string
	sessionToken string
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
	if cfg.bucket, ok = c.field(bag, ConfigFieldBucket, "", "").(string); !ok || cfg.bucket == "" {
		return nil, invalid(ConfigFieldBucket)
	}
	if cfg.prefix, ok = c.field(bag, ConfigFieldPrefix, "", "").(string); !ok {
		return nil, invalid(ConfigFieldPrefix)
	}
	if cfg.region, ok = c.field(bag, ConfigFieldRegion, DefaultRegionConfigPath, defaultRegion).(string); !ok {
		return nil, invalid(ConfigFieldRegion)
	}
	if cfg.endpoint, ok = c.field(bag, ConfigFieldEndpoint, DefaultEndpointConfigPath, defaultEndpoint).(string); !ok {
		return nil, invalid(ConfigFieldEndpoint)
	}
	if cfg.pathStyle, ok = c.field(bag, ConfigFieldPathStyle, DefaultPathStyleConfigPath, defaultPathStyle).(bool); !ok {
		return nil, invalid(ConfigFieldPathStyle)
	}
	if cfg.accessKey, ok = c.field(bag, ConfigFieldAccessKey, "", "").(string); !ok {
		return nil, invalid(ConfigFieldAccessKey)
	}
	if cfg.secretKey, ok = c.field(bag, ConfigFieldSecretKey, "", "").(string); !ok || (cfg.secretKey == "") != (cfg.accessKey == "") {
		return nil, invalid(ConfigFieldSecretKey)
	}
	if cfg.sessionToken, ok = c.field(bag, ConfigFieldSessionToken, "", "").(string); !ok || (cfg.sessionToken != "" && cfg.accessKey == "") {
		return nil, invalid(ConfigFieldSessionToken)
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
