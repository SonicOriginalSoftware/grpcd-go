//revive:disable:package-comments
package service

import (
	"strings"
)

// infrastructurePrefixes name the services a caller never advertises to grpcd
// and never reports a health status for. They are up exactly when the process
// is, which the health service's own "" entry already reports.
var infrastructurePrefixes = []string{
	"grpc.",        // gRPC infrastructure (health, reflection)
	"info.",        // Cumulus internal info endpoint
	"diagnostics.", // Cumulus internal diagnostics endpoint
}

// serviceNames reports the distinct services owning the given fully qualified
// method names, in the order the methods appear.
func serviceNames(methodNames []string) []string {
	seen := map[string]struct{}{}
	names := []string{}

	for _, method := range methodNames {
		name, _, _ := strings.Cut(strings.TrimPrefix(method, "/"), "/")
		if _, found := seen[name]; found {
			continue
		}

		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}
