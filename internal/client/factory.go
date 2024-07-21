package client

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/Roshick/go-autumn-web/auth"
	"github.com/Roshick/go-autumn-web/logging"
	"github.com/Roshick/go-autumn-web/metrics"
	"github.com/Roshick/go-autumn-web/resiliency"
	"github.com/Roshick/go-autumn-web/tracing"
	"github.com/gofri/go-github-pagination/githubpagination"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v78/github"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Factory struct {
}

func NewFactory() *Factory {
	return &Factory{}
}

type BasicAuthOptions struct {
	Username string
	Password string
}

type HTTPClientOptions struct {
	*BasicAuthOptions
	Timeout time.Duration
}

//nolint:mnd // magic numbers are used for client configuration
func DefaultHTTPClientOptions() *HTTPClientOptions {
	return &HTTPClientOptions{
		Timeout: 30 * time.Second,
	}
}

//nolint:mnd // magic numbers are used for client configuration
func (f *Factory) NewHTTPClient(clientName string, opts *HTTPClientOptions) (*http.Client, error) {
	if opts == nil {
		opts = DefaultHTTPClientOptions()
	}

	// RoundTrippers are called bottom to top
	rt := http.DefaultTransport
	// inject basic auth transport
	if opts.BasicAuthOptions != nil {
		rt = auth.NewBasicAuthTransport(rt, opts.Username, opts.Password, nil)
	}
	// record metrics for every retry
	rt = metrics.NewRequestMetricsTransport(rt, clientName, nil)
	// log every retry
	rt = logging.NewRequestLoggerTransport(rt, nil)
	// retry
	retryFn := rehttp.RetryAll(rehttp.RetryStatusInterval(500, 600), rehttp.RetryMaxRetries(3))
	delayFn := rehttp.ExpJitterDelay(1*time.Second, 10*time.Second)
	rt = rehttp.NewTransport(rt, retryFn, delayFn)
	// instrument tracing before retry
	rt = otelhttp.NewTransport(rt, otelhttp.WithServerName(clientName))
	rt = tracing.NewRequestIDHeaderTransport(rt, nil)

	return &http.Client{
		Transport: rt,
		Timeout:   opts.Timeout,
	}, nil
}

type GitHubClientOptions struct {
	Timeout time.Duration
}

//nolint:mnd // magic numbers are used for client configuration
func DefaultGitHubClientOptions() *GitHubClientOptions {
	return &GitHubClientOptions{
		Timeout: 30 * time.Second,
	}
}

//nolint:mnd // magic numbers are used for client configuration
func (f *Factory) NewGitHubClient(appID int64, appInstallationID int64, privateKey *rsa.PrivateKey, opts *GitHubClientOptions) (*github.Client, error) {
	if opts == nil {
		opts = DefaultGitHubClientOptions()
	}

	clientName := fmt.Sprintf("github-%d-%d", appID, appInstallationID)

	// RoundTrippers are called bottom to top
	rt := http.DefaultTransport
	// rate limit every retry
	rt = github_ratelimit.NewSecondaryLimiter(rt)
	rt = github_ratelimit.NewPrimaryLimiter(rt)
	// update auth for every retry
	rt = NewGitHubAuthTransport(rt, appID, appInstallationID, privateKey)
	rt = resiliency.NewCircuitBreakerTransport(rt, nil)
	// record metrics for every retry
	rt = metrics.NewRequestMetricsTransport(rt, clientName, nil)
	// log every retry
	rt = logging.NewRequestLoggerTransport(rt, nil)
	// retry
	retryFn := rehttp.RetryAll(rehttp.RetryStatusInterval(500, 600), rehttp.RetryMaxRetries(3))
	delayFn := rehttp.ExpJitterDelay(1*time.Second, 10*time.Second)
	rt = rehttp.NewTransport(rt, retryFn, delayFn)
	// resolve pagination before retry
	rt = githubpagination.New(rt)
	// instrument tracing before retry
	rt = otelhttp.NewTransport(rt, otelhttp.WithServerName(clientName))
	rt = tracing.NewRequestIDHeaderTransport(rt, nil)

	return github.NewClient(&http.Client{
		Transport: rt,
		Timeout:   opts.Timeout,
	}), nil
}
