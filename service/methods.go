//revive:disable:package-comments
package service

// infrastructurePrefixes name the services a caller never advertises to grpcd
// and never reports a health status for. They are up exactly when the process
// is, which the health service's own "" entry already reports.
var infrastructurePrefixes = []string{
	"grpc.",        // gRPC infrastructure (health, reflection)
	"info.",        // Cumulus internal info endpoint
	"diagnostics.", // Cumulus internal diagnostics endpoint
}
