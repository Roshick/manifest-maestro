package cache

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/repository/helmremote"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/targz"
	aulogging "github.com/StephanHCB/go-autumn-logging"
)

type HelmChartRemote interface {
	GetChart(context.Context, url.URL) ([]byte, error)
}

type HelmChartCache struct {
	helmRemote HelmChartRemote
	indexCache *HelmIndexCache
	cache      cache.Cache[[]byte]
}

func NewHelmChartCache(helmRemote HelmChartRemote, indexCache *HelmIndexCache, cache cache.Cache[[]byte]) *HelmChartCache {
	return &HelmChartCache{
		helmRemote: helmRemote,
		indexCache: indexCache,
		cache:      cache,
	}
}

func (c *HelmChartCache) RetrieveChart(ctx context.Context, chartReference openapi.HelmChartRepositoryChartReference) ([]byte, error) {
	if strings.HasPrefix(chartReference.RepositoryURL, "https://") || strings.HasPrefix(chartReference.RepositoryURL, "http://") {
		return c.retrieveHelmChartViaHTTP(ctx, chartReference)
	} else if strings.HasPrefix(chartReference.RepositoryURL, "oci://") {
		return c.retrieveHelmChartViaOCI(ctx, chartReference)
	}
	return nil, NewInvalidHelmRepositoryURLError(chartReference.RepositoryURL)
}

func (c *HelmChartCache) RetrieveChartToFileSystem(ctx context.Context, chartReference openapi.HelmChartRepositoryChartReference, fileSystem *filesystem.FileSystem) error {
	tarball, err := c.RetrieveChart(ctx, chartReference)
	if err != nil {
		return err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(tarball), fileSystem.Root); err != nil {
		return err
	}
	return nil
}

func (c *HelmChartCache) retrieveHelmChartViaOCI(ctx context.Context, chartReference openapi.HelmChartRepositoryChartReference) ([]byte, error) {
	chartURL, err := url.JoinPath(chartReference.RepositoryURL, chartReference.ChartName)
	if err != nil {
		return nil, fmt.Errorf("failed to construct chart url: %w", err)
	}

	parsedURL, err := url.Parse(chartURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chart url '%s': %w", chartURL, err)
	}
	if parsedURL == nil {
		return nil, fmt.Errorf("failed to parse chart url '%s': parsed url is nil", chartURL)
	}

	if chartReference.ChartVersion != nil {
		parsedURL.Path = fmt.Sprintf("%s:%s", parsedURL.Path, *chartReference.ChartVersion)
	}

	cacheKey := parsedURL.String()
	cached, err := c.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm chart with key '%s'", cacheKey)
		return *cached, nil
	}

	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm chart with key '%s', retrieving from remote", cacheKey)
	chartBytes, err := c.helmRemote.GetChart(ctx, *parsedURL)
	if err != nil {
		return nil, err
	}

	if err = c.cache.Set(ctx, cacheKey, chartBytes, 5*time.Minute); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm chart with key '%s'", cacheKey)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm chart with key '%s'", cacheKey)
	}
	return chartBytes, nil
}

func (c *HelmChartCache) retrieveHelmChartViaHTTP(ctx context.Context, chartReference openapi.HelmChartRepositoryChartReference) ([]byte, error) {
	index, err := c.indexCache.RetrieveIndex(ctx, chartReference.RepositoryURL)
	if err != nil {
		return nil, err
	}

	chartVersion := utils.DefaultIfNil(chartReference.ChartVersion, "")
	chartEntry, err := index.Get(chartReference.ChartName, chartVersion)
	if err != nil || len(chartEntry.URLs) == 0 {
		return nil, helmremote.NewRepositoryChartNotFoundError(chartReference.RepositoryURL, chartReference.ChartName, chartVersion)
	}

	chartURL := chartEntry.URLs[0]
	// no protocol => url is relative
	if !strings.Contains(chartURL, "://") {
		chartURL, err = url.JoinPath(chartReference.RepositoryURL, chartURL)
		if err != nil {
			return nil, fmt.Errorf("failed to construct chart url: %w", err)
		}
	}

	parsedURL, err := url.Parse(chartURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse chart url '%s': %w", chartURL, err)
	}
	if parsedURL == nil {
		return nil, fmt.Errorf("failed to parse chart url '%s': parsed url is nil", chartURL)
	}

	cacheKey := fmt.Sprintf("%s|%s", parsedURL.String(), chartEntry.Digest)
	cached, err := c.cache.Get(ctx, cacheKey)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm chart with key '%s'", cacheKey)
		return *cached, nil
	}

	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm chart with key '%s', retrieving from remote", cacheKey)
	chartBytes, err := c.helmRemote.GetChart(ctx, *parsedURL)
	if err != nil {
		return nil, err
	}

	if err = c.cache.Set(ctx, cacheKey, chartBytes, 15*time.Minute); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm chart with key '%s'", cacheKey)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm chart with key '%s'", cacheKey)
	}
	return chartBytes, nil
}
