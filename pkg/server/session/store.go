package session

import (
	"net/http"
	"strings"
	"time"

	echo "github.com/labstack/echo/v4"

	"git.backbone/corpix/gpgfs/pkg/errors"
)

type Store interface {
	Load() (error, bool)
	Save() error
	Drop() error

	Refresh()
	RefreshRequired() bool
	Validate() error

	Get(PayloadKey) (PayloadValue, bool)
	GetString(PayloadKey) (string, bool)
	Set(PayloadKey, PayloadValue)
	SetString(PayloadKey, string)
	Del(PayloadKey) bool
	Clean()

	Data() Payload
	Unwrap() *Session
}

var _ Store = &CookieStore{}

type CookieStore struct {
	config  CookieConfig
	context echo.Context

	*Session
}

func (s *CookieStore) Load() (error, bool) {
	cookie, _ := s.context.Cookie(s.Session.Name())
	// XXX: dang untyped errors
	if cookie == nil {
		return nil, false
	}

	err := s.Session.Load([]byte(cookie.Value))
	if err != nil {
		return err, false
	}

	return nil, true
}

func (s *CookieStore) Save() error {
	buf, err := s.Session.Save()
	if err != nil {
		return err
	}

	s.setCookie(
		buf,
		s.Session.ValidBefore().Sub(s.Session.ValidAfter()),
		s.Session.ValidBefore(),
	)

	return nil
}

func (s *CookieStore) Drop() error {
	s.setCookie(nil, -1, time.Unix(0, 0))

	return nil
}

func (s *CookieStore) setCookie(value []byte, maxAge time.Duration, expires time.Time) {
	domain := s.config.Domain
	if domain == "" {
		domain = s.context.Request().URL.Host
	}
	// net/http: invalid Cookie.Domain "xxx.localhost:4180"; dropping domain attribute
	domain = strings.Split(domain, ":")[0]

	s.context.SetCookie(&http.Cookie{
		Name:     s.Session.Name(),
		Value:    string(value),
		Path:     s.config.Path,
		Domain:   domain,
		MaxAge:   int(maxAge / time.Second),
		Expires:  expires,
		Secure:   *s.config.Secure,
		HttpOnly: *s.config.HTTPOnly,
		SameSite: SameSite[strings.ToLower(s.config.SameSite)],
	})
}

func (s *CookieStore) Unwrap() *Session {
	return s.Session
}

//

func GetStore(key string, c echo.Context) (Store, bool) {
	v := c.Get(key)
	if v == nil {
		return nil, false
	}

	s, ok := v.(Store)

	return s, ok
}

func MustGetStore(key string, c echo.Context) Store {
	s, ok := GetStore(key, c)
	if !ok {
		panic(errors.Errorf(
			"failed to load session store from context key %q",
			key,
		))
	}

	return s
}

func NewCookieStore(c CookieConfig, ctx echo.Context, s *Session) *CookieStore {
	return &CookieStore{
		config:  c,
		context: ctx,
		Session: s,
	}
}
