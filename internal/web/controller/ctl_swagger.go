package controller

import (
	"context"
	"net/http"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/go-chi/chi/v5"
	swagger "github.com/swaggo/http-swagger/v2"
)

func NewSwaggerController() *SwaggerController {
	return &SwaggerController{}
}

type SwaggerController struct {
}

func (c *SwaggerController) WireUp(_ context.Context, r chi.Router) {
	r.Handle("/index.html", http.RedirectHandler("/swagger-ui/index.html", http.StatusPermanentRedirect))
	r.Route("/swagger-ui", func(r chi.Router) {
		r.Handle("/api/*", http.StripPrefix("/swagger-ui/", http.FileServer(http.FS(openapi.APIFs))))
		r.Get("/*", swagger.Handler(swagger.URL("api/openapi.yaml")))
	})
}
