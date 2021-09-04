package middleware

import (
	"strconv"
	"time"

	"git.backbone/corpix/gpgfs/pkg/log"
	"git.backbone/corpix/gpgfs/pkg/server/errors"

	echo "github.com/labstack/echo/v4"
)

type loggerContext struct {
	echo.Context
	logger *Logger
}

func withLoggerContext(ctx echo.Context, logger *Logger) *loggerContext {
	return &loggerContext{ctx, logger}
}

func (c *loggerContext) Logger() echo.Logger {
	return c.logger
}

//

func NewLogger(l log.Logger, msg string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var (
				req   = c.Request()
				res   = c.Response()
				start = time.Now()
			)

			ll := l.With().
				Str("request_id", res.Header().Get(echo.HeaderXRequestID)).
				Str("remote_ip", c.RealIP()).
				Str("host", req.Host).
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Str("user_agent", req.UserAgent()).
				Str("referer", req.Referer()).
				Logger()

			err := next(withLoggerContext(c, &Logger{Logger: ll}))

			stop := time.Now()

			var evt *log.Event
			if err == nil {
				evt = ll.Info()
			} else {
				evt = ll.Error()
			}

			evt.
				Int("status", res.Status).
				Dur("latency", stop.Sub(start)).
				Str("latency_human", stop.Sub(start).String())

			cl := req.Header.Get(echo.HeaderContentLength)
			if cl == "" {
				cl = "0"
			}

			evt.
				Str("bytes_in", cl).
				Str("bytes_out", strconv.FormatInt(res.Size, 10))

			if err != nil {
				if e, ok := err.(*errors.Error); ok {
					evt.Interface("meta", e.Meta).Err(e.Chain())
				} else {
					evt.Err(err)
				}
			}
			evt.Msg(msg)

			return err
		}
	}
}
