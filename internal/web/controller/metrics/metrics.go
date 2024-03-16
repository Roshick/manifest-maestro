package metrics

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func New() *Controller {
	return &Controller{}
}

type Controller struct{}

func (c *Controller) WireUp(_ context.Context, router chi.Router) {
	router.Handle("/metrics", promhttp.Handler())
}
