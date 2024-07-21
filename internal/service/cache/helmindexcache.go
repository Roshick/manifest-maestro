package cache

import (
	"context"
	"encoding/json"
	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
	"time"
)

type HelmIndexRemote interface {
	GetIndex(context.Context, string) ([]byte, error)
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
	cached, err := c.cache.Get(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm repository index with key '%s'", repositoryURL)
		return c.parseIndex(*cached)
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm repository index with key '%s', retrieving from remote", repositoryURL)
	return c.RefreshIndex(ctx, repositoryURL)
}

func (c *HelmIndexCache) RefreshIndex(ctx context.Context, repositoryURL string) (*repo.IndexFile, error) {
	indexBytes, err := c.helmRemote.GetIndex(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}

	if err = c.cache.Set(ctx, repositoryURL, indexBytes, 3*time.Minute); err != nil {
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
	if err := c.jsonOrYamlUnmarshal(data, index); err != nil {
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

func (c *HelmIndexCache) jsonOrYamlUnmarshal(unknownBytes []byte, obj any) error {
	if json.Valid(unknownBytes) {
		return json.Unmarshal(unknownBytes, obj)
	}
	return yaml.UnmarshalStrict(unknownBytes, obj)
}
