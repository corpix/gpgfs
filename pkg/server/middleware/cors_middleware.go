package middleware

import (
	"regexp"

	echo "github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
)

func NewCORSMiddleware(c CORSConfig) echo.MiddlewareFunc {
	cc := echomw.CORSConfig{
		Skipper:          echomw.DefaultSkipper,
		AllowMethods:     c.AllowMethods,
		AllowHeaders:     c.AllowHeaders,
		AllowCredentials: c.AllowCredentials,
		ExposeHeaders:    c.ExposeHeaders,
		MaxAge:           c.MaxAge,
	}

	if len(c.AllowOriginsRegexp) != 0 {
		xs := make([]*regexp.Regexp, len(c.AllowOriginsRegexp))
		for k, v := range c.AllowOriginsRegexp {
			xs[k] = regexp.MustCompile(v)
		}

		cc.AllowOriginFunc = func(origin string) (bool, error) {
			for _, r := range xs {
				if r.MatchString(origin) {
					return true, nil
				}
			}

			return false, nil
		}
	} else {
		cc.AllowOrigins = c.AllowOrigins
	}

	return echomw.CORSWithConfig(cc)
}
