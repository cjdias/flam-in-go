package vault

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/url"
	"os"
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
	Clock  flam.IClock
}

// newCreator returns the [flam.ISecretStoreCreator] of the vault stores,
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
// defaults, with its own HTTP client.
func (c *creator) Create(
	bag flam.IBag,
) (flam.ISecretStore, error) {
	id := bag.String(factory.ConfigFieldId)

	strs := map[string]string{}
	for _, f := range []struct{ field, path, builtIn string }{
		{ConfigFieldAddress, DefaultAddressConfigPath, defaultAddress},
		{ConfigFieldMount, DefaultMountConfigPath, defaultMount},
		{ConfigFieldNamespace, DefaultNamespaceConfigPath, defaultNamespace},
		{ConfigFieldToken, DefaultTokenConfigPath, defaultToken},
		{ConfigFieldTokenFile, DefaultTokenFileConfigPath, defaultTokenFile},
		{ConfigFieldCAFile, DefaultCAFileConfigPath, defaultCAFile},
	} {
		str, ok := c.field(bag, f.field, f.path, f.builtIn).(string)
		if !ok {
			return nil, newErrInvalidField(creatorCreateMessage, id, f.field)
		}
		strs[f.field] = str
	}
	address := strings.TrimRight(strs[ConfigFieldAddress], "/")
	if !creatorValidAddress(address) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldAddress)
	}
	mount := strings.Trim(strs[ConfigFieldMount], "/")
	if !storeValidPath(mount) {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldMount)
	}
	timeout, ok := settings.Duration(c.field(bag, ConfigFieldTimeout, DefaultTimeoutConfigPath, defaultTimeout))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldTimeout)
	}
	ttl, ok := settings.Duration(c.field(bag, ConfigFieldCacheTTL, DefaultCacheTTLConfigPath, defaultCacheTTL))
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldCacheTTL)
	}
	skipVerify, ok := c.field(bag, ConfigFieldSkipVerify, DefaultSkipVerifyConfigPath, defaultSkipVerify).(bool)
	if !ok {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldSkipVerify)
	}

	token, tokenFile := strs[ConfigFieldToken], strs[ConfigFieldTokenFile]
	if token == "" && tokenFile == "" {
		token = os.Getenv(EnvToken)
	}
	if token == "" && tokenFile == "" {
		return nil, newErrInvalidField(creatorCreateMessage, id, ConfigFieldToken)
	}
	if token != "" {
		tokenFile = ""
	}

	transport, err := creatorTransport(id, strs[ConfigFieldCAFile], skipVerify)
	if err != nil {
		return nil, err
	}
	return &store{
		id:        id,
		base:      address + "/v1/" + storeEscape(mount) + "/data/",
		namespace: strs[ConfigFieldNamespace],
		token:     token,
		tokenFile: tokenFile,
		timeout:   timeout,
		ttl:       ttl,
		clock:     c.clock,
		transport: transport,
		client:    &http.Client{Transport: transport},
		cache:     map[string]storeEntry{},
	}, nil
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create vault secret store"

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

// creatorValidAddress reports whether the address is an absolute http or
// https URL, with a host.
func creatorValidAddress(
	address string,
) bool {
	parsed, err := url.Parse(address)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

// creatorTransport returns a new transport, as the default one, trusting
// the certificate authorities of the PEM file, when not empty, instead of
// those of the system, and verifying the certificate of the server unless
// skipVerify. It fails with ErrCAFile when the file cannot be read, or
// holds no certificate.
func creatorTransport(
	id string,
	caFile string,
	skipVerify bool,
) (*http.Transport, error) {
	var pool *x509.CertPool
	if caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, newErr(ErrCAFile, creatorCreateMessage, id, err)
		}
		pool = x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, newErr(ErrCAFile, creatorCreateMessage, id, nil)
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            pool,
		InsecureSkipVerify: skipVerify, // asked for by the configuration, for tests only
	}
	return transport, nil
}
