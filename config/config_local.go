//go:build localfs

package config

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"

	"github.com/fclairamb/ftpserver/config/confpar"
	"github.com/fclairamb/ftpserver/fs"
	"golang.org/x/crypto/bcrypt"
)

var ErrUnknownUser = errors.New("unknown user")

type Config struct {
	fileName string
	logger   *slog.Logger
	Content  *confpar.Content
}

func NewConfig(fileName string, logger *slog.Logger) (*Config, error) {
	if fileName == "" {
		fileName = "ftpserver.json"
	}
	c := &Config{fileName: fileName, logger: logger}
	if err := c.Load(); err != nil {
		return nil, err
	}
	return c, nil
}

func FromContent(content *confpar.Content, fileName string, logger *slog.Logger) (*Config, error) {
	c := &Config{fileName: fileName, logger: logger, Content: content}
	if err := c.Prepare(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Config) Load() error {
	data, err := os.ReadFile(c.fileName)
	if err != nil {
		return err
	}
	var content confpar.Content
	if err := json.Unmarshal(data, &content); err != nil {
		return err
	}
	c.Content = &content
	if c.Content.HashPlaintextPasswords {
		if err := c.HashPlaintextPasswords(); err != nil {
			return err
		}
	}
	return c.Prepare()
}

func (c *Config) Prepare() error {
	if c.Content.ListenAddress == "" {
		c.Content.ListenAddress = "0.0.0.0:2121"
	}
	if publicHost := os.Getenv("PUBLIC_HOST"); publicHost != "" {
		c.Content.PublicHost = publicHost
	}
	return nil
}

func (c *Config) HashPlaintextPasswords() error {
	changed := false
	for _, access := range c.Content.Accesses {
		if access.User == "anonymous" && access.Pass == "*" {
			continue
		}
		if strings.HasPrefix(access.Pass, "$2") {
			continue
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(access.Pass), 10)
		if err != nil {
			return err
		}
		access.Pass = string(hash)
		changed = true
	}
	if !changed {
		return nil
	}
	data, err := json.MarshalIndent(c.Content, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.fileName, data, 0600)
}

func (c *Config) CheckAccesses() error {
	for _, access := range c.Content.Accesses {
		if _, err := fs.LoadFs(access, c.logger); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) GetAccess(user, pass string) (*confpar.Access, error) {
	for _, access := range c.Content.Accesses {
		if access.User != user {
			continue
		}
		if strings.HasPrefix(access.Pass, "$2") {
			if bcrypt.CompareHashAndPassword([]byte(access.Pass), []byte(pass)) == nil {
				return access, nil
			}
			continue
		}
		if access.Pass == pass || (access.User == "anonymous" && access.Pass == "*") {
			return access, nil
		}
	}
	return nil, ErrUnknownUser
}
