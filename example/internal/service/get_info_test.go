package service

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"git.sonicoriginal.software/grpc-protos/info"
	"git.sonicoriginal.software/grpc-testing/mocks/meter"
)

func TestGetInfo_Success(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	server := NewServer(log, m)

	// Set SERVICE_VERSION env var
	os.Setenv("SERVICE_VERSION", "1.0.0")
	defer os.Unsetenv("SERVICE_VERSION")

	resp, err := server.GetInfo(t.Context(), &info.GetInfoRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Info.Version != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %s", resp.Info.Version)
	}

	if resp.Info.Details == nil {
		t.Error("expected details map to be initialized")
	}
}

func TestGetInfo_MissingVersion(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	server := NewServer(log, m)

	// Ensure SERVICE_VERSION is not set
	os.Unsetenv("SERVICE_VERSION")

	resp, err := server.GetInfo(t.Context(), &info.GetInfoRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Info.Version != "" {
		t.Errorf("expected version '', got %s", resp.Info.Version)
	}
}
