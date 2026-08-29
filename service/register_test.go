package service

import (
	"context"
	"strings"
	"testing"

	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"

	"git.sonicoriginal.software/grpcd-go/diagnostics"
)

const serviceName = "example-service"

// noCheck satisfies diagnostics.Check without inspecting anything.
func noCheck(_ context.Context) (*diagpb.ServiceDependency, error) {
	return nil, nil
}

func TestRegister(t *testing.T) {
	t.Run("returns an error when srv is nil", func(t *testing.T) {
		err := Register(nil, serviceName, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "srv cannot be nil") {
			t.Errorf("error = %q, want it to mention a nil srv", err)
		}
	})

	t.Run("returns an error when serviceName is empty", func(t *testing.T) {
		err := Register(newServerStub(), "", nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "serviceName cannot be empty") {
			t.Errorf("error = %q, want it to mention an empty serviceName", err)
		}
	})

	t.Run("returns an error when checks already reserve the grpcd name", func(t *testing.T) {
		checks := diagnostics.Checks{grpcdCheckName: noCheck}

		err := Register(newServerStub(), serviceName, checks, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), grpcdCheckName) {
			t.Errorf("error = %q, want it to name %q", err, grpcdCheckName)
		}
	})

	t.Run("registers the caller's services", func(t *testing.T) {
		srv := newServerStub()

		if err := Register(srv, serviceName, nil, registerExampleService); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !srv.registered("example.ExampleService") {
			t.Errorf("registered = %v, want the caller's service", srv.order)
		}
	})

	t.Run("registers diagnostics, health, and reflection", func(t *testing.T) {
		srv := newServerStub()

		if err := Register(srv, serviceName, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{
			"diagnostics.DiagnosticsService",
			"grpc.health.v1.Health",
			"grpc.reflection.v1.ServerReflection",
		}
		for _, serviceName := range want {
			if !srv.registered(serviceName) {
				t.Errorf("%q was not registered; got %v", serviceName, srv.order)
			}
		}
	})

	t.Run("adds the grpcd check to the caller's checks", func(t *testing.T) {
		checks := diagnostics.Checks{}

		if err := Register(newServerStub(), serviceName, checks, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := checks[grpcdCheckName]; !exists {
			t.Errorf("checks = %v, want a %q entry", checks, grpcdCheckName)
		}
	})

	t.Run("accepts a nil checks map", func(t *testing.T) {
		srv := newServerStub()

		if err := Register(srv, serviceName, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !srv.registered("diagnostics.DiagnosticsService") {
			t.Errorf("registered = %v, want the diagnostics service", srv.order)
		}
	})

	t.Run("accepts a nil registerFn", func(t *testing.T) {
		srv := newServerStub()

		if err := Register(srv, serviceName, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if srv.registered("example.ExampleService") {
			t.Error("no caller service should be registered")
		}
	})
}
