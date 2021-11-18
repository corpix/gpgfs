package fuse

import (
	"git.backbone/corpix/gpgfs/pkg/errors"
)

type Config struct {
	Key        *KeyConfig `yaml:"key"`
	AllowOther bool       `yaml:"allow-other"`
	Debug      bool       `yaml:"debug"`
}

func (c *Config) Default() {
loop:
	for {
		switch {
		case c.Key == nil:
			c.Key = &KeyConfig{}
		default:
			break loop
		}
	}
}

//

type KeyConfig struct {
	Format string `yaml:"format"`
	Path   string `yaml:"path"`
}

func (c *KeyConfig) Default() {
loop:
	for {
		switch {
		case c.Format == "":
			c.Format = KeyFormatSSH
		default:
			break loop
		}
	}
}

func (c *KeyConfig) Validate() error {
	if c.Path == "" {
		return errors.New("private key path should not be empty")
	}
	return nil
}
