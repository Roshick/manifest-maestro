package helmremote

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Roshick/manifest-maestro/internal/config"
)

type HelmRemote struct {
	hostProviders config.HelmHostProviders
}

func New(hostProviders config.HelmHostProviders) *HelmRemote {
	return &HelmRemote{hostProviders: hostProviders}
}

func (r *HelmRemote) GetIndex(_ context.Context, repositoryURL url.URL) ([]byte, error) {
	if repositoryURL.Scheme != "http" && repositoryURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", repositoryURL.Scheme)
	}

	if !strings.HasSuffix(repositoryURL.Path, "/index.yaml") {
		repositoryURL.Path = filepath.Join(repositoryURL.Path, "index.yaml")
	}

	providers, ok := r.hostProviders[repositoryURL.Host]
	if !ok {
		return nil, fmt.Errorf("no providers configured for host: %s", repositoryURL.Host)
	}

	urlGetter, err := providers.ByScheme(repositoryURL.Scheme)
	if err != nil {
		return nil, err
	}

	chartBuffer, err := urlGetter.Get(repositoryURL.String())
	if err != nil {
		return nil, err
	}

	return chartBuffer.Bytes(), nil
}

func (r *HelmRemote) GetChart(_ context.Context, chartURL url.URL) ([]byte, error) {
	providers, ok := r.hostProviders[chartURL.Host]
	if !ok {
		return nil, fmt.Errorf("no providers configured for host: %s", chartURL.Host)
	}

	urlGetter, err := providers.ByScheme(chartURL.Scheme)
	if err != nil {
		return nil, err
	}

	chartBuffer, err := urlGetter.Get(chartURL.String())
	if err != nil {
		return nil, err
	}

	return chartBuffer.Bytes(), nil
}

func (r *HelmRemote) Write(_ []byte) (int, error) {
	return 0, nil
}
