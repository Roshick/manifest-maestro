package web

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Roshick/go-autumn-web/logging"
	"github.com/Roshick/go-autumn-web/metrics"
	"github.com/Roshick/go-autumn-web/resiliency"
	"github.com/Roshick/go-autumn-web/security"
	"github.com/Roshick/go-autumn-web/tracing"
	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/Roshick/manifest-maestro/internal/web/controller"

	"github.com/Roshick/manifest-maestro/internal/config"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-chi/chi/v5"
	"github.com/riandyrn/otelchi"
)

type Controller interface {
	WireUp(ctx context.Context, r chi.Router)
}

type Server struct {
	primaryAddress  string
	applicationName string
	controllers     []Controller

	Router chi.Router

	readTimeout  time.Duration
	writeTimeout time.Duration
	idleTimeout  time.Duration

	shutdownGracePeriod time.Duration
}

func NewServer(
	ctx context.Context,
	address string,
	primaryPort uint,
	applicationName string,
	controllers ...Controller,
) (*Server, error) {
	server := &Server{
		primaryAddress:  fmt.Sprintf("%s:%d", address, primaryPort),
		applicationName: applicationName,
		controllers:     controllers,

		readTimeout:  10 * time.Second,
		writeTimeout: 120 * time.Second,
		idleTimeout:  120 * time.Second,

		shutdownGracePeriod: 30 * time.Second,
	}

	if err := server.WireUp(ctx); err != nil {
		return nil, err
	}
	return server, nil
}

func (s *Server) WireUp(ctx context.Context) error {
	if s.Router == nil {
		aulogging.Logger.Ctx(ctx).Info().Print("creating router and setting up filter chain")
		s.Router = chi.NewRouter()

		s.setupRootMiddlewares(ctx)
	}

	for _, c := range s.controllers {
		c.WireUp(ctx, s.Router)
	}

	return nil
}

func (s *Server) setupRootMiddlewares(_ context.Context) {
	opts := logging.DefaultContextCancellationLoggerMiddlewareOptions()
	opts.Description = "server"
	s.Router.Use(logging.NewContextCancellationLoggerMiddleware(opts))

	s.Router.Use(security.NewCORSMiddleware(nil))

	s.Router.Use(logging.NewContextLoggerMiddleware(nil))

	s.Router.Use(tracing.NewRequestIDHeaderMiddleware(nil))
	s.Router.Use(tracing.NewRequestIDLoggerMiddleware(nil))

	s.Router.Use(otelchi.Middleware(s.applicationName, otelchi.WithRequestMethodInSpanName(true)))
	s.Router.Use(tracing.NewTracingLoggerMiddleware(nil))

	s.Router.Use(metrics.NewRequestMetricsMiddleware(nil))

	s.Router.Use(resiliency.NewPanicRecoveryMiddleware(&resiliency.RecoveryMiddlewareOptions{
		ErrorResponse: &controller.APIError{StatusCode: http.StatusInternalServerError, Error: openapi.Error{
			Title: utils.Ptr("Internal server error"),
		}},
	}))
}

func (s *Server) NewServer(ctx context.Context, address string, router http.Handler) *http.Server {
	return &http.Server{
		Addr:         address,
		Handler:      router,
		ReadTimeout:  s.readTimeout,
		WriteTimeout: s.writeTimeout,
		IdleTimeout:  s.idleTimeout,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	srvPrimary := s.CreatePrimaryServer(ctx)

	go func() {
		<-sig // wait for signal notification
		defer cancel()
		aulogging.Logger.Ctx(ctx).Debug().Print("stopping services now")

		tCtx, tCancel := context.WithTimeout(context.Background(), s.shutdownGracePeriod)
		defer tCancel()

		if err := srvPrimary.Shutdown(tCtx); err != nil {
			aulogging.Logger.NoCtx().Error().WithErr(err).
				Printf("failed to shut down primary http web gracefully within %f seconds: %s", s.shutdownGracePeriod.Seconds(), err.Error())
			// this is not perfect, but we need to terminate the entire process because we've trapped sigterm
			os.Exit(config.ExitCodeDirtyShutdown)
		}
	}()

	if err := s.StartPrimaryServer(ctx, srvPrimary); err != nil {
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Print("failed to start primary server. BAILING OUT")
		return err
	}

	aulogging.Logger.Ctx(ctx).Info().Print("web layer torn down successfully")
	return nil
}
