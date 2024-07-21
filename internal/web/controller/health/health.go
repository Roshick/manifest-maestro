package health

import (
	"context"
	"net/http"

	"github.com/Roshick/manifest-maestro/internal/web/header"
	"github.com/Roshick/manifest-maestro/internal/web/mimetype"

	"github.com/Roshick/manifest-maestro/internal/web/helper"
	"github.com/Roshick/manifest-maestro/pkg/api"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/go-chi/chi/v5"
)

func NewController() *Controller {
	return &Controller{}
}

type Controller struct{}

func (c *Controller) WireUp(_ context.Context, router chi.Router) {
	router.Route("/health", func(router chi.Router) {
		router.Get("/readiness", c.Health)
		router.Get("/liveness", c.Health)
	})
}

func (c *Controller) Health(w http.ResponseWriter, r *http.Request) {
	response := api.HealthResponse{
		Status: commonutils.Ptr("UP"),
	}
	w.Header().Set(header.ContentType, mimetype.ApplicationJSON)
	helper.WriteJSON(r.Context(), w, response)
}
