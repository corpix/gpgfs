package session

import (
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"git.backbone/corpix/gpgfs/pkg/crypto"
	"git.backbone/corpix/gpgfs/pkg/crypto/container"
	"git.backbone/corpix/gpgfs/pkg/errors"
	"git.backbone/corpix/gpgfs/pkg/meta"
)

type (
	Container    = container.Container
	Payload      = container.Payload
	PayloadKey   = container.PayloadKey
	PayloadValue = container.PayloadValue

	ErrIncompatible = container.ErrIncompatible
	ErrInvalid      = container.ErrInvalid

	Session struct {
		container *Container
		config    Config
		box       *crypto.SecretBox
	}
)

const (
	Version = container.Version

	SameSiteDefault = "default"
	SameSiteLax     = "lax"
	SameSiteStrict  = "strict"
	SameSiteNone    = "none"
)

var (
	Name     = "_" + meta.Name
	SameSite = map[string]http.SameSite{
		SameSiteDefault: http.SameSiteDefaultMode,
		SameSiteLax:     http.SameSiteLaxMode,
		SameSiteStrict:  http.SameSiteStrictMode,
		SameSiteNone:    http.SameSiteNoneMode,
	}
)

//

func (s *Session) Name() string {
	return s.config.Name
}

func (s *Session) Unwrap() *Container {
	return s.container
}

func (s *Session) ValidAfter() time.Time {
	return s.container.ValidAfter
}

func (s *Session) ValidBefore() time.Time {
	return s.container.ValidBefore
}

func (s *Session) RefreshRequired() bool {
	return time.Now().After(s.container.ValidAfter.Add(s.config.Refresh))
}

func (s *Session) Save() ([]byte, error) {
	buf, err := container.Marshal(s.container)
	if err != nil {
		return nil, err
	}
	enc, err := container.Encrypt(s.box, container.Compress(buf))
	if err != nil {
		return nil, err
	}

	return container.Encode(enc), nil
}

func (s *Session) Load(encoded []byte) error {
	enc, err := container.Decode(encoded)
	if err != nil {
		return err
	}
	dec, err := container.Decrypt(s.box, enc)
	if err != nil {
		return err
	}
	buf, err := container.Decompress(dec)
	if err != nil {
		return err
	}

	return container.Unmarshal(buf, s.container)
}

func (s *Session) Validate() error {
	return container.Validate(s.container)
}

//

func (s *Session) Refresh() {
	t := time.Now()
	s.container.Refresh(t, t.Add(s.config.MaxAge))
}

//

func (s *Session) Get(key PayloadKey) (PayloadValue, bool) {
	return s.container.Get(key)
}

func (s *Session) GetString(key PayloadKey) (string, bool) {
	bytes, ok := s.container.Get(key)
	return string(bytes), ok
}

func (s *Session) Set(key PayloadKey, value PayloadValue) {
	s.container.Set(key, value)
	s.Refresh()
}

func (s *Session) SetString(key PayloadKey, value string) {
	s.container.Set(key, []byte(value))
	s.Refresh()
}

func (s *Session) Del(key PayloadKey) bool {
	ok := s.container.Del(key)
	if ok {
		s.Refresh()
	}
	return ok
}

func (s *Session) Clean() {
	s.container.Clean()
}

func (s *Session) Data() Payload {
	return s.container.Data()
}

//

func New(c Config, rand io.Reader) (*Session, error) {
	var s []byte
	if c.EncryptionKeyFile != "" {
		buf, err := ioutil.ReadFile(c.EncryptionKeyFile)
		if err != nil {
			return nil, errors.Wrapf(
				err, "failed to load encryption-key-file: %q",
				c.EncryptionKeyFile,
			)
		}
		s = buf
	} else {
		s = []byte(c.EncryptionKey)
	}

	if crypto.SecretBoxKeySize != len(s) {
		return nil, errors.Errorf(
			"invalid encryption key length, want %d, got %d",
			crypto.SecretBoxKeySize, len(s),
		)
	}
	key := new(crypto.SecretBoxKey)
	copy(key[:], s)

	return &Session{
		container: container.New(time.Now(), c.MaxAge, nil),
		config:    c,
		box:       crypto.NewSecretBox(rand, key),
	}, nil
}
