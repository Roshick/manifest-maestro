package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	aulogging "github.com/StephanHCB/go-autumn-logging"
)

func (s *Server) CreatePrimaryServer(ctx context.Context) *http.Server {
	aulogging.Logger.Ctx(ctx).Info().Printf("creating primary http server on %s", s.primaryAddress)
	return s.NewServer(ctx, s.primaryAddress, s.Router)
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
