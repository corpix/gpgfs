package telemetry

import (
	"context"
	"net"
	"net/http"

	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"git.backbone/corpix/gpgfs/pkg/log"
	"git.backbone/corpix/gpgfs/pkg/server"
	"git.backbone/corpix/gpgfs/pkg/telemetry/registry"
)

type (
	Registry = registry.Registry
	Listener net.Listener
)

var DefaultRegistry = registry.DefaultRegistry

//

type Server struct {
	config  Config
	log     log.Logger
	srv     *server.Server
	handler http.Handler
}

func (s *Server) ListenAndServe() error {
	err := s.srv.StartServer(
		server.NewHTTP(
			s.config.Addr,
			server.HTTPTimeoutOption(*s.config.Timeout),
		),
	)
	if err == http.ErrServerClosed {
		s.log.
			Warn().
			Str("addr", s.config.Addr).
			Msg("server shutdown")
		return nil
	}

	return err
}

func (s *Server) Handle(ctx server.Context) error {
	s.handler.ServeHTTP(
		ctx.Response().Writer,
		ctx.Request(),
	)
	return nil
}

func (s *Server) Close() error {
	err := s.srv.Close()
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func New(c Config, l log.Logger, r *Registry, lr Listener) *Server {
	var addr string

	if lr != nil {
		addr = lr.Addr().String()
	} else {
		addr = c.Addr
	}

	l = l.With().Str("component", Subsystem).Str("listener", addr).Logger()

	h := promhttp.InstrumentMetricHandler(
		r,
		promhttp.HandlerFor(
			r,
			promhttp.HandlerOpts{ErrorLog: log.Std(l)},
		),
	)

	e := server.New(Subsystem, l, r)
	e.Listener = lr
	e.Use(echomw.BodyLimit("0"))

	s := &Server{
		config:  c,
		log:     l,
		srv:     e,
		handler: h,
	}

	e.GET(c.Path, s.Handle)

	return s
}
