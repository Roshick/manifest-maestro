package controller

import (
	"context"
	"net/http"

	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/go-chi/render"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/go-chi/chi/v5"
)

func NewHealthController() *HealthController {
	return &HealthController{}
}

type HealthController struct{}

func (c *HealthController) WireUp(_ context.Context, r chi.Router) {
	r.Route("/health", func(router chi.Router) {
		router.Get("/readiness", c.Health)
		router.Get("/liveness", c.Health)
	})
}

func (c *HealthController) Health(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, openapi.HealthResponse{
		Status: utils.Ptr("OK"),
	})
}
