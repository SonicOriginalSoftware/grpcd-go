//revive:disable:package-comments
package methods

import "strings"

// ServiceName reports the fully qualified service owning a method named in
// gRPC wire format, "/package.Service/Method". A name without the leading
// slash is read the same way.
func ServiceName(method string) string {
	name, _, _ := strings.Cut(strings.TrimPrefix(method, "/"), "/")

	return name
}

// ServiceNames reports the distinct services owning the given fully qualified
// method names, in the order the methods appear.
func ServiceNames(methods []string) []string {
	seen := map[string]struct{}{}
	names := []string{}

	for _, method := range methods {
		name := ServiceName(method)
		if _, found := seen[name]; found {
			continue
		}

		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names
}
