package container

import (
	"encoding/base64"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
	msgpack "github.com/vmihailenco/msgpack/v5"

	"git.backbone/corpix/gpgfs/pkg/crypto"
	"git.backbone/corpix/gpgfs/pkg/errors"
)

const Version uint = 0

var (
	encoding = base64.URLEncoding

	compressor, _   = zstd.NewWriter(nil)
	decompressor, _ = zstd.NewReader(nil)
)

type (
	EncryptedContainer = []byte
	Container          struct {
		lock *sync.RWMutex

		Version     uint
		ValidAfter  time.Time
		ValidBefore time.Time
		Payload     Payload
	}
)

func (s *Container) Refresh(validAfter time.Time, validBefore time.Time) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.ValidAfter = validAfter
	s.ValidBefore = validBefore
}

func (s *Container) Get(key PayloadKey) (PayloadValue, bool) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	v, ok := s.Payload[key]
	if !ok {
		return nil, false
	}

	return v, true
}

func (s *Container) Set(key PayloadKey, value PayloadValue) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.Payload[key] = value
}

func (s *Container) Del(key PayloadKey) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.Payload[key]; ok {
		delete(s.Payload, key)
		return ok
	} else {
		return ok
	}
}

func (s *Container) Clean() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.Payload = Payload{}
}

func (s *Container) Data() Payload {
	s.lock.RLock()
	defer s.lock.RUnlock()

	res := make(Payload, len(s.Payload))
	for k, v := range s.Payload {
		res[k] = v
	}
	return res
}

//

func Marshal(v *Container) ([]byte, error) {
	buf, err := msgpack.Marshal(v)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal container")
	}

	return buf, nil
}
func Unmarshal(buf []byte, v *Container) error {
	err := msgpack.Unmarshal(buf, v)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal container")
	}
	return nil
}

//

func Encrypt(box *crypto.SecretBox, buf []byte) (EncryptedContainer, error) {
	nonce, err := crypto.SecretBoxNonceGen(box.Rand())
	if err != nil {
		return nil, errors.Wrap(err, "failed to encrypt container")
	}

	return box.Seal(nonce, buf), nil
}

func Decrypt(box *crypto.SecretBox, enc []byte) ([]byte, error) {
	buf, err := box.Open(enc)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decrypt container")
	}

	return buf, nil
}

func Validate(s *Container) error {
	t := time.Now()

	if s.Version != Version {
		return ErrIncompatible{
			Want: Version,
			Got:  s.Version,
		}
	}

	if !t.After(s.ValidAfter) {
		return ErrInvalid{
			Relation:  "after",
			Timestamp: s.ValidAfter,
			Now:       t,
		}
	}

	if !t.Before(s.ValidBefore) {
		return ErrInvalid{
			Relation:  "before",
			Timestamp: s.ValidBefore,
			Now:       t,
		}
	}

	return nil
}

//

func Compress(buf []byte) []byte {
	return compressor.EncodeAll(buf, make([]byte, 0, len(buf)))
}

func Decompress(buf []byte) ([]byte, error) {
	return decompressor.DecodeAll(buf, nil)
}

//

func Encode(es EncryptedContainer) []byte {
	buf := make([]byte, encoding.EncodedLen(len(es)))
	encoding.Encode(buf, es)

	return buf
}

func Decode(buf []byte) (EncryptedContainer, error) {
	es := make(EncryptedContainer, encoding.DecodedLen(len(buf)))
	n, err := encoding.Decode(es, buf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode encrypted container")
	}

	return es[:n], nil
}

//

func New(validAfter time.Time, ttl time.Duration, payload Payload) *Container {
	if payload == nil {
		payload = make(Payload)
	}

	p := make(Payload, len(payload))
	for k, v := range payload {
		p[k] = v
	}

	return &Container{
		lock: &sync.RWMutex{},

		Version:     Version,
		ValidAfter:  validAfter,
		ValidBefore: validAfter.Add(ttl),
		Payload:     p,
	}
}
