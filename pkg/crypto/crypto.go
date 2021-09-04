package crypto

import (
	"crypto/rand"
	"io"
)

type Rand io.Reader

var DefaultRand = Rand(rand.Reader)
