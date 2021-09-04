package server

import (
	"net/http"

	echo "github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"git.backbone/corpix/gpgfs/pkg/log"
	serverErrors "git.backbone/corpix/gpgfs/pkg/server/errors"
	"git.backbone/corpix/gpgfs/pkg/server/middleware"
	"git.backbone/corpix/gpgfs/pkg/server/session"
	telemetry "git.backbone/corpix/gpgfs/pkg/telemetry/registry"
)

type (
	Server         = echo.Echo
	MiddlewareFunc = echo.MiddlewareFunc
	HandlerFunc    = echo.HandlerFunc
	Context        = echo.Context
	Router         = echo.Group
	Session        = session.Session

	HTTPError = echo.HTTPError
	Error     = serverErrors.Error

	ResultError struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	ResultPayload = interface{}
	Result        struct {
		Ok      bool          `json:"ok"`
		Error   *ResultError  `json:"error,omitempty"`
		Payload ResultPayload `json:"payload,omitempty"`
	}

	HTTPOption = func(*http.Server)
)

const (
	HeaderAccept                          = echo.HeaderAccept
	HeaderAcceptEncoding                  = echo.HeaderAcceptEncoding
	HeaderAllow                           = echo.HeaderAllow
	HeaderAuthorization                   = echo.HeaderAuthorization
	HeaderContentDisposition              = echo.HeaderContentDisposition
	HeaderContentEncoding                 = echo.HeaderContentEncoding
	HeaderContentLength                   = echo.HeaderContentLength
	HeaderContentType                     = echo.HeaderContentType
	HeaderCookie                          = echo.HeaderCookie
	HeaderSetCookie                       = echo.HeaderSetCookie
	HeaderIfModifiedSince                 = echo.HeaderIfModifiedSince
	HeaderLastModified                    = echo.HeaderLastModified
	HeaderLocation                        = echo.HeaderLocation
	HeaderUpgrade                         = echo.HeaderUpgrade
	HeaderVary                            = echo.HeaderVary
	HeaderWWWAuthenticate                 = echo.HeaderWWWAuthenticate
	HeaderXForwardedFor                   = echo.HeaderXForwardedFor
	HeaderXForwardedProto                 = echo.HeaderXForwardedProto
	HeaderXForwardedProtocol              = echo.HeaderXForwardedProtocol
	HeaderXForwardedSsl                   = echo.HeaderXForwardedSsl
	HeaderXUrlScheme                      = echo.HeaderXUrlScheme
	HeaderXHTTPMethodOverride             = echo.HeaderXHTTPMethodOverride
	HeaderXRealIP                         = echo.HeaderXRealIP
	HeaderXRequestID                      = echo.HeaderXRequestID
	HeaderXRequestedWith                  = echo.HeaderXRequestedWith
	HeaderServer                          = echo.HeaderServer
	HeaderOrigin                          = echo.HeaderOrigin
	HeaderAccessControlRequestMethod      = echo.HeaderAccessControlRequestMethod
	HeaderAccessControlRequestHeaders     = echo.HeaderAccessControlRequestHeaders
	HeaderAccessControlAllowOrigin        = echo.HeaderAccessControlAllowOrigin
	HeaderAccessControlAllowMethods       = echo.HeaderAccessControlAllowMethods
	HeaderAccessControlAllowHeaders       = echo.HeaderAccessControlAllowHeaders
	HeaderAccessControlAllowCredentials   = echo.HeaderAccessControlAllowCredentials
	HeaderAccessControlExposeHeaders      = echo.HeaderAccessControlExposeHeaders
	HeaderAccessControlMaxAge             = echo.HeaderAccessControlMaxAge
	HeaderStrictTransportSecurity         = echo.HeaderStrictTransportSecurity
	HeaderXContentTypeOptions             = echo.HeaderXContentTypeOptions
	HeaderXXSSProtection                  = echo.HeaderXXSSProtection
	HeaderXFrameOptions                   = echo.HeaderXFrameOptions
	HeaderContentSecurityPolicy           = echo.HeaderContentSecurityPolicy
	HeaderContentSecurityPolicyReportOnly = echo.HeaderContentSecurityPolicyReportOnly
	HeaderXCSRFToken                      = echo.HeaderXCSRFToken
	HeaderReferrerPolicy                  = echo.HeaderReferrerPolicy

	QueryDelimiter = "?"
)

var (
	NewError = serverErrors.NewError
)

func HTTPTimeoutOption(c TimeoutConfig) HTTPOption {
	return func(s *http.Server) {
		s.ReadTimeout = c.Read
		s.WriteTimeout = c.Write
	}
}

func NewHTTP(addr string, options ...HTTPOption) *http.Server {
	s := &http.Server{Addr: addr}
	for _, fn := range options {
		fn(s)
	}
	return s
}

//

func DefaultHTTPErrorHandler(err error, c echo.Context) {
	if _, ok := err.(*echo.HTTPError); ok {
		c.Echo().DefaultHTTPErrorHandler(err, c)
		return
	}

	//

	code := http.StatusInternalServerError
	r := Result{
		Ok: false,
		Error: &ResultError{
			Code:    code,
			Message: http.StatusText(code),
		},
	}

	if e, ok := err.(*Error); ok {
		r.Error.Code = e.Code
		r.Error.Message = e.Error()
	}

	_ = c.JSON(r.Error.Code, r)
}

func New(name string, l log.Logger, r *telemetry.Registry) *Server {
	e := echo.New()
	e.HideBanner = true
	e.Logger = &middleware.Logger{Logger: l}
	e.HTTPErrorHandler = DefaultHTTPErrorHandler

	e.Use(echomw.RequestID())
	e.Use(middleware.NewLogger(l, ""))
	e.Use(middleware.NewTelemetry(r, name))
	e.Use(middleware.NewRecover(nil, l))

	return e
}
