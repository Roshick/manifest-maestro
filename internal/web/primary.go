package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	aulogging "github.com/StephanHCB/go-autumn-logging"
)

func (s *Server) CreatePrimaryServer(ctx context.Context) *http.Server {
	address := fmt.Sprintf("%s:%d", s.config.ServerAddress(), s.config.ServerPrimaryPort())
	aulogging.Logger.Ctx(ctx).Info().Printf("creating primary http server on %s", address)
	return s.NewServer(ctx, address, s.Router)
}

func (s *Server) StartPrimaryServer(ctx context.Context, srv *http.Server) error {
	aulogging.Logger.Ctx(ctx).Info().Print("starting primary http server")
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("failed to start primary http server: %w", err)
	}
	aulogging.Logger.Ctx(ctx).Info().Print("primary http server has shut down")
	return nil
}
