package v1

import (
	"context"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"net/http"
	"time"

	"github.com/Roshick/manifest-maestro/internal/web/helper"

	"github.com/Roshick/manifest-maestro/pkg/api"
	"github.com/go-chi/chi/v5"
)

type Controller struct {
	clock Clock

	helmChartProvider     *helm.ChartProvider
	helmChartRenderer     *helm.ChartRenderer
	kustomizationProvider *kustomize.KustomizationProvider
	kustomizationRenderer *kustomize.KustomizationRenderer
}

type Clock interface {
	Now() time.Time
}

func NewController(
	clock Clock,
	helmChartProvider *helm.ChartProvider,
	helmChartRenderer *helm.ChartRenderer,
	kustomizationProvider *kustomize.KustomizationProvider,
	kustomizationRenderer *kustomize.KustomizationRenderer,
) *Controller {
	return &Controller{
		clock:                 clock,
		helmChartProvider:     helmChartProvider,
		helmChartRenderer:     helmChartRenderer,
		kustomizationProvider: kustomizationProvider,
		kustomizationRenderer: kustomizationRenderer,
	}
}

func (c *Controller) WireUp(_ context.Context, router chi.Router) {
	router.Route("/rest/api/v1", func(router chi.Router) {
		router.Route("/helm/actions", func(router chi.Router) {
			router.Post("/list-charts", c.helmListCharts)
			router.Post("/get-chart-metadata", c.helmGetChartMetadata)
			router.Post("/render-chart", c.helmActionsRender)
		})
		router.Route("/kustomize/actions", func(router chi.Router) {
			router.Post("/render-kustomization", c.kustomizeRenderKustomization)
		})
	})
}

func (c *Controller) helmListCharts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := api.HelmListChartsAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	helmCharts, err := c.helmChartProvider.ListHelmCharts(ctx, action.Reference)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, api.HelmListChartsActionResponse{
		Items: helmCharts,
	})
}

func (c *Controller) helmGetChartMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := api.HelmGetChartMetadataAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	helmChart, err := c.helmChartProvider.GetHelmChart(ctx, action.Reference)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, api.HelmGetChartMetadataActionResponse{
		DefaultValues: helmChart.DefaultValues(),
	})
}

func (c *Controller) helmActionsRender(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := api.HelmRenderChartAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	helmChart, err := c.helmChartProvider.GetHelmChart(ctx, action.Reference)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	manifests, metadata, err := c.helmChartRenderer.Render(ctx, helmChart, action.Parameters)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, api.HelmRenderChartActionResponse{
		Manifests: manifests,
		Metadata:  metadata,
	})
}

func (c *Controller) kustomizeRenderKustomization(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := api.KustomizeRenderKustomizationAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	kustomization, err := c.kustomizationProvider.GetKustomization(ctx, action.Reference)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	manifests, err := c.kustomizationRenderer.Render(ctx, kustomization, action.Parameters)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, api.KustomizeRenderKustomizationActionResponse{
		Manifests: manifests,
	})
}
