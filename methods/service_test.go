package methods

import (
	"slices"
	"testing"
)

func TestServiceName(t *testing.T) {
	cases := map[string]struct {
		method string
		want   string
	}{
		"qualified":        {"/package.Service/Method", "package.Service"},
		"unqualified":      {"/Service/Method", "Service"},
		"no leading slash": {"package.Service/Method", "package.Service"},
		"no method":        {"/package.Service", "package.Service"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := ServiceName(tc.method); got != tc.want {
				t.Errorf("ServiceName(%q) = %q, want %q", tc.method, got, tc.want)
			}
		})
	}
}

func TestServiceNames(t *testing.T) {
	t.Run("reports each service once in first-seen order", func(t *testing.T) {
		got := ServiceNames([]string{
			"/b.Service/One",
			"/a.Service/One",
			"/b.Service/Two",
		})

		want := []string{"b.Service", "a.Service"}
		if !slices.Equal(got, want) {
			t.Errorf("ServiceNames = %v, want %v", got, want)
		}
	})

	t.Run("reports nothing for no methods", func(t *testing.T) {
		if got := ServiceNames(nil); len(got) != 0 {
			t.Errorf("ServiceNames(nil) = %v, want empty", got)
		}
	})
}
