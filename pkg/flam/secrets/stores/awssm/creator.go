package awssm

import (
	"context"
	"net/http"
	"net/url"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
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
	Clock  flam.IClock
}

// newCreator returns the [flam.ISecretStoreCreator] of the awssm stores,
// reading the defaults of the missing fields from the configuration. It is
// the provider registered by the bundle returned from [NewBundle].
func newCreator(
	args creatorArgs,
) flam.ISecretStoreCreator {
	return &creator{config: args.Config, clock: args.Clock}
}

// creator is the [flam.ISecretStoreCreator] of the configurations with the
// [Driver] driver.
type creator struct {
	config flam.IConfig
	clock  flam.IClock
}

func (c *creator) Accept(
	bag flam.IBag,
) bool {
	return bag.String(factory.ConfigFieldDriver) == Driver
}

// Create returns the store of the fields of the configuration, or of their
// defaults, with its own Secrets Manager client.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ISecretStore, error) {
	id := bag.String(factory.ConfigFieldId)

	strs := map[string]string{}
	for _, f := range []struct{ field, path, builtIn string }{
		{ConfigFieldRegion, DefaultRegionConfigPath, defaultRegion},
		{ConfigFieldEndpoint, DefaultEndpointConfigPath, defaultEndpoint},
		{ConfigFieldProfile, DefaultProfileConfigPath, defaultProfile},
		{ConfigFieldAccessKeyID, DefaultAccessKeyIDConfigPath, defaultAccessKeyID},
		{ConfigFieldSecretAccessKey, DefaultSecretAccessKeyConfigPath, defaultSecretAccessKey},
		{ConfigFieldSessionToken, DefaultSessionTokenConfigPath, defaultSessionToken},
		{ConfigFieldVersionStage, DefaultVersionStageConfigPath, defaultVersionStage},
	} {
		str, ok := c.field(bag, f.field, f.path, f.builtIn).(string)
		if !ok {
			return nil, newErrInvalidField(creatorCreateMessage, id, f.field)
		}
		strs[f.field] = str
	}
	endpoint := strs[ConfigFieldEndpoint]
	if endpoint != "" && !creatorValidEndpoint(endpoint) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldEndpoint)
	}
	accessKeyID, secretAccessKey := strs[ConfigFieldAccessKeyID], strs[ConfigFieldSecretAccessKey]
	if accessKeyID == "" && secretAccessKey != "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldAccessKeyID)
	}
	if secretAccessKey == "" && (accessKeyID != "" || strs[ConfigFieldSessionToken] != "") {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldSecretAccessKey)
	}
	if strs[ConfigFieldVersionStage] == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldVersionStage)
	}
	timeout, ok := settings.Duration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimeout)
	}
	ttl, ok := settings.Duration(c.field(bag, ConfigFieldCacheTTL, DefaultCacheTTLConfigPath, defaultCacheTTL))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldCacheTTL)
	}
	maxAttempts, ok := c.field(bag, ConfigFieldMaxAttempts, DefaultMaxAttemptsConfigPath, defaultMaxAttempts).(int)
	if !ok || maxAttempts <= 0 {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldMaxAttempts)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	options := []func(*config.LoadOptions) error{
		config.WithHTTPClient(&http.Client{Transport: transport}),
		config.WithRetryMaxAttempts(maxAttempts),
	}
	if region := strs[ConfigFieldRegion]; region != "" {
		options = append(options, config.WithRegion(region))
	}
	if endpoint != "" {
		options = append(options, config.WithBaseEndpoint(endpoint))
	}
	if profile := strs[ConfigFieldProfile]; profile != "" {
		options = append(options, config.WithSharedConfigProfile(profile))
	}
	if accessKeyID != "" {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID, secretAccessKey, strs[ConfigFieldSessionToken],
		)))
	}
	awsConfig, err := config.LoadDefaultConfig(context.Background(), options...)
	if err != nil {
		return nil, newErr(ErrClient, creatorCreateMessage, id, err)
	}

	return &store{
		id:        id,
		client:    secretsmanager.NewFromConfig(awsConfig),
		transport: transport,
		stage:     strs[ConfigFieldVersionStage],
		timeout:   timeout,
		ttl:       ttl,
		clock:     c.clock,
		cache:     map[string]storeEntry{},
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create awssm secret store"

// field returns the value of the field of the store configuration, or else
// its default.
func (c *creator) field(
	bag flam.IBag,
	field string,
	defaultPath string,
	builtIn any,
) any {
	return settings.Field(c.config, bag, field, defaultPath, builtIn)
}

// creatorValidEndpoint reports whether the endpoint is an absolute http or
// https URL, with a host.
func creatorValidEndpoint(
	endpoint string,
) bool {
	parsed, err := url.Parse(endpoint)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}
