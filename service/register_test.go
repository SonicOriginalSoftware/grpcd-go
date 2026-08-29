package service

import (
	"context"
	"slices"
	"strings"
	"testing"

	"google.golang.org/grpc/health/grpc_health_v1"

	diagpb "git.sonicoriginal.software/grpc-protos/diagnostics"

	"git.sonicoriginal.software/grpcd-go/diagnostics"
)

// noCheck satisfies diagnostics.Check without inspecting anything.
func noCheck(_ context.Context) (*diagpb.ServiceDependency, error) {
	return nil, nil
}

func TestRegister(t *testing.T) {
	t.Run("returns an error when srv is nil", func(t *testing.T) {
		_, err := Register(nil, newHealthStub(), nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "srv cannot be nil") {
			t.Errorf("error = %q, want it to mention a nil srv", err)
		}
	})

	t.Run("returns an error when healthSrv is nil", func(t *testing.T) {
		_, err := Register(newServerStub(), nil, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "healthSrv cannot be nil") {
			t.Errorf("error = %q, want it to mention a nil healthSrv", err)
		}
	})

	t.Run("returns an error when checks already reserve the grpcd name", func(t *testing.T) {
		checks := diagnostics.Checks{grpcdCheckName: noCheck}

		_, err := Register(newServerStub(), newHealthStub(), checks, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), grpcdCheckName) {
			t.Errorf("error = %q, want it to name %q", err, grpcdCheckName)
		}
	})

	t.Run("registers the caller's services", func(t *testing.T) {
		srv := newServerStub()

		_, err := Register(srv, newHealthStub(), nil, registerExampleService)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !srv.registered("example.ExampleService") {
			t.Errorf("registered = %v, want the caller's service", srv.order)
		}
	})

	t.Run("registers diagnostics, info, health, and reflection", func(t *testing.T) {
		srv := newServerStub()

		if _, err := Register(srv, newHealthStub(), nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{
			"diagnostics.DiagnosticsService",
			"info.InfoService",
			"grpc.health.v1.Health",
			"grpc.reflection.v1.ServerReflection",
		}
		for _, name := range want {
			if !srv.registered(name) {
				t.Errorf("%q was not registered; got %v", name, srv.order)
			}
		}
	})

	t.Run("adds the grpcd check to the caller's checks", func(t *testing.T) {
		checks := diagnostics.Checks{}

		if _, err := Register(newServerStub(), newHealthStub(), checks, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, exists := checks[grpcdCheckName]; !exists {
			t.Errorf("checks = %v, want a %q entry", checks, grpcdCheckName)
		}
	})

	t.Run("accepts a nil checks map", func(t *testing.T) {
		srv := newServerStub()

		if _, err := Register(srv, newHealthStub(), nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !srv.registered("diagnostics.DiagnosticsService") {
			t.Errorf("registered = %v, want the diagnostics service", srv.order)
		}
	})

	t.Run("accepts a nil registerFn", func(t *testing.T) {
		srv := newServerStub()

		if _, err := Register(srv, newHealthStub(), nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if srv.registered("example.ExampleService") {
			t.Error("no caller service should be registered")
		}
	})

	t.Run("returns the caller's methods and excludes infrastructure", func(t *testing.T) {
		got, err := Register(newServerStub(), newHealthStub(), nil, registerExampleService)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := []string{
			"/example.ExampleService/Create",
			"/example.ExampleService/Delete",
		}
		if !slices.Equal(got, want) {
			t.Errorf("methods = %v, want %v", got, want)
		}
	})

	t.Run("marks the caller's services serving, once each", func(t *testing.T) {
		healthSrv := newHealthStub()

		if _, err := Register(newServerStub(), healthSrv, nil, registerExampleService); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := map[string]grpc_health_v1.HealthCheckResponse_ServingStatus{
			"example.ExampleService": grpc_health_v1.HealthCheckResponse_SERVING,
		}
		if len(healthSrv.statuses) != len(want) {
			t.Fatalf("statuses = %v, want %v", healthSrv.statuses, want)
		}
		for name, status := range want {
			if healthSrv.statuses[name] != status {
				t.Errorf("%q status = %v, want %v", name, healthSrv.statuses[name], status)
			}
		}
	})

	t.Run("reports no serving status for infrastructure services", func(t *testing.T) {
		healthSrv := newHealthStub()

		if _, err := Register(newServerStub(), healthSrv, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(healthSrv.statuses) != 0 {
			t.Errorf("statuses = %v, want none", healthSrv.statuses)
		}
	})
}
