package service

import (
	"fmt"
	"io"
	"log/slog"
	"testing"

	"git.sonicoriginal.software/grpc-testing/mocks/meter"
)

func TestNewServer(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	server := NewServer(log, m)

	if server == nil {
		t.Fatal("expected server, got nil")
	}
	if server.log == nil {
		t.Fatal("expected logger to be set")
	}
}

func TestNewServer_NilLogger(t *testing.T) {
	m := meter.New()
	server := NewServer(nil, m)

	if server == nil {
		t.Fatal("expected server, got nil")
	}
	if server.log == nil {
		t.Fatal("expected logger to be set even when nil passed")
	}
}

func TestNewServer_MetricCreationError(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	m.SetInt64CounterError(fmt.Errorf("metric creation failed"))

	server := NewServer(log, m)

	if server == nil {
		t.Fatal("expected server even when metrics fail, got nil")
	}
	if server.log == nil {
		t.Fatal("expected logger to be set")
	}
}
