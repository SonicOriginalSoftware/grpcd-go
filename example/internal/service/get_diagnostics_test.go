package service

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"git.sonicoriginal.software/grpcd-go/client"

	"git.sonicoriginal.software/grpc-protos/diagnostics"
	"git.sonicoriginal.software/grpc-testing/mocks/meter"
)

func TestGetDiagnostics_NotConfigured(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	server := NewServer(log, m)

	// Ensure grpcd address is not set
	os.Unsetenv(client.GRPCDAddressKey)

	resp, err := server.GetDiagnostics(t.Context(), &diagnostics.GetDiagnosticsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Services == nil {
		t.Fatal("expected services map to be set")
	}

	grpcd, ok := resp.Services["grpcd"]
	if !ok {
		t.Fatal("expected grpcd service in response")
	}

	if grpcd.State != "" {
		t.Errorf("expected '' state, got %v", grpcd.State)
	}

	if grpcd.Address != "" {
		t.Errorf("expected empty address, got %s", grpcd.Address)
	}
}

func TestGetDiagnostics_WithAddress(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	server := NewServer(log, m)

	// Set a grpcd address (will likely fail to connect in test)
	os.Setenv(client.GRPCDAddressKey, "localhost:50060")
	defer os.Unsetenv(client.GRPCDAddressKey)

	resp, err := server.GetDiagnostics(t.Context(), &diagnostics.GetDiagnosticsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Services == nil {
		t.Fatal("expected services map to be set")
	}

	grpcd, ok := resp.Services["grpcd"]
	if !ok {
		t.Fatal("expected grpcd service in response")
	}

	if grpcd.Address != "localhost:50060" {
		t.Errorf("expected address localhost:50060, got %s", grpcd.Address)
	}

	// In test environment, connection will likely fail
	// Just verify we got a response with proper structure
	if grpcd.LastChecked == 0 {
		t.Error("expected last_checked to be set")
	}
}
