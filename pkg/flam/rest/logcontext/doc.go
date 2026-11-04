// Package logcontext adds the request scoped fields of the REST feature to
// the log entries given with the context of a request: the bundle
// returned from [NewBundle] provides a
// [github.com/cjdias/flam-in-go/pkg/flam.ILogContextExtractor] in the
// [github.com/cjdias/flam-in-go/pkg/flam/log.ContextExtractorGroup]
// container group, extracting the id of the request, read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.RequestIdOf], and its
// principal, read by
// [github.com/cjdias/flam-in-go/pkg/flam/rest.PrincipalOf], at the fields
// read from [RequestIdFieldConfigPath] and [PrincipalFieldConfigPath].
//
// Register its bundle along with the log bundle, so that the entries the
// handlers and the middlewares give to the
// [github.com/cjdias/flam-in-go/pkg/flam.IContextLogger] with the context
// of their request, such as those of the log middleware of
// [github.com/cjdias/flam-in-go/pkg/flam/rest/middlewares/log], hold
// them:
//
//	app.RegisterBundle(log.NewBundle())
//	app.RegisterBundle(logcontext.NewBundle())
//
// It lives apart from the REST bundle, so that the log package never
// imports the REST one.
package logcontext
