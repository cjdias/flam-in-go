package authz

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.middlewares.authz"

const (
	// Id is the id of the middleware configured by the bundle returned
	// from [NewBundle].
	Id = "authz"

	// Driver is the driver name of the authz middlewares, held by the
	// [github.com/cjdias/flam-in-go/pkg/flam/factory.ConfigFieldDriver]
	// field of their configuration.
	Driver = "flam.rest.middlewares.driver.authz"
)

// Fields of an authz middleware configuration.
const (
	// ConfigFieldPolicy is the field holding the policy of the endpoints
	// no rule matches, [PolicyDeny] or [PolicyAuthenticated].
	ConfigFieldPolicy = "policy"

	// ConfigFieldRules is the field holding the list of the rules, each a
	// map of the [RuleFieldEndpoints] or the [RuleFieldPatterns] it
	// matches, and of its [RuleFieldScopes], [RuleFieldRoles] or
	// [RuleFieldPublic] requirements, the first rule matching an endpoint
	// being the one applied to it.
	ConfigFieldRules = "rules"

	// ConfigFieldRolesClaim is the field holding the path of the claim of
	// the roles of the tokens, its segments split on the dots, such as
	// "roles" or "realm_access.roles", a string, empty for none.
	ConfigFieldRolesClaim = "roles_claim"

	// ConfigFieldPrincipals is the field holding the map of the name of
	// each principal, such as one of the apikey middlewares, to its role,
	// or to the list of its roles, all non-empty.
	ConfigFieldPrincipals = "principals"

	// ConfigFieldChallenge is the field holding the value of the
	// WWW-Authenticate header of the 401 answers, a string, empty for no
	// header.
	ConfigFieldChallenge = "challenge"
)

// Fields of a rule of [ConfigFieldRules].
const (
	// RuleFieldEndpoints is the field holding the list of the int ids of
	// the endpoints the rule matches, whatever their service.
	RuleFieldEndpoints = "endpoints"

	// RuleFieldPatterns is the field holding the list of the patterns of
	// the endpoints the rule matches, such as "GET /admin/*", matched
	// against the pattern of each endpoint, such as
	// "GET /admin/{id}", as [path.Match] does, a "*" not matching a "/".
	RuleFieldPatterns = "patterns"

	// RuleFieldScopes is the field holding the list of the scopes the
	// token of a request must all hold, in its "scope" claim, a space
	// separated string, or its "scp" claim, a list or a space separated
	// string.
	RuleFieldScopes = "scopes"

	// RuleFieldRoles is the field holding the list of the roles the
	// request must hold one of, in the claim of [ConfigFieldRolesClaim],
	// a list or a space separated string, or among the roles of its
	// principal in [ConfigFieldPrincipals].
	RuleFieldRoles = "roles"

	// RuleFieldPublic is the field holding the bool telling whether the
	// endpoints of the rule serve the requests without authentication,
	// then holding no scope or role.
	RuleFieldPublic = "public"
)

// Policies of [ConfigFieldPolicy].
const (
	// PolicyDeny answers with a 403 every request of the endpoints no rule
	// matches.
	PolicyDeny = "deny"

	// PolicyAuthenticated serves every authenticated request of the
	// endpoints no rule matches.
	PolicyAuthenticated = "authenticated"
)

// Configuration paths of the values taken by the fields missing from a
// middleware configuration. The rules and the principals have none, as
// they belong to each middleware.
const (
	// DefaultPolicyConfigPath is the path of the default of
	// [ConfigFieldPolicy], [PolicyDeny] unless changed.
	DefaultPolicyConfigPath = "flam.rest.defaults.middlewares.authz.policy"

	// DefaultRolesClaimConfigPath is the path of the default of
	// [ConfigFieldRolesClaim], "roles" unless changed.
	DefaultRolesClaimConfigPath = "flam.rest.defaults.middlewares.authz.roles_claim"

	// DefaultChallengeConfigPath is the path of the default of
	// [ConfigFieldChallenge], "Bearer" unless changed.
	DefaultChallengeConfigPath = "flam.rest.defaults.middlewares.authz.challenge"
)

// Context fields of the [*flam.Error] values returned by the creator.
const (
	// ErrorFieldID holds the id of the middleware.
	ErrorFieldID = "id"

	// ErrorFieldField holds the name of an invalid configuration field.
	ErrorFieldField = "field"

	// ErrorFieldRule holds the index of an invalid rule, in the list of
	// [ConfigFieldRules], along with the name of its invalid field in
	// ErrorFieldField.
	ErrorFieldRule = "rule"
)

// Defaults of the fields of a middleware configuration.
const (
	defaultPolicy     = PolicyDeny
	defaultRolesClaim = "roles"
	defaultChallenge  = "Bearer"
)
