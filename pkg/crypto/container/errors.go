package container

import (
	"fmt"
	"time"
)

type ErrIncompatible struct {
	Want uint
	Got  uint
}

func (e ErrIncompatible) Error() string {
	return fmt.Sprintf(
		"container version is incompatible, want %d, got %d",
		e.Want, e.Got,
	)
}

//

type ErrInvalid struct {
	Relation  string
	Timestamp time.Time
	Now       time.Time
}

func (e ErrInvalid) Error() string {
	return fmt.Sprintf(
		"container version is expired, valid %s %q, but now %q",
		e.Relation, e.Timestamp, e.Now,
	)
}
