//go:build localfs

package config

import (
	"errors"
	"log/slog"
	"testing"

	"github.com/fclairamb/ftpserver/config/confpar"
	"golang.org/x/crypto/bcrypt"
)

func TestLocalConfigAuthenticatesPlainAndBcryptPasswords(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	c := &Config{logger: slog.Default(), Content: &confpar.Content{Accesses: []*confpar.Access{{User: "plain", Pass: "secret"}, {User: "hashed", Pass: string(hash)}}}}
	if _, err := c.GetAccess("plain", "secret"); err != nil {
		t.Fatalf("plain password: %v", err)
	}
	if _, err := c.GetAccess("hashed", "password"); err != nil {
		t.Fatalf("bcrypt password: %v", err)
	}
	if _, err := c.GetAccess("plain", "wrong"); !errors.Is(err, ErrUnknownUser) {
		t.Fatalf("wrong password error = %v", err)
	}
}
