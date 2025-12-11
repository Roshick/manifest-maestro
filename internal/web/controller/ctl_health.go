package controller

import (
	"context"
	"net/http"

	"github.com/go-chi/render"

	"github.com/go-chi/chi/v5"
)

func NewHealthController() *HealthController {
	return &HealthController{}
}

type HealthController struct{}

func (c *HealthController) WireUp(_ context.Context, r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Route("/health", func(r chi.Router) {
			r.Get("/readiness", c.GetReadiness)
			r.Get("/liveness", c.GetLiveness)
		})
	})
}

func (c *HealthController) GetReadiness(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"Status": "OK",
	})
}

func (c *HealthController) GetLiveness(w http.ResponseWriter, r *http.Request) {
	render.JSON(w, r, map[string]string{
		"Status": "OK",
	})
}
