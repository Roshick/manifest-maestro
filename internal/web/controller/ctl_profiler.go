package controller

import (
	"context"
	"net/http"

	"github.com/Roshick/go-autumn-web/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/utils"
)

// NewProfilerController creates the controller for the profiler, which is only accessible with valid
// credentials. Without authFns, all requests are rejected.
func NewProfilerController(authFns []auth.AuthorizationFn) *ProfilerController {
	return &ProfilerController{
		authFns: authFns,
	}
}

type ProfilerController struct {
	authFns []auth.AuthorizationFn
}

func (c *ProfilerController) WireUp(_ context.Context, r chi.Router) {
	requireAuth := auth.NewAuthorizationMiddleware(&auth.AuthorizationMiddlewareOptions{
		AuthorizationFns: c.authFns,
		ErrorResponse: &APIError{StatusCode: http.StatusUnauthorized, Error: openapi.Error{
			Title: utils.Ptr("Unauthorized"),
		}},
	})

	r.Group(func(r chi.Router) {
		r.With(requireAuth).Mount("/debug", middleware.Profiler())
	})
}
