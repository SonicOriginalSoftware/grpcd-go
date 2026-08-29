package service

import (
	"testing"

	infopb "git.sonicoriginal.software/grpc-protos/info"
)

func TestInfoServerVersion(t *testing.T) {
	t.Run("reports the version it was built with", func(t *testing.T) {
		want := "1.2.3"
		srv := &infoServer{version: want}

		response, err := srv.Version(t.Context(), &infopb.VersionRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if response.Version != want {
			t.Errorf("version = %q, want %q", response.Version, want)
		}
	})
}
