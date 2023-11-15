package web

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/web/controller"
	"github.com/Roshick/manifest-maestro/internal/web/middleware"
	auapmmiddleware "github.com/StephanHCB/go-autumn-restclient-apm/implementation/middleware"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/cancellogger"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/corsheader"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/recoverer"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/requestid"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/requestidinresponse"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/requestmetrics"
	"github.com/StephanHCB/go-backend-service-common/web/middleware/timeout"

	aulogging "github.com/StephanHCB/go-autumn-logging"
	libcontroller "github.com/StephanHCB/go-backend-service-common/acorns/controller"
	"github.com/go-chi/chi/v5"
)

type Server struct {
	config *config.ApplicationConfig

	health  libcontroller.HealthController
	swagger libcontroller.SwaggerController
	v1      *controller.V1

	Router chi.Router

	RequestTimeoutInSeconds     int
	ServerReadTimeoutInSeconds  int
	ServerWriteTimeoutInSeconds int
	ServerIdleTimeoutInSeconds  int
	GracePeriodInSeconds        int
	RequestTimeoutSeconds       int
}

func NewServer(
	ctx context.Context,
	config *config.ApplicationConfig,
	health libcontroller.HealthController,
	swagger libcontroller.SwaggerController,
	v1 *controller.V1,
) (*Server, error) {
	server := &Server{
		config: config,

		health:  health,
		swagger: swagger,
		v1:      v1,

		RequestTimeoutInSeconds:     60,
		ServerWriteTimeoutInSeconds: 60,
		ServerIdleTimeoutInSeconds:  60,
		ServerReadTimeoutInSeconds:  60,
		GracePeriodInSeconds:        30,
		RequestTimeoutSeconds:       60,
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

	s.health.WireUp(ctx, s.Router)
	s.swagger.WireUp(ctx, s.Router)
	s.v1.WireUp(ctx, s.Router)

	return nil
}

func (s *Server) setupMiddlewareStack(_ context.Context) error {
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("Top"))

	s.Router.Use(middleware.AddLoggerToContext)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddLoggerToContext"))

	s.Router.Use(requestid.RequestID)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestIDToContext"))

	s.Router.Use(requestidinresponse.AddRequestIdHeaderToResponse)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestIDHeaderToResponse"))

	s.Router.Use(middleware.AddRequestIDToContextLogger)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestIDToContextLogger"))

	addRequestResponseContextLoggingOptions := middleware.AddRequestResponseContextLoggingOptions{
		ExcludeLogging: []string{
			"GET / 200",
			"GET /health 200",
			"GET /management/health 200",
		},
	}
	s.Router.Use(middleware.CreateAddRequestResponseContextLogging(addRequestResponseContextLoggingOptions))
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestResponseContextLogging"))

	s.Router.Use(recoverer.PanicRecoverer)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("RecoverPanic"))

	s.Router.Use(auapmmiddleware.AddTraceHeadersToResponse)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddTraceHeadersToResponse"))

	s.Router.Use(corsheader.CorsHandlingWithCorsAllowOrigin("*"))
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("HandleCORS"))

	requestmetrics.Setup()
	s.Router.Use(requestmetrics.RecordRequestMetrics)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("RecordRequestMetrics"))

	timeout.RequestTimeoutSeconds = s.RequestTimeoutInSeconds
	s.Router.Use(timeout.AddRequestTimeout)
	s.Router.Use(cancellogger.ConstructContextCancellationLoggerMiddleware("AddRequestTimeout"))

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
	srvMetrics := s.CreateMetricsServer(ctx)

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
		if srvMetrics != nil {
			if err := srvMetrics.Shutdown(tCtx); err != nil {
				aulogging.Logger.NoCtx().Error().WithErr(err).
					Printf("failed to shut down metrics http server gracefully within %d seconds: %s", s.GracePeriodInSeconds, err.Error())
				// this is not perfect, but we need to terminate the entire process because we've trapped sigterm
				os.Exit(config.ExitCodeDirtyShutdown)
			}
		}
	}()

	if srvMetrics != nil {
		go s.StartMetricsServer(ctx, srvMetrics)
	}
	if err := s.StartPrimaryServer(ctx, srvPrimary); err != nil {
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Print("failed to start primary server. BAILING OUT")
		return err
	}

	aulogging.Logger.Ctx(ctx).Info().Print("web layer torn down successfully")
	return nil
}
