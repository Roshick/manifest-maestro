package controller

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewProfilerController() *ProfilerController {
	return &ProfilerController{}
}

type ProfilerController struct{}

func (c *ProfilerController) WireUp(_ context.Context, r chi.Router) {
	r.Mount("/debug", middleware.Profiler())
}
