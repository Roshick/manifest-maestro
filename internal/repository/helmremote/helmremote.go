package helmremote

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/Roshick/manifest-maestro/internal/config"
	"oras.land/oras-go/v2"
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

	providers, ok := r.hostProviders[repositoryURL.Host]
	if !ok {
		return nil, NewMissingProviderError(repositoryURL)
	}

	urlGetter, err := providers.ByScheme(repositoryURL.Scheme)
	if err != nil {
		return nil, NewMissingProviderError(repositoryURL)
	}

	indexURL := repositoryURL
	indexURL.Path = filepath.Join(repositoryURL.Path, "index.yaml")
	chartBuffer, err := urlGetter.Get(indexURL.String())
	if err != nil {
		if strings.HasSuffix(err.Error(), "404 Not Found") {
			return nil, NewRepositoryNotFoundError2(repositoryURL)
		}
		return nil, err
	}

	return chartBuffer.Bytes(), nil
}

func (r *HelmRemote) GetChart(_ context.Context, chartURL url.URL) ([]byte, error) {
	providers, ok := r.hostProviders[chartURL.Host]
	if !ok {
		return nil, NewMissingProviderError(chartURL)
	}

	urlGetter, err := providers.ByScheme(chartURL.Scheme)
	if err != nil {
		return nil, NewMissingProviderError(chartURL)
	}

	chartBuffer, err := urlGetter.Get(chartURL.String())
	if err != nil {
		if errors.As(err, new(*oras.CopyError)) {
			return nil, NewRepositoryChartNotFoundError2(chartURL)
		}
		if strings.HasPrefix(err.Error(), "invalid reference:") {
			return nil, NewRepositoryChartNotFoundError2(chartURL)
		}
		return nil, err
	}

	return chartBuffer.Bytes(), nil
}

func (r *HelmRemote) Write(_ []byte) (int, error) {
	return 0, nil
}
