//go:build localfs

// Package fs provides the local-disk-only filesystem loader used by the
// lightweight Windows GUI build.
package fs

import (
	"fmt"
	"log/slog"

	"github.com/fclairamb/ftpserver/config/confpar"
	"github.com/fclairamb/ftpserver/fs/afos"
	"github.com/spf13/afero"
)

// UnsupportedFsError is returned when a backend is unavailable in this build.
type UnsupportedFsError struct {
	Type string
}

func (err UnsupportedFsError) Error() string {
	return fmt.Sprintf("Unsupported FS in local-only build: %s", err.Type)
}

// LoadFs loads a local OS directory and optionally makes it read-only.
func LoadFs(access *confpar.Access, _ *slog.Logger) (afero.Fs, error) {
	if access.Fs != "os" {
		return nil, &UnsupportedFsError{Type: access.Fs}
	}
	loaded, err := afos.LoadFs(access)
	if err == nil && access.ReadOnly {
		loaded = afero.NewReadOnlyFs(loaded)
	}
	return loaded, err
}
