package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	aulogging "github.com/StephanHCB/go-autumn-logging"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (s *Server) CreateMetricsServer(ctx context.Context) *http.Server {
	if s.config.ServerMetricsPort() > 0 {
		address := fmt.Sprintf("%s:%d", s.config.ServerAddress(), s.config.ServerMetricsPort())
		aulogging.Logger.Ctx(ctx).Info().Printf("creating metrics http server on %s", address)
		metricsServeMux := http.NewServeMux()
		metricsServeMux.Handle("/metrics", promhttp.Handler())
		return s.NewServer(ctx, address, metricsServeMux)
	}
	aulogging.Logger.Ctx(ctx).Info().Print("will not start metrics http server - no metrics port configured")
	return nil
}

func (s *Server) StartMetricsServer(ctx context.Context, srv *http.Server) {
	aulogging.Logger.Ctx(ctx).Info().Print("starting metrics http server")
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		aulogging.Logger.NoCtx().Error().WithErr(err).Print("failed to start background metrics http server")
	}
	aulogging.Logger.NoCtx().Info().Print("metrics http server has shut down")
}
