package controller

import (
	"context"

	"github.com/Roshick/manifest-maestro/internal/web/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/go-github/v80/github"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsController struct {
	gitHubClient            *github.Client
	gitHubAppID             int64
	gitHubAppInstallationID int64
}

func NewMetricsController(
	gitHubClient *github.Client,
	gitHubAppID int64,
	gitHubAppInstallationID int64,
) *MetricsController {
	return &MetricsController{
		gitHubClient:            gitHubClient,
		gitHubAppID:             gitHubAppID,
		gitHubAppInstallationID: gitHubAppInstallationID,
	}
}

func (c *MetricsController) WireUp(_ context.Context, r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Use(middleware.RecordGitHubRateLimitMetrics(middleware.RecordGitHubRateLimitMetricsOptions{
			Client:            c.gitHubClient,
			AppID:             c.gitHubAppID,
			AppInstallationID: c.gitHubAppInstallationID,
		}))
		r.Handle("/metrics", promhttp.Handler())
	})
}
