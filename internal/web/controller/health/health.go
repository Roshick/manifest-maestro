package health

import (
	"context"
	"net/http"

	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/internal/web/helper"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/go-chi/chi/v5"
	"github.com/go-http-utils/headers"
)

func New() *Controller {
	return &Controller{}
}

type Controller struct{}

func (c *Controller) WireUp(_ context.Context, router chi.Router) {
	router.Route("/health", func(router chi.Router) {
		router.Get("/ready", c.Health)
		router.Get("/live", c.Health)
	})
}

func (c *Controller) Health(w http.ResponseWriter, r *http.Request) {
	response := apimodel.Health{
		Status: commonutils.Ptr("UP"),
	}
	r.Context().Err()
	w.Header().Set(headers.ContentType, helper.ContentTypeApplicationJSON)
	helper.WriteJSON(r.Context(), w, response)
}
