package grpc

import (
	"strings"

	"google.golang.org/grpc/codes"
)

// SplitMethod returns the service and the method of the full method of a
// call, as given to the interceptors, such as "pkg.Users" and "Get" for
// "/pkg.Users/Get", the service being empty when the full method holds no
// service.
func SplitMethod(
	fullMethod string,
) (string, string) {
	name := strings.TrimPrefix(fullMethod, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		return name[:i], name[i+1:]
	}
	return "", name
}

// IsServerError reports whether the code of a call tells a failure of the
// server rather than of the call, as the OpenTelemetry semantic
// conventions of gRPC tell: [codes.Unknown], [codes.DeadlineExceeded],
// [codes.Unimplemented], [codes.Internal], [codes.Unavailable] and
// [codes.DataLoss].
func IsServerError(
	code codes.Code,
) bool {
	switch code {
	case codes.Unknown, codes.DeadlineExceeded, codes.Unimplemented,
		codes.Internal, codes.Unavailable, codes.DataLoss:
		return true
	}
	return false
}
