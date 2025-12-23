package controller

import (
	"context"
	"errors"
	"net/http"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/repository/git"
	"github.com/Roshick/manifest-maestro/internal/repository/helmremote"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/internal/utils"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-chi/render"
)

type APIError struct {
	openapi.ErrorResponse
	StatusCode int `json:"-"`
}

func (e *APIError) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, e.StatusCode)
	return nil
}

func handleError(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
	if innerErr := handleErrorWrapped(ctx, w, r, err); innerErr != nil {
		panic(innerErr)
	}
}

func handleErrorWrapped(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) error {
	switch {
	case errors.As(err, new(*helmremote.RepositoryNotFoundError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Helm repository not found"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*helmremote.RepositoryChartNotFoundError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Helm repository chart not found"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*helmremote.MissingProviderError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Helm repository provider missing"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*cache.InvalidHelmRepositoryURLError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Helm repository URL invalid"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*git.RepositoryNotFoundError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Git repository not found"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*git.RepositoryReferenceNotFoundError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Git repository reference not found"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*helm.ChartReferenceInvalidError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Helm chart reference invalid"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*helm.ChartBuildError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Failed to build Helm chart"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*helm.ChartRenderError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Failed to render Helm chart"),
			Detail: utils.Ptr(err.Error()),
		}})
	case errors.As(err, new(*kustomize.KustomizationRenderError)):
		return render.Render(w, r, &APIError{StatusCode: http.StatusBadRequest, ErrorResponse: openapi.ErrorResponse{
			Title:  utils.Ptr("Failed to render Kustomize kustomization"),
			Detail: utils.Ptr(err.Error()),
		}})
	default:
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Printf("unhandled error occurred")
		return render.Render(w, r, &APIError{StatusCode: http.StatusInternalServerError, ErrorResponse: openapi.ErrorResponse{
			Title: utils.Ptr("Internal server error"),
		}})
	}
}
