package service

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"git.sonicoriginal.software/grpc-testing/mocks/listener"

	"google.golang.org/grpc"
)

func TestRun(t *testing.T) {
	// Disable OTEL exporters for all tests
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	t.Setenv("OTEL_METRICS_EXPORTER", "none")
	t.Setenv("OTEL_LOGS_EXPORTER", "none")

	t.Run("returns error when serviceName is empty", func(t *testing.T) {
		lis := listener.New()
		err := Run(t.Context(), "", nil, nil, lis)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "serviceName cannot be empty") {
			t.Errorf("expected error to contain %q, got %q", "serviceName cannot be empty", err.Error())
		}
	})

	t.Run("handles nil logger", func(t *testing.T) {
		lis := listener.New()
		ctx, cancel := context.WithCancel(t.Context())

		var wg sync.WaitGroup

		wg.Go(func() {
			_ = Run(ctx, "test-service", nil, nil, lis)
		})

		// Give server time to start
		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
	})

	t.Run("uses SERVICE_VERSION from environment", func(t *testing.T) {
		t.Setenv("SERVICE_VERSION", "1.2.3")

		lis := listener.New()
		ctx, cancel := context.WithCancel(t.Context())

		var wg sync.WaitGroup

		wg.Go(func() {
			_ = Run(ctx, "test-service", slog.Default(), nil, lis)
		})

		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
	})

	t.Run("returns error when listener creation fails", func(t *testing.T) {
		// Use an invalid address - listener is nil so it will try to create one
		t.Setenv("GRPC_SERVER_ADDRESS", "invalid:address:format")

		err := Run(t.Context(), "test-service", nil, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to create listener") {
			t.Errorf("expected error to contain %q, got %q", "failed to create listener", err.Error())
		}
	})

	t.Run("returns error when OTEL initialization fails", func(t *testing.T) {
		t.Setenv("OTEL_TRACES_EXPORTER", "unsupported")

		lis := listener.New()
		err := Run(t.Context(), "test-service", nil, nil, lis)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to initialize OTEL") {
			t.Errorf("expected error to contain %q, got %q", "failed to initialize OTEL", err.Error())
		}
	})

	t.Run("returns error when serve fails", func(t *testing.T) {
		lis := listener.NewFailing(errors.New("accept error"))
		err := Run(t.Context(), "test-service", nil, nil, lis)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to serve") {
			t.Errorf("expected error to contain %q, got %q", "failed to serve", err.Error())
		}
	})

	t.Run("calls registerFn and starts grpcd client when methods exist", func(t *testing.T) {
		t.Setenv("GRPCD_ADDRESS", "")

		lis := listener.New()
		ctx, cancel := context.WithCancel(t.Context())

		registerFn := func(s grpc.ServiceRegistrar) {
			s.RegisterService(&grpc.ServiceDesc{
				ServiceName: "test.TestService",
				HandlerType: (*any)(nil),
				Methods: []grpc.MethodDesc{
					{MethodName: "TestMethod"},
				},
			}, struct{}{})
		}

		var wg sync.WaitGroup

		wg.Go(func() {
			_ = Run(ctx, "test-service", slog.Default(), registerFn, lis)
		})

		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
	})

	t.Run("graceful shutdown on context cancellation", func(t *testing.T) {
		lis := listener.New()
		ctx, cancel := context.WithCancel(t.Context())

		var wg sync.WaitGroup

		wg.Go(func() {
			_ = Run(ctx, "test-service", slog.Default(), nil, lis)
		})

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Cancel to trigger graceful shutdown
		cancel()

		// Wait for shutdown with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for graceful shutdown")
		}
	})

	t.Run("graceful shutdown on SIGTERM", func(t *testing.T) {
		lis := listener.New()

		var wg sync.WaitGroup

		wg.Go(func() {
			_ = Run(t.Context(), "test-service", slog.Default(), nil, lis)
		})

		// Wait for server to start
		time.Sleep(100 * time.Millisecond)

		// Send SIGTERM to ourselves
		proc, err := os.FindProcess(os.Getpid())
		if err != nil {
			t.Fatalf("unexpected error finding process: %v", err)
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			t.Fatalf("unexpected error sending signal: %v", err)
		}

		// Wait for shutdown with timeout
		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for graceful shutdown")
		}
	})

	t.Run("accepts custom server options", func(t *testing.T) {
		lis := listener.New()
		ctx, cancel := context.WithCancel(t.Context())

		customOpt := grpc.MaxRecvMsgSize(1024)

		var wg sync.WaitGroup

		wg.Go(func() {
			_ = Run(ctx, "test-service", nil, nil, lis, customOpt)
		})

		time.Sleep(50 * time.Millisecond)
		cancel()
		wg.Wait()
	})
}
