package controller

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsController struct{}

func NewMetricsController() *MetricsController {
	return &MetricsController{}
}

func (c *MetricsController) WireUp(_ context.Context, r chi.Router) {
	r.Handle("/metrics", promhttp.Handler())
}
