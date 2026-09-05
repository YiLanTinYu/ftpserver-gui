//go:build localfs

package fs

import (
	"log/slog"
	"testing"

	"github.com/fclairamb/ftpserver/config/confpar"
)

func TestLocalBuildLoadsOSFilesystem(t *testing.T) {
	loaded, err := LoadFs(&confpar.Access{Fs: "os", Params: map[string]string{"basePath": t.TempDir()}}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	if loaded == nil {
		t.Fatal("LoadFs returned a nil filesystem")
	}
}

func TestLocalBuildRejectsCloudBackend(t *testing.T) {
	_, err := LoadFs(&confpar.Access{Fs: "s3"}, slog.Default())
	if err == nil {
		t.Fatal("local-only build unexpectedly accepted the s3 backend")
	}
}
