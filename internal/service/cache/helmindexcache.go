package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

type HelmIndexRemote interface {
	GetIndex(context.Context, url.URL) ([]byte, error)
}

type HelmIndexCache struct {
	helmRemote HelmIndexRemote
	cache      cache.Cache[[]byte]
}

func NewHelmIndexCache(helmRemote HelmIndexRemote, cache cache.Cache[[]byte]) *HelmIndexCache {
	return &HelmIndexCache{
		helmRemote: helmRemote,
		cache:      cache,
	}
}

func (c *HelmIndexCache) RetrieveIndex(ctx context.Context, repositoryURL string) (*repo.IndexFile, error) {
	parsedURL, err := url.Parse(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository url '%s': %w", repositoryURL, err)
	}
	if parsedURL == nil {
		return nil, fmt.Errorf("failed to parse repository url '%s': parsed url is nil", repositoryURL)
	}

	cacheKey := parsedURL.String()
	cached, err := c.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm repository index with key '%s'", repositoryURL)
		return c.parseIndex(*cached)
	}

	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm repository index with key '%s', retrieving from remote", repositoryURL)
	indexBytes, err := c.helmRemote.GetIndex(ctx, *parsedURL)
	if err != nil {
		return nil, err
	}

	if err = c.cache.Set(ctx, cacheKey, indexBytes, 15*time.Minute); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm repository index with key '%s'", repositoryURL)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm repository index with key '%s'", repositoryURL)
	}

	return c.parseIndex(indexBytes)
}

func (c *HelmIndexCache) parseIndex(data []byte) (*repo.IndexFile, error) {
	index := &repo.IndexFile{}

	if len(data) == 0 {
		return nil, repo.ErrEmptyIndexYaml
	}
	if json.Valid(data) {
		if err := json.Unmarshal(data, index); err != nil {
			return nil, err
		}
	}
	if err := yaml.UnmarshalStrict(data, index); err != nil {
		return nil, err
	}
	if index.APIVersion == "" {
		return index, repo.ErrNoAPIVersion
	}

	for _, cvs := range index.Entries {
		for idx := len(cvs) - 1; idx >= 0; idx-- {
			if cvs[idx] == nil {
				continue
			}
			if cvs[idx].APIVersion == "" {
				cvs[idx].APIVersion = chart.APIVersionV1
			}
			if err := cvs[idx].Validate(); err != nil {
				cvs = append(cvs[:idx], cvs[idx+1:]...)
			}
		}
	}
	index.SortEntries()

	return index, nil
}
