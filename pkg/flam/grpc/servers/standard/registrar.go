package standard

import (
	"reflect"

	"google.golang.org/grpc"
)

// registrar is the [grpc.ServiceRegistrar] given to the service registers
// of a server, registering their services on its [grpc.Server] once
// checked, as the server ends the process on an invalid one. Once a
// service is invalid, it registers no other, and the creator fails.
type registrar struct {
	server  *grpc.Server
	invalid bool
	service string
}

// compile time check that *registrar implements ServiceRegistrar
var _ grpc.ServiceRegistrar = (*registrar)(nil)

// RegisterService registers the service on the server, unless it is
// registered already, or its implementation, when not nil, is not of its
// handler type, the service being then recorded as invalid.
func (r *registrar) RegisterService(
	desc *grpc.ServiceDesc,
	impl any,
) {
	if r.invalid {
		return
	}
	if _, ok := r.server.GetServiceInfo()[desc.ServiceName]; ok || !registrarImplements(desc, impl) {
		r.invalid = true
		r.service = desc.ServiceName
		return
	}
	r.server.RegisterService(desc, impl)
}

// GetServiceInfo returns the services registered on the server, for the
// reflection service.
func (r *registrar) GetServiceInfo() map[string]grpc.ServiceInfo {
	return r.server.GetServiceInfo()
}

// registrarImplements reports whether the implementation is nil, or of
// the handler type of the service, a pointer to an interface.
func registrarImplements(
	desc *grpc.ServiceDesc,
	impl any,
) bool {
	if impl == nil {
		return true
	}
	handlerType := reflect.TypeOf(desc.HandlerType)
	return handlerType != nil &&
		handlerType.Kind() == reflect.Pointer &&
		handlerType.Elem().Kind() == reflect.Interface &&
		reflect.TypeOf(impl).Implements(handlerType.Elem())
}
