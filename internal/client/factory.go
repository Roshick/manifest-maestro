package client

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/rehttp"
	"github.com/Roshick/go-autumn-web/auth"
	"github.com/Roshick/go-autumn-web/logging"
	"github.com/Roshick/go-autumn-web/metrics"
	"github.com/Roshick/go-autumn-web/resiliency"
	"github.com/Roshick/go-autumn-web/tracing"
	"github.com/gofri/go-github-pagination/githubpagination"
	"github.com/gofri/go-github-ratelimit/v2/github_ratelimit"
	"github.com/google/go-github/v93/github"
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

	httpClient := &http.Client{
		Transport: rt,
		Timeout:   opts.Timeout,
	}
	if opts.BasicAuthOptions != nil {
		httpClient.CheckRedirect = rejectCrossOriginRedirects
	}
	return httpClient, nil
}

const maxRedirects = 10

// rejectCrossOriginRedirects prevents leaking basic auth credentials: the basic auth transport adds them to
// every request, including redirected ones, so net/http's stripping of sensitive headers does not apply.
func rejectCrossOriginRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	if !strings.EqualFold(req.URL.Scheme, via[0].URL.Scheme) || !strings.EqualFold(req.URL.Host, via[0].URL.Host) {
		return errors.New("refusing to follow cross-origin redirect of authenticated client")
	}
	return nil
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
func (f *Factory) NewGitHubClient(
	appID int64,
	appInstallationID int64,
	privateKey *rsa.PrivateKey,
	opts *GitHubClientOptions,
) (*github.Client, error) {
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

	return github.NewClient(github.WithHTTPClient(&http.Client{
		Transport: rt,
		Timeout:   opts.Timeout,
	}))
}
