package service

import (
	"slices"
	"testing"
)

func TestMethods(t *testing.T) {
	t.Run("reports the caller's methods after Register", func(t *testing.T) {
		srv := newServerStub()

		if err := Register(srv, serviceName, nil, registerExampleService); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := Methods(srv)
		want := []string{"/example.ExampleService/Create"}

		if !slices.Equal(got, want) {
			t.Errorf("methods = %v, want %v", got, want)
		}
	})

	t.Run("excludes the infrastructure services Register adds", func(t *testing.T) {
		srv := newServerStub()

		if err := Register(srv, serviceName, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := Methods(srv); len(got) != 0 {
			t.Errorf("methods = %v, want none", got)
		}
	})

	t.Run("reports nothing for a server with no services", func(t *testing.T) {
		if got := Methods(newServerStub()); len(got) != 0 {
			t.Errorf("methods = %v, want none", got)
		}
	})
}
