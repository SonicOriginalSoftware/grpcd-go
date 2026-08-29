package service

import (
	"google.golang.org/grpc"
)

// serverStub records what was registered on it and reports it back the way a
// real server does, so Methods sees the result of an actual Register call
// rather than a canned map.
type serverStub struct {
	services map[string]grpc.ServiceInfo
	order    []string
}

func newServerStub() *serverStub {
	return &serverStub{services: map[string]grpc.ServiceInfo{}}
}

func (s *serverStub) RegisterService(desc *grpc.ServiceDesc, _ any) {
	s.order = append(s.order, desc.ServiceName)
	s.services[desc.ServiceName] = grpc.ServiceInfo{Methods: methodInfo(desc)}
}

func (s *serverStub) GetServiceInfo() map[string]grpc.ServiceInfo {
	return s.services
}

// registered reports whether a service of that name was registered.
func (s *serverStub) registered(serviceName string) bool {
	_, exists := s.services[serviceName]

	return exists
}

// methodInfo flattens a service description the way grpc.Server does, folding
// streams in alongside unary methods.
func methodInfo(desc *grpc.ServiceDesc) []grpc.MethodInfo {
	info := make([]grpc.MethodInfo, 0, len(desc.Methods)+len(desc.Streams))

	for _, method := range desc.Methods {
		info = append(info, grpc.MethodInfo{Name: method.MethodName})
	}
	for _, stream := range desc.Streams {
		info = append(info, grpc.MethodInfo{
			Name:           stream.StreamName,
			IsClientStream: stream.ClientStreams,
			IsServerStream: stream.ServerStreams,
		})
	}

	return info
}

// registerExampleService stands in for a caller's registerFn.
func registerExampleService(s grpc.ServiceRegistrar) {
	s.RegisterService(&grpc.ServiceDesc{
		ServiceName: "example.ExampleService",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{MethodName: "Create"},
		},
	}, struct{}{})
}
