package web

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	healthcontroller "github.com/Roshick/manifest-maestro/internal/web/controller/health"
	metricscontroller "github.com/Roshick/manifest-maestro/internal/web/controller/metrics"
	swaggercontroller "github.com/Roshick/manifest-maestro/internal/web/controller/swagger"
	v1controller "github.com/Roshick/manifest-maestro/internal/web/controller/v1"
	"github.com/google/uuid"

	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/web/middleware"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	auapmmiddleware "github.com/StephanHCB/go-autumn-restclient-apm/implementation/middleware"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	config *config.ApplicationConfig

	healthController  *healthcontroller.Controller
	swaggerController *swaggercontroller.Controller
	metricsController *metricscontroller.Controller
	v1Controller      *v1controller.Controller

	Router chi.Router

	RequestTimeoutInSeconds     int
	ServerReadTimeoutInSeconds  int
	ServerWriteTimeoutInSeconds int
	ServerIdleTimeoutInSeconds  int
	GracePeriodInSeconds        int
}

func NewServer(
	ctx context.Context,
	config *config.ApplicationConfig,
	healthController *healthcontroller.Controller,
	swaggerController *swaggercontroller.Controller,
	metricsController *metricscontroller.Controller,
	v1Controller *v1controller.Controller,
) (*Server, error) {
	server := &Server{
		config: config,

		healthController:  healthController,
		swaggerController: swaggerController,
		metricsController: metricsController,
		v1Controller:      v1Controller,

		RequestTimeoutInSeconds:     60,
		ServerWriteTimeoutInSeconds: 60,
		ServerIdleTimeoutInSeconds:  60,
		ServerReadTimeoutInSeconds:  60,
		GracePeriodInSeconds:        30,
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

		if err := s.setupMiddlewareStack(ctx); err != nil {
			return err
		}

	}

	s.healthController.WireUp(ctx, s.Router)
	s.swaggerController.WireUp(ctx, s.Router)
	s.metricsController.WireUp(ctx, s.Router)
	s.v1Controller.WireUp(ctx, s.Router)

	return nil
}

func (s *Server) setupMiddlewareStack(_ context.Context) error {
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "Top",
	}))

	s.Router.Use(middleware.AddLoggerToContext)
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddLoggerToContext",
	}))

	s.Router.Use(middleware.CreateAddRequestIDToContext(middleware.AddRequestIDToContextOptions{
		RequestIDHeader: "X-Request-ID",
		RequestIDFunc:   uuid.NewString,
	}))
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddRequestIDToContext",
	}))

	s.Router.Use(middleware.CreateAddRequestIDToResponseHeader(middleware.AddRequestIDToResponseHeaderOptions{
		RequestIDHeader: "X-Request-ID",
	}))
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddRequestIDToResponseHeader",
	}))

	s.Router.Use(middleware.AddRequestIDToContextLogger)
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddRequestIDToContextLogger",
	}))

	s.Router.Use(middleware.CreateAddRequestResponseContextLogging(middleware.AddRequestResponseContextLoggingOptions{
		ExcludeLogging: []string{
			"GET /health/ready 200",
			"GET /health/live 200",
			"GET /metrics 200",
		},
	}))
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddRequestResponseContextLogging",
	}))

	s.Router.Use(middleware.RecoverPanic)
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "RecoverPanic",
	}))

	s.Router.Use(auapmmiddleware.AddTraceHeadersToResponse)
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddTraceHeadersToResponse",
	}))

	s.Router.Use(middleware.CreateHandleCORS(middleware.HandleCORSOptions{
		AllowOrigin:             "*",
		AdditionalAllowHeaders:  []string{"X-Request-ID"},
		AdditionalExposeHeaders: []string{"X-Request-ID"},
	}))
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "HandleCORS",
	}))

	s.Router.Use(middleware.CreateRecordRequestMetrics())
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "RecordRequestMetrics",
	}))

	s.Router.Use(middleware.CreateAddRequestTimeout(middleware.AddRequestTimeoutOptions{
		RequestTimeoutInSeconds: s.RequestTimeoutInSeconds,
	}))
	s.Router.Use(middleware.CreateLogContextCancellation(middleware.LogContextCancellationOptions{
		Description: "AddRequestTimeout",
	}))

	return nil
}

func (s *Server) NewServer(ctx context.Context, address string, router http.Handler) *http.Server {
	return &http.Server{
		Addr:         address,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context {
			return ctx
		},
	}
}

func (s *Server) Run(ctx context.Context) error {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	srvPrimary := s.CreatePrimaryServer(ctx)

	go func() {
		<-sig // wait for signal notification

		tCtx, tCancel := context.WithTimeout(context.Background(), time.Duration(s.GracePeriodInSeconds)*time.Second)
		defer tCancel()

		aulogging.Logger.NoCtx().Debug().Print("stopping services now")

		if err := srvPrimary.Shutdown(tCtx); err != nil {
			aulogging.Logger.NoCtx().Error().WithErr(err).
				Printf("failed to shut down primary http web gracefully within %d seconds: %s", s.GracePeriodInSeconds, err.Error())
			// this is not perfect, but we need to terminate the entire process because we've trapped sigterm
			os.Exit(config.ExitCodeDirtyShutdown)
		}
	}()

	if err := s.StartPrimaryServer(ctx, srvPrimary); err != nil {
		aulogging.Logger.Ctx(ctx).Fatal().WithErr(err).Print("failed to start primary server")
		return err
	}

	aulogging.Logger.Ctx(ctx).Info().Print("web layer torn down successfully")
	return nil
}
