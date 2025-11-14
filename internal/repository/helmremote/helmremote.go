package helmremote

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"helm.sh/helm/v3/pkg/getter"
)

type HelmRemote struct {
	providers getter.Providers
}

func New(providers getter.Providers) *HelmRemote {
	return &HelmRemote{providers: providers}
}

func (r *HelmRemote) GetIndex(_ context.Context, repositoryURL url.URL) ([]byte, error) {
	if repositoryURL.Scheme != "http" && repositoryURL.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme: %s", repositoryURL.Scheme)
	}

	if !strings.HasSuffix(repositoryURL.Path, "/index.yaml") {
		repositoryURL.Path = filepath.Join(repositoryURL.Path, "index.yaml")
	}

	urlGetter, err := r.providers.ByScheme(repositoryURL.Scheme)
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
	urlGetter, err := r.providers.ByScheme(chartURL.Scheme)
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
