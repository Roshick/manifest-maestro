package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/Roshick/go-autumn-web/logging"
	"github.com/Roshick/go-autumn-web/validation"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/go-chi/render"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/go-chi/chi/v5"
)

type V1Controller struct {
	clock Clock

	helmChartProvider     *helm.ChartProvider
	helmChartRenderer     *helm.ChartRenderer
	kustomizationProvider *kustomize.KustomizationProvider
	kustomizationRenderer *kustomize.KustomizationRenderer
}

type Clock interface {
	Now() time.Time
}

func NewV1Controller(
	clock Clock,
	helmChartProvider *helm.ChartProvider,
	helmChartRenderer *helm.ChartRenderer,
	kustomizationProvider *kustomize.KustomizationProvider,
	kustomizationRenderer *kustomize.KustomizationRenderer,
) *V1Controller {
	return &V1Controller{
		clock:                 clock,
		helmChartProvider:     helmChartProvider,
		helmChartRenderer:     helmChartRenderer,
		kustomizationProvider: kustomizationProvider,
		kustomizationRenderer: kustomizationRenderer,
	}
}

func (c *V1Controller) WireUp(_ context.Context, r chi.Router) {
	malformedBodyOptions := &validation.ContextRequestBodyMiddlewareOptions{
		ErrorResponse: &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title: utils.Ptr("Malformed body"),
		}},
	}

	r.Group(func(r chi.Router) {
		r.Use(logging.NewRequestLoggerMiddleware(nil))
		r.Route("/rest/api/v1", func(r chi.Router) {
			r.Route("/helm/actions", func(r chi.Router) {
				r.With(validation.NewContextRequestBodyMiddleware[openapi.HelmListChartsAction](malformedBodyOptions)).
					Post("/list-charts", c.helmActionsListCharts)
				r.With(validation.NewContextRequestBodyMiddleware[openapi.HelmListChartVersionsAction](malformedBodyOptions)).
					Post("/list-charts", c.helmActionsListChartVersions)
				r.With(validation.NewContextRequestBodyMiddleware[openapi.HelmGetChartMetadataAction](malformedBodyOptions)).
					Post("/get-chart-metadata", c.helmActionsGetChartMetadata)
				r.With(validation.NewContextRequestBodyMiddleware[openapi.HelmRenderChartAction](malformedBodyOptions)).
					Post("/render-chart", c.helmActionsRenderChart)
			})
			r.Route("/kustomize/actions", func(r chi.Router) {
				r.With(validation.NewContextRequestBodyMiddleware[openapi.KustomizeRenderKustomizationAction](malformedBodyOptions)).
					Post("/render-kustomization", c.kustomizeRenderKustomization)
			})
		})
	})
}

func (c *V1Controller) helmActionsListCharts(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context()

	// action := validation.RequestBodyFromContext[openapi.HelmListChartsAction](ctx)

	if err := render.Render(w, r, &APIError{StatusCode: http.StatusInternalServerError, ErrorResponse: openapi.ErrorResponse{
		Title: utils.Ptr("Internal server error"),
	}}); err != nil {
		panic(err)
	}
}

func (c *V1Controller) helmActionsListChartVersions(w http.ResponseWriter, r *http.Request) {
	// ctx := r.Context()

	// action := validation.RequestBodyFromContext[openapi.HelmListChartVersionsAction](ctx)

	if err := render.Render(w, r, &APIError{StatusCode: http.StatusInternalServerError, ErrorResponse: openapi.ErrorResponse{
		Title: utils.Ptr("Internal server error"),
	}}); err != nil {
		panic(err)
	}
}

func (c *V1Controller) helmActionsGetChartMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := validation.RequestBodyFromContext[openapi.HelmGetChartMetadataAction](ctx)
	helmChart, err := c.helmChartProvider.GetHelmChart(ctx, action.Reference)
	if err != nil {
		handleError(ctx, w, r, err)
		return
	}

	render.JSON(w, r, openapi.HelmGetChartMetadataActionResponse{
		DefaultValues: helmChart.DefaultValues(),
	})
}

func (c *V1Controller) helmActionsRenderChart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := validation.RequestBodyFromContext[openapi.HelmRenderChartAction](ctx)

	helmChart, err := c.helmChartProvider.GetHelmChart(ctx, action.Reference)
	if err != nil {
		handleError(ctx, w, r, err)
		return
	}

	manifests, metadata, err := c.helmChartRenderer.Render(ctx, helmChart, action.Parameters)
	if err != nil {
		handleError(ctx, w, r, err)
		return
	}

	render.JSON(w, r, openapi.HelmRenderChartActionResponse{
		Manifests: manifests,
		Metadata:  metadata,
	})
}

func (c *V1Controller) kustomizeRenderKustomization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := validation.RequestBodyFromContext[openapi.KustomizeRenderKustomizationAction](ctx)

	kustomization, err := c.kustomizationProvider.GetKustomization(ctx, action.Reference)
	if err != nil {
		handleError(ctx, w, r, err)
		return
	}

	manifests, err := c.kustomizationRenderer.Render(ctx, kustomization, action.Parameters)
	if err != nil {
		handleError(ctx, w, r, err)
		return
	}

	render.JSON(w, r, openapi.KustomizeRenderKustomizationActionResponse{
		Manifests: manifests,
	})
}
