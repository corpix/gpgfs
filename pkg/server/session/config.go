package session

import (
	"sort"
	"time"

	"git.backbone/corpix/gpgfs/pkg/errors"
)

type Config struct {
	Name              string        `yaml:"name"`
	MaxAge            time.Duration `yaml:"max-age"`
	Refresh           time.Duration `yaml:"refresh"`
	EncryptionKey     string        `yaml:"encryption-key"`
	EncryptionKeyFile string        `yaml:"encryption-key-file"`

	Cookie *CookieConfig `yaml:"cookie"`
}

func (c *Config) Default() {
loop:
	for {
		switch {
		case c.Name == "":
			c.Name = Name
		case c.MaxAge <= 0:
			c.MaxAge = 7 * 24 * time.Hour
		case c.Refresh <= 0:
			c.Refresh = 3 * time.Hour
		case c.Cookie == nil:
			c.Cookie = &CookieConfig{}
		default:
			break loop
		}
	}
}

func (c *Config) Validate() error {
	if c.EncryptionKey != "" && c.EncryptionKeyFile != "" {
		return errors.New("either encryption-key or encryption-key-file must be defined, not both")
	}
	if c.EncryptionKey == "" && c.EncryptionKeyFile == "" {
		return errors.New("either encryption-key or encryption-key-file must be defined")
	}
	return nil
}

//

type CookieConfig struct {
	Path     string `yaml:"path"`
	Domain   string `yaml:"domain"`
	Secure   *bool  `yaml:"secure"`
	HTTPOnly *bool  `yaml:"httponly"`
	SameSite string `yaml:"same-site"`
}

func (c *CookieConfig) Default() {
loop:
	for {
		switch {
		case c.Path == "":
			c.Path = "/"
		case c.Secure == nil:
			b := true
			c.Secure = &b
		case c.HTTPOnly == nil:
			b := true
			c.HTTPOnly = &b
		case c.SameSite == "":
			c.SameSite = SameSiteDefault
		default:
			break loop
		}
	}
}

func (c *CookieConfig) Validate() error {
	if _, ok := SameSite[c.SameSite]; !ok {
		available := make([]string, len(SameSite))
		n := 0
		for k := range SameSite {
			available[n] = k
			n++
		}
		sort.Strings(available)

		return errors.Errorf(
			"unexpected same-site value %q, expected one of: %q",
			c.SameSite, available,
		)
	}
	return nil
}
