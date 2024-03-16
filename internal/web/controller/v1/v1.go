package v1controller

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/web/helper"

	"github.com/Roshick/manifest-maestro/internal/service/manifestrenderer"

	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/go-chi/chi/v5"
)

const (
	repositoryURLPathParam = "repositoryURL"
	pathPathParam          = "repositoryPath"
	chartNamePathParam     = "chartName"

	atQueryParam              = "at"
	versionQueryParam         = "version"
	includeMetadataQueryParam = "include-metadata"
)

type Controller struct {
	clock Clock

	helm             *helm.Helm
	manifestRenderer *manifestrenderer.ManifestRenderer
}

type Clock interface {
	Now() time.Time
}

func New(
	clock Clock,
	helm *helm.Helm,
	manifestRenderer *manifestrenderer.ManifestRenderer,
) *Controller {
	return &Controller{
		clock:            clock,
		helm:             helm,
		manifestRenderer: manifestRenderer,
	}
}

func (c *Controller) WireUp(_ context.Context, router chi.Router) {
	noopFunc := func(w http.ResponseWriter, r *http.Request) {}

	router.Route("/rest/api/v1", func(router chi.Router) {
		router.Route("/sources", func(router chi.Router) {
			router.Get("/", noopFunc)
			router.Route(fmt.Sprintf("/git-repo/{%s}", repositoryURLPathParam), func(router chi.Router) {
				router.Get("/", noopFunc)
				router.Route("/charts", func(router chi.Router) {
					router.Get("/", noopFunc)
					router.Route(fmt.Sprintf("/{%s}", pathPathParam), func(router chi.Router) {
						router.Get("/", noopFunc)
						router.Get("/defaultValues", noopFunc)
						router.Route("/actions", func(router chi.Router) {
							router.Post("/render", c.RenderHelmFromGitRepository)
						})
					})
				})
				router.Route("/kustomizations", func(router chi.Router) {
					router.Get("/", noopFunc)
					router.Route(fmt.Sprintf("/${%s}", pathPathParam), func(router chi.Router) {
						router.Get("/", noopFunc)
						router.Route("/actions", func(router chi.Router) {
							router.Post("/render", c.RenderKustomizeFromGitRepository)
						})
					})
				})
			})
			router.Route(fmt.Sprintf("/chart-repo/{%s}", repositoryURLPathParam), func(router chi.Router) {
				router.Get("/", noopFunc)
				router.Route("/charts", func(router chi.Router) {
					router.Get("/", noopFunc)
					router.Route(fmt.Sprintf("/{%s}", chartNamePathParam), func(router chi.Router) {
						router.Get("/", noopFunc)
						router.Get("/defaultValues", noopFunc)
						router.Route("/actions", func(router chi.Router) {
							router.Post("/render", c.RenderHelmFromChartRepository)
						})
					})
				})
			})
		})
	})
}

func (c *Controller) RenderHelmFromGitRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := apimodel.RenderManifestsViaHelmFromGitRepositoryAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	repositoryURL, err := helper.StringPathParam(r, repositoryURLPathParam)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	path, err := helper.StringPathParam(r, pathPathParam)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	path = strings.TrimPrefix(path, "/")

	at, err := helper.StringQueryParam(r, atQueryParam, "HEAD")
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	includeMetadata, err := helper.BooleanQueryParam(r, includeMetadataQueryParam, false)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	manifests, metadata, err := c.manifestRenderer.RenderHelmFromGitRepository(ctx, apimodel.GitSource{
		Url:       repositoryURL,
		Path:      path,
		Reference: at,
	}, action.Parameters)
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

func (c *Controller) RenderHelmFromChartRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := apimodel.RenderManifestsViaHelmFromChartRepositoryAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	repositoryURL, err := helper.StringPathParam(r, repositoryURLPathParam)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	chartName, err := helper.StringPathParam(r, chartNamePathParam)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	version, err := helper.StringQueryParam(r, versionQueryParam, "")
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	includeMetadata, err := helper.BooleanQueryParam(r, includeMetadataQueryParam, false)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	manifests, metadata, err := c.manifestRenderer.RenderHelmFromChartRepository(ctx, apimodel.ChartReference{
		RepositoryURL: repositoryURL,
		Name:          chartName,
		Version:       &version,
	}, action.Parameters)
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

func (c *Controller) RenderKustomizeFromGitRepository(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	action := apimodel.RenderManifestsViaKustomizeFromGitRepositoryAction{}
	if err := helper.ParseBody(ctx, r.Body, &action); err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	repositoryURL, err := helper.StringPathParam(r, repositoryURLPathParam)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	path, err := helper.StringPathParam(r, pathPathParam)
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}
	path = strings.TrimPrefix(path, "/")

	at, err := helper.StringQueryParam(r, atQueryParam, "HEAD")
	if err != nil {
		helper.BadRequestErrorHandler(ctx, w, r, err.Error(), c.clock.Now())
		return
	}

	manifests, err := c.manifestRenderer.RenderKustomizeFromGitRepository(ctx, apimodel.GitSource{
		Url:       repositoryURL,
		Path:      path,
		Reference: at,
	}, action.Parameters)
	if err != nil {
		helper.HandleError(ctx, w, r, err, c.clock.Now())
		return
	}

	helper.Success(ctx, w, r, apimodel.RenderManifestsViaKustomizeActionResponse{
		Manifests: manifests,
	})
}
