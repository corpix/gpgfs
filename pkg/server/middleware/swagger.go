package middleware

import (
	"net/http"

	echo "github.com/labstack/echo/v4"
	swagger "github.com/swaggo/echo-swagger"
)

// see: https://github.com/swaggo/swag#declarative-comments-format

func MountSwagger(e *echo.Echo, prefix string) {
	index := prefix + "/index.html"

	e.GET(prefix, func(ctx echo.Context) error {
		return ctx.Redirect(http.StatusPermanentRedirect, index)
	})
	e.GET(prefix+"/", func(ctx echo.Context) error {
		return ctx.Redirect(http.StatusPermanentRedirect, index)
	})
	e.GET(prefix+"/*", swagger.WrapHandler)
}
