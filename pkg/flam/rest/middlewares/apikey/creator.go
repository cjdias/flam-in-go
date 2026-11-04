package apikey

import (
	"crypto/sha256"

	"go.uber.org/dig"

	"github.com/cjdias/flam-in-go/pkg/flam"
	"github.com/cjdias/flam-in-go/pkg/flam/container"
	"github.com/cjdias/flam-in-go/pkg/flam/factory"
	"github.com/cjdias/flam-in-go/pkg/flam/internal/settings"
)

// creatorArgs are the dependencies of the creator, taken from the
// container.
type creatorArgs struct {
	dig.In

	Config flam.IConfig
}

// newCreator returns the [flam.IRestMiddlewareCreator] of the apikey
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

// Create returns the middleware of the header, the query parameter, the
// keys and the optionality of the configuration, or of their defaults,
// holding the hashes of the keys.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)

	header, ok := c.field(bag, ConfigFieldHeader, DefaultHeaderConfigPath, defaultHeader).(string)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldHeader)
	}
	query, ok := c.field(bag, ConfigFieldQuery, DefaultQueryConfigPath, defaultQuery).(string)
	if !ok || (query == "" && header == "") {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldQuery)
	}
	keys, ok := creatorKeys(c.field(bag, ConfigFieldKeys, "", nil))
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldKeys)
	}
	optional, ok := c.field(bag, ConfigFieldOptional, DefaultOptionalConfigPath, defaultOptional).(bool)
	if !ok {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, ConfigFieldOptional)
	}

	return &middleware{
		id:       id,
		header:   header,
		query:    query,
		keys:     keys,
		optional: optional,
	}, nil
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

// creatorKeys returns the keys of the value, a list of keys, of no
// principal, or a map of the name of each principal to its key, or to the
// list of its keys, and whether the value is such a list or map of at
// least one key, all non-empty and distinct.
func creatorKeys(
	value any,
) ([]key, bool) {
	set := &creatorSet{seen: map[[sha256.Size]byte]struct{}{}}
	var ok bool
	switch v := value.(type) {
	case container.Bag:
		ok = set.addPrincipals(v)
	case map[string]any:
		ok = set.addPrincipals(v)
	default:
		var secrets []string
		secrets, ok = settings.Strings(value)
		ok = ok && set.add("", secrets)
	}
	return set.keys, ok && len(set.keys) > 0
}

// creatorSet is the set of the keys read from the configuration, along
// with the hashes seen, to reject a key listed twice.
type creatorSet struct {
	keys []key
	seen map[[sha256.Size]byte]struct{}
}

// addPrincipals adds the keys of each principal of the map, its key or
// the list of its keys, reporting whether all are valid. The names are
// never empty, as the configuration paths cannot be.
func (s *creatorSet) addPrincipals(
	principals map[string]any,
) bool {
	for principal, listed := range principals {
		secrets, ok := settings.Strings(listed)
		if secret, isString := listed.(string); isString {
			secrets, ok = []string{secret}, true
		}
		if !ok || !s.add(principal, secrets) {
			return false
		}
	}
	return true
}

// add adds the keys of the principal, reporting whether all are
// non-empty and none was added before.
func (s *creatorSet) add(
	principal string,
	secrets []string,
) bool {
	for _, secret := range secrets {
		hash := sha256.Sum256([]byte(secret))
		if _, dup := s.seen[hash]; dup || secret == "" {
			return false
		}
		s.seen[hash] = struct{}{}
		s.keys = append(s.keys, key{hash: hash, principal: principal})
	}
	return true
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create apikey rest middleware"
