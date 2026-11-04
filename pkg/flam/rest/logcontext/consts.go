package logcontext

// bundleId is the id of the bundle returned by [NewBundle].
const bundleId = "flam.rest.logcontext"

// Configuration paths of the fields of the extractor.
const (
	// RequestIdFieldConfigPath is the configuration path of the context
	// field the extractor stores the id of the request at. It defaults to
	// [ContextFieldRequestId], and an empty field leaves the id out. It is
	// read when the extractor is created, with the logger.
	RequestIdFieldConfigPath = "flam.rest.logcontext.request_id"

	// PrincipalFieldConfigPath is the configuration path of the context
	// field the extractor stores the principal of the request at. It
	// defaults to [ContextFieldPrincipal], and an empty field leaves the
	// principal out. It is read when the extractor is created, with the
	// logger.
	PrincipalFieldConfigPath = "flam.rest.logcontext.principal"
)

// Default context fields of the extractor.
const (
	// ContextFieldRequestId is the default field of the id of the request.
	ContextFieldRequestId = "request_id"

	// ContextFieldPrincipal is the default field of the principal of the
	// request.
	ContextFieldPrincipal = "principal"
)
