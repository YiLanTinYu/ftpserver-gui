//go:build localfs

package server

import (
	"errors"

	"github.com/fclairamb/ftpserver/config/confpar"
)

func (s *Server) getAccessFromWebhook(_, _ string) (*confpar.Access, error) {
	return nil, errors.New("access webhook is not available in the local-only build")
}
