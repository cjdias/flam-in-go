package openapi

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.openapi"

// Version is the version of OpenAPI of the documents returned by
// [NewDocument].
const Version = "3.1.0"

// ProblemName is the name of the schema, and of the response, of the
// components of a [Document] describing the RFC 9457 problems the errors
// are written as.
const ProblemName = "Problem"

// Configuration paths of the endpoints, read when the register is
// created.
const (
	// GroupConfigPath is the configuration path of the group of the
	// servers serving the endpoints, "api" unless set.
	GroupConfigPath = "flam.rest.openapi.group"

	// PathConfigPath is the configuration path of the path of the endpoint
	// of the document, served to the GET requests, "/openapi.json" unless
	// set.
	PathConfigPath = "flam.rest.openapi.path"

	// IdConfigPath is the configuration path of the id of the endpoint of
	// the document, a whole number, 0 unless set.
	IdConfigPath = "flam.rest.openapi.id"

	// TitleConfigPath is the configuration path of the title of the API,
	// "API" unless set.
	TitleConfigPath = "flam.rest.openapi.title"

	// VersionConfigPath is the configuration path of the version of the
	// API, "1.0.0" unless set.
	VersionConfigPath = "flam.rest.openapi.version"

	// DescriptionConfigPath is the configuration path of the description
	// of the API, none unless set.
	DescriptionConfigPath = "flam.rest.openapi.description"

	// ServersConfigPath is the configuration path of the list of the URLs
	// of the servers of the API, such as "https://api.example.com", none
	// unless set.
	ServersConfigPath = "flam.rest.openapi.servers"

	// UIConfigPath is the configuration path of whether the endpoint of
	// the Swagger UI page is served, false unless set.
	UIConfigPath = "flam.rest.openapi.ui.enabled"

	// UIPathConfigPath is the configuration path of the path of the
	// endpoint of the Swagger UI page, served to the GET requests, "/docs"
	// unless set.
	UIPathConfigPath = "flam.rest.openapi.ui.path"

	// UIIdConfigPath is the configuration path of the id of the endpoint
	// of the Swagger UI page, a whole number, 0 unless set.
	UIIdConfigPath = "flam.rest.openapi.ui.id"

	// UICDNConfigPath is the configuration path of the base URL of the
	// swagger-ui-dist files the Swagger UI page loads, the version 5.17.14
	// on the jsDelivr CDN unless set.
	UICDNConfigPath = "flam.rest.openapi.ui.cdn"
)

// Defaults of the configuration of the endpoints.
const (
	defaultGroup   = "api"
	defaultPath    = "/openapi.json"
	defaultId      = 0
	defaultTitle   = "API"
	defaultVersion = "1.0.0"
	defaultUIPath  = "/docs"
	defaultUIId    = 0
	defaultUICDN   = "https://cdn.jsdelivr.net/npm/swagger-ui-dist@5.17.14"
)
