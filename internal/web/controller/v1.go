package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Roshick/manifest-maestro/internal/service/helm"

	"github.com/Roshick/manifest-maestro/internal/service/manifestrenderer"

	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/internal/web/controller/helper"

	"github.com/go-chi/chi/v5"
)

const (
	includeMetadataQueryParam = "include-metadata"
)

type V1 struct {
	clock Clock

	helm             *helm.Helm
	manifestRenderer *manifestrenderer.ManifestRenderer
}

type Clock interface {
	Now() time.Time
}

func NewV1(
	clock Clock,
	helm *helm.Helm,
	manifestRenderer *manifestrenderer.ManifestRenderer,
) *V1 {
	return &V1{
		clock:            clock,
		helm:             helm,
		manifestRenderer: manifestRenderer,
	}
}

func (c *V1) WireUp(_ context.Context, router chi.Router) {
	base := "/rest/api/v1"
	router.Post(fmt.Sprintf("%s/tools/helm/caches/index/actions/refresh", base),
		c.RefreshHelmIndexCache)
	router.Post(fmt.Sprintf("%s/tools/helm/sources/git-repository/actions/render", base),
		c.RenderHelmFromGitRepository)
	router.Post(fmt.Sprintf("%s/tools/helm/sources/chart-repository/actions/render", base),
		c.RenderHelmFromChartRepository)
	router.Post(fmt.Sprintf("%s/tools/kustomize/sources/git-repository/actions/render", base),
		c.RenderKustomizeFromGitRepository)
}

func (c *V1) RefreshHelmIndexCache(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := c.helm.RefreshCachedIndexes(ctx); err != nil {
		// ToDo
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, nil)
}

func (c *V1) RenderHelmFromGitRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := apimodel.RenderManifestsViaHelmFromGitRepositoryAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	includeMetadata, err := helper.BooleanQueryParam(r, includeMetadataQueryParam, false)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	manifests, metadata, err := c.manifestRenderer.RenderHelmFromGitRepository(ctx, action.GitSource, action.Parameters)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	var metadataPtr *apimodel.HelmRenderMetadata
	if includeMetadata {
		metadataPtr = metadata
	}
	helper.Success(ctx, w, r, apimodel.RenderManifestsViaHelmActionResponse{
		Manifests: manifests,
		Metadata:  metadataPtr,
	})
}

func (c *V1) RenderHelmFromChartRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := apimodel.RenderManifestsViaHelmFromChartRepositoryAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	includeMetadata, err := helper.BooleanQueryParam(r, includeMetadataQueryParam, false)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	manifests, metadata, err := c.manifestRenderer.RenderHelmFromChartRepository(ctx, action.ChartReference, action.Parameters)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	var metadataPtr *apimodel.HelmRenderMetadata
	if includeMetadata {
		metadataPtr = metadata
	}
	helper.Success(ctx, w, r, apimodel.RenderManifestsViaHelmActionResponse{
		Manifests: manifests,
		Metadata:  metadataPtr,
	})
}

func (c *V1) RenderKustomizeFromGitRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := apimodel.RenderManifestsViaKustomizeFromGitRepositoryAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	manifests, err := c.manifestRenderer.RenderKustomizeFromGitRepository(ctx, action.GitSource, action.Parameters)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, apimodel.RenderManifestsViaKustomizeActionResponse{
		Manifests: manifests,
	})
}
