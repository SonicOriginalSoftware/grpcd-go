//revive:disable:package-comments
package service

import (
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Health is the health service the caller builds and owns. Register attaches
// it and marks the caller's own services SERVING; every status change after
// that is the caller's, since only the application knows what a degraded
// dependency means for it.
//
// *health.Server satisfies this. A caller keeps the handle to flip a service
// to NOT_SERVING, or calls its Shutdown to drain before stopping.
type Health interface {
	grpc_health_v1.HealthServer

	SetServingStatus(service string, status grpc_health_v1.HealthCheckResponse_ServingStatus)
}
