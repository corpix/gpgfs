package middleware

import (
	"git.backbone/corpix/gpgfs/pkg/errors"
)

type CORSConfig struct {
	AllowOriginsRegexp []string
	AllowOrigins       []string
	AllowMethods       []string
	AllowHeaders       []string
	AllowCredentials   bool
	ExposeHeaders      []string
	MaxAge             int
}

func (c *CORSConfig) Validate() error {
	if len(c.AllowOriginsRegexp) != 0 && len(c.AllowOrigins) != 0 {
		return errors.New("either allow origins regexp or allow origins list should be defined, not both")
	}

	return nil
}

func (c *CORSConfig) Default() {
loop:
	for {
		switch {
		case len(c.AllowMethods) == 0:
			c.AllowMethods = []string{"POST"}
		case c.MaxAge == 0:
			c.MaxAge = 900 // 15 min
		default:
			break loop
		}
	}
}
