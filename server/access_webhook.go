//go:build !localfs

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fclairamb/ftpserver/config/confpar"
)

func (s *Server) getAccessFromWebhook(user, pass string) (*confpar.Access, error) {
	jsonData, err := json.Marshal(map[string]string{"user": user, "pass": pass})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Content.AccessesWebhook.Timeout.Duration)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", s.config.Content.AccessesWebhook.URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range s.config.Content.AccessesWebhook.Headers {
		req.Header.Set(key, value)
	}
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	access := new(confpar.Access)
	if err := json.Unmarshal(body, access); err != nil {
		return nil, err
	}
	return access, nil
}
