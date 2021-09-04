package middleware

import (
	echo "github.com/labstack/echo/v4"

	"git.backbone/corpix/gpgfs/pkg/crypto"
	"git.backbone/corpix/gpgfs/pkg/errors"
	"git.backbone/corpix/gpgfs/pkg/server/session"
)

const SessionContextKey = "session"

func NewSession(sc session.Config, rand crypto.Rand) echo.MiddlewareFunc {
	var (
		decryptErr      = crypto.ErrDecrypt{}
		formatErr       = crypto.ErrFormat{}
		invalidErr      = session.ErrInvalid{}
		incompatibleErr = session.ErrIncompatible{}
	)

	// TODO: configurable session store type (cookie | ...)
	newStore := func(c echo.Context) (session.Store, error) {
		s, err := session.New(sc, rand)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create session")
		}

		return session.NewCookieStore(*sc.Cookie, c, s), nil
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			s, err := newStore(c)
			if err != nil {
				return err
			}

			err, _ = s.Load()
			if err != nil {
				werr := errors.Wrap(err, "failed to load session")
				if errors.HasType(err, decryptErr) || errors.HasType(err, formatErr) {
					c.Logger().Warn(werr)
					// it is ok to continue here because session was not loaded
					// and we still have our initially
				} else {
					return werr
				}
			}

			err = s.Validate()
			if err != nil {
				werr := errors.Wrap(err, "failed to validate session")
				// if session is invalid or has wrong version then just drop it
				// otherwise fail the request
				if errors.HasType(err, invalidErr) || errors.HasType(err, incompatibleErr) {
					c.Logger().Warn(werr)

					s, err = newStore(c)
					if err != nil {
						return err
					}
				} else {
					return werr
				}
			}

			if s.RefreshRequired() {
				c.Logger().Debug("session refresh required, refreshing")
				s.Refresh()

				// FIXME: this may send 2 cookies with the same name to the client
				// first which created by session.New(...)
				// second is an update saved from handler
				// usually this is fine, but... could we do better?
				err = s.Save()
				if err != nil {
					return err
				}
			}

			c.Set(SessionContextKey, s)

			return next(c)
		}
	}
}
