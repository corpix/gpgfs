package telemetry

import (
	"git.backbone/corpix/gpgfs/pkg/bus"
	"git.backbone/corpix/gpgfs/pkg/errors"
	"git.backbone/corpix/gpgfs/pkg/server"
)

type Config struct {
	Enable  bool
	Addr    string
	Path    string
	Timeout *server.TimeoutConfig
}

func (c *Config) Default() {
loop:
	for {
		switch {
		case c.Addr == "":
			c.Addr = "127.0.0.1:4280"
		case c.Path == "":
			c.Path = "/"
		case c.Timeout == nil:
			c.Timeout = &server.TimeoutConfig{}
		default:
			break loop
		}
	}
}

func (c *Config) Validate() error {
	if !c.Enable {
		return nil
	}
	if c.Path == "" {
		return errors.New("path should not be empty")
	}

	return nil
}

func (c *Config) Update(cc interface{}) error {
	bus.Config <- bus.ConfigUpdate{
		Subsystem: Subsystem,
		Config:    cc,
	}
	return nil
}
