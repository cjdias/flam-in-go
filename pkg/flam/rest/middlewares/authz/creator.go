package authz

import (
	"path"
	"strings"

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

// newCreator returns the [flam.IRestMiddlewareCreator] of the authz
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

// Create returns the middleware of the policy, the rules, the roles claim,
// the principals and the challenge of the configuration, or of their
// defaults.
func (c *creator) Create(
	bag flam.IBag,
) (flam.IRestMiddleware, error) {
	id := bag.String(factory.ConfigFieldId)
	fail := func(field string) (flam.IRestMiddleware, error) {
		return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field)
	}

	policy, _ := c.field(bag, ConfigFieldPolicy, DefaultPolicyConfigPath, defaultPolicy).(string)
	if policy != PolicyDeny && policy != PolicyAuthenticated {
		return fail(ConfigFieldPolicy)
	}
	value := c.field(bag, ConfigFieldRules, "", nil)
	list, ok := value.([]any)
	if !ok && value != nil {
		return fail(ConfigFieldRules)
	}
	rules := make([]rule, len(list))
	for i, item := range list {
		var field string
		if rules[i], field = creatorRule(item); field != "" {
			return nil, newErrCreate(ErrInvalidField, creatorCreateMessage, id, field).
				With(ErrorFieldRule, i)
		}
	}
	claim, ok := c.field(bag, ConfigFieldRolesClaim, DefaultRolesClaimConfigPath, defaultRolesClaim).(string)
	var claimPath []string
	if claim != "" {
		claimPath = strings.Split(claim, ".")
	}
	if !ok || creatorHasEmpty(claimPath) {
		return fail(ConfigFieldRolesClaim)
	}
	principals, ok := creatorPrincipals(c.field(bag, ConfigFieldPrincipals, "", nil))
	if !ok {
		return fail(ConfigFieldPrincipals)
	}
	challenge, ok := c.field(bag, ConfigFieldChallenge, DefaultChallengeConfigPath, defaultChallenge).(string)
	if !ok {
		return fail(ConfigFieldChallenge)
	}

	return &middleware{
		id:         id,
		deny:       policy == PolicyDeny,
		rules:      rules,
		rolesClaim: claimPath,
		principals: principals,
		challenge:  challenge,
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

// creatorRule returns the rule of the item, a map of its fields, along
// with the name of its first invalid field, or "rules" when the item is
// not a map, empty when the rule is valid: matching at least one endpoint
// or pattern, all well-formed, and requiring non-empty scopes and roles,
// or being public without any.
func creatorRule(
	item any,
) (rule, string) {
	fields, ok := creatorMap(item)
	if !ok {
		return rule{}, ConfigFieldRules
	}
	var r rule
	if r.endpoints, ok = creatorInts(fields[RuleFieldEndpoints]); !ok {
		return rule{}, RuleFieldEndpoints
	}
	r.patterns, ok = settings.Strings(fields[RuleFieldPatterns])
	if !ok || creatorHasEmpty(r.patterns) || !creatorPatterns(r.patterns) {
		return rule{}, RuleFieldPatterns
	}
	if len(r.endpoints) == 0 && len(r.patterns) == 0 {
		return rule{}, RuleFieldEndpoints
	}
	if r.scopes, ok = settings.Strings(fields[RuleFieldScopes]); !ok || creatorHasEmpty(r.scopes) {
		return rule{}, RuleFieldScopes
	}
	if r.roles, ok = settings.Strings(fields[RuleFieldRoles]); !ok || creatorHasEmpty(r.roles) {
		return rule{}, RuleFieldRoles
	}
	public, isBool := fields[RuleFieldPublic].(bool)
	if (fields[RuleFieldPublic] != nil && !isBool) || (public && len(r.scopes)+len(r.roles) > 0) {
		return rule{}, RuleFieldPublic
	}
	r.public = public
	return r, ""
}

// creatorPrincipals returns the roles of each principal of the value, a
// map of the name of each principal to its role, or to the list of its
// roles, nil being an empty map, and whether the value is such a map of
// non-empty roles.
func creatorPrincipals(
	value any,
) (map[string][]string, bool) {
	if value == nil {
		return nil, true
	}
	principals, ok := creatorMap(value)
	if !ok {
		return nil, false
	}
	roles := make(map[string][]string, len(principals))
	for principal, listed := range principals {
		list, isList := settings.Strings(listed)
		if role, isString := listed.(string); isString {
			list, isList = []string{role}, true
		}
		if !isList || creatorHasEmpty(list) {
			return nil, false
		}
		roles[principal] = list
	}
	return roles, true
}

// creatorMap returns the map of the value, a [container.Bag] or a
// map[string]any, and whether it is one of them.
func creatorMap(
	value any,
) (map[string]any, bool) {
	switch v := value.(type) {
	case container.Bag:
		return v, true
	case map[string]any:
		return v, true
	}
	return nil, false
}

// creatorInts returns the ints of the list, a []int or a []any of ints,
// nil being an empty list, and whether the value is one.
func creatorInts(
	value any,
) ([]int, bool) {
	switch list := value.(type) {
	case nil:
		return nil, true
	case []int:
		return list, true
	case []any:
		items := make([]int, len(list))
		for i, item := range list {
			n, ok := item.(int)
			if !ok {
				return nil, false
			}
			items[i] = n
		}
		return items, true
	}
	return nil, false
}

// creatorPatterns reports whether every pattern is well-formed, as
// [path.Match] reads it.
func creatorPatterns(
	patterns []string,
) bool {
	for _, pattern := range patterns {
		if _, err := path.Match(pattern, ""); err != nil {
			return false
		}
	}
	return true
}

// creatorHasEmpty reports whether one of the values is empty.
func creatorHasEmpty(
	values []string,
) bool {
	for _, value := range values {
		if value == "" {
			return true
		}
	}
	return false
}

// creatorCreateMessage is the message of the errors returned by Create.
const creatorCreateMessage = "unable to create authz rest middleware"
