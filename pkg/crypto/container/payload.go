package container

type (
	Payload      = map[PayloadKey]PayloadValue
	PayloadKey   uint
	PayloadValue = []byte
)
