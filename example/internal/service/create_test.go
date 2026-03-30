package service

import (
	"io"
	"log/slog"
	"testing"

	"git.sonicoriginal.software/grpcd-go/example/internal/station"

	"git.sonicoriginal.software/grpc-testing/mocks/meter"
	"git.sonicoriginal.software/logger"
)

func TestCreate_Success(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m := meter.New()
	server := NewServer(log, m)
	ctx := logger.ContextWithLogger(t.Context(), log)

	req := &station.CreateRequest{}

	resp, err := server.Create(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected , got nil")
	}
}
