package gcpsm

import (
	"context"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"go.uber.org/dig"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

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

// newCreator returns the [flam.ISecretStoreCreator] of the gcpsm stores,
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
// defaults, with its own Secret Manager client.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ISecretStore, error) {
	id := bag.String(factory.ConfigFieldId)

	strs := map[string]string{}
	for _, f := range []struct{ field, path, builtIn string }{
		{ConfigFieldProject, DefaultProjectConfigPath, defaultProject},
		{ConfigFieldVersion, DefaultVersionConfigPath, defaultVersion},
		{ConfigFieldEndpoint, DefaultEndpointConfigPath, defaultEndpoint},
		{ConfigFieldCredentialsFile, DefaultCredentialsFileConfigPath, defaultCredentialsFile},
		{ConfigFieldCredentialsJSON, DefaultCredentialsJSONConfigPath, defaultCredentialsJSON},
		{ConfigFieldCredentialsType, DefaultCredentialsTypeConfigPath, defaultCredentialsType},
	} {
		str, ok := c.field(bag, f.field, f.path, f.builtIn).(string)
		if !ok {
			return nil, newErrInvalidField(creatorCreateMessage, id, f.field)
		}
		strs[f.field] = str
	}
	for _, field := range []string{ConfigFieldProject, ConfigFieldVersion} {
		if strs[field] == "" || strings.Contains(strs[field], "/") {
			return nil, newErrInvalidField(creatorCreateMessage, id, field)
		}
	}
	credentialsFile, credentialsJSON := strs[ConfigFieldCredentialsFile], strs[ConfigFieldCredentialsJSON]
	if credentialsFile != "" && credentialsJSON != "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldCredentialsJSON)
	}
	credentialsType, ok := creatorCredentialsTypes[strs[ConfigFieldCredentialsType]]
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldCredentialsType)
	}
	plaintext, ok := c.field(bag, ConfigFieldInsecure, DefaultInsecureConfigPath, defaultInsecure).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldInsecure)
	}
	endpoint := strs[ConfigFieldEndpoint]
	if plaintext && endpoint == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldEndpoint)
	}
	timeout, ok := settings.Duration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimeout)
	}
	ttl, ok := settings.Duration(c.field(bag, ConfigFieldCacheTTL, DefaultCacheTTLConfigPath, defaultCacheTTL))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldCacheTTL)
	}

	var options []option.ClientOption
	if endpoint != "" {
		options = append(options, option.WithEndpoint(endpoint))
	}
	switch {
	case plaintext:
		options = append(options,
			option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		)
	case credentialsFile != "":
		options = append(options, option.WithAuthCredentialsFile(credentialsType, credentialsFile))
	case credentialsJSON != "":
		options = append(options, option.WithAuthCredentialsJSON(credentialsType, []byte(credentialsJSON)))
	}
	client, err := secretmanager.NewClient(context.Background(), options...)
	if err != nil {
		if credentialsJSON != "" && !plaintext {
			err = nil // dropped, as it may quote the credentials
		}
		return nil, newErr(ErrClient, creatorCreateMessage, id, err)
	}

	return &store{
		id:      id,
		client:  client,
		prefix:  "projects/" + strs[ConfigFieldProject] + "/secrets/",
		version: strs[ConfigFieldVersion],
		timeout: timeout,
		ttl:     ttl,
		clock:   c.clock,
		cache:   map[string]storeEntry{},
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create gcpsm secret store"

// creatorCredentialsTypes are the credentials types, by their value of
// [ConfigFieldCredentialsType].
var creatorCredentialsTypes = map[string]option.CredentialsType{
	string(option.ServiceAccount):             option.ServiceAccount,
	string(option.AuthorizedUser):             option.AuthorizedUser,
	string(option.ImpersonatedServiceAccount): option.ImpersonatedServiceAccount,
	string(option.ExternalAccount):            option.ExternalAccount,
}

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
