package server

import (
	"time"
)

type TimeoutConfig struct {
	Read  time.Duration
	Write time.Duration
}

func (c *TimeoutConfig) Default() {
loop:
	for {
		switch {
		case c.Read <= 0:
			c.Read = 5 * time.Second
		case c.Write <= 0:
			c.Write = 5 * time.Second
		default:
			break loop
		}
	}
}
