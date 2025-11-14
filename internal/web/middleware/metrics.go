package middleware

import (
	"context"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/google/go-github/v78/github"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"net/http"
	"time"
)

// RecordRequestMetrics //

type RecordGitHubRateLimitMetricsOptions struct {
	Client            *github.Client
	AppID             int64
	AppInstallationID int64
}

func RecordGitHubRateLimitMetrics(options RecordGitHubRateLimitMetricsOptions) func(next http.Handler) http.Handler {
	meter := otel.GetMeterProvider().Meter("")

	apiRequestsLimit, _ := meter.Int64Gauge(
		"github.api_requests.limit",
		metric.WithDescription("Maximum number of API requests that can be made per hour."),
	)
	apiRequestsRemaining, _ := meter.Int64Gauge(
		"github.api_requests.remaining",
		metric.WithDescription("Number of API requests remaining in the current rate limit window."),
	)
	apiRequestsResetTimestamp, _ := meter.Int64Gauge(
		"github.api_requests.reset_timestamp",
		metric.WithDescription("Unix timestamp (in seconds) of when the next rate limit reset will occur."),
	)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()

			timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			if limits, _, err := options.Client.RateLimit.Get(timeoutCtx); err != nil {
				aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to update rate limits for github application %d", options.AppID)
			} else {
				apiRequestsLimit.Record(ctx, int64(limits.Core.Limit), metric.WithAttributes(
					attribute.Int64("github_app_id", options.AppID),
					attribute.Int64("github_app_installation_id", options.AppInstallationID),
					attribute.String("github_resource", "core"),
				))
				apiRequestsRemaining.Record(ctx, int64(limits.Core.Remaining), metric.WithAttributes(
					attribute.Int64("github_app_id", options.AppID),
					attribute.Int64("github_app_installation_id", options.AppInstallationID),
					attribute.String("github_resource", "core"),
				))
				apiRequestsResetTimestamp.Record(ctx, limits.Core.Reset.Unix(), metric.WithAttributes(
					attribute.Int64("github_app_id", options.AppID),
					attribute.Int64("github_app_installation_id", options.AppInstallationID),
					attribute.String("github_resource", "core"),
				))
			}

			next.ServeHTTP(w, req)
		}
		return http.HandlerFunc(fn)
	}
}
