package cache

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/pkg/api"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/targz"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"helm.sh/helm/v3/pkg/repo"
	"strings"
	"time"
)

type HelmChartRemote interface {
	GetChart(context.Context, string) ([]byte, error)
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

func (c *HelmChartCache) RetrieveChart(ctx context.Context, reference api.HelmChartRepositoryChartReference) ([]byte, error) {
	index, err := c.indexCache.RetrieveIndex(ctx, reference.RepositoryURL)
	if err != nil {
		return nil, err
	}

	// ToDo: Maybe retry with refreshed index cache on error or empty urls
	chartVersion, err := index.Get(reference.ChartName, commonutils.DefaultIfNil(reference.ChartVersion, ""))
	if err != nil {
		return nil, err
	}
	if len(chartVersion.URLs) == 0 {
		return nil, fmt.Errorf("failed to find downloadable chart version in index")
	}

	key := c.cacheKey(reference.RepositoryURL, chartVersion.Name, chartVersion.Version, chartVersion.Digest)
	cached, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm chart with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm chart with key '%s', retrieving from remote", key)
	return c.RefreshChart(ctx, reference.RepositoryURL, chartVersion)
}

func (c *HelmChartCache) RetrieveChartToFileSystem(ctx context.Context, reference api.HelmChartRepositoryChartReference, fileSystem *filesystem.FileSystem) error {
	tarball, err := c.RetrieveChart(ctx, reference)
	if err != nil {
		return err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(tarball), fileSystem.Root); err != nil {
		return err
	}
	return nil
}

func (c *HelmChartCache) RefreshChart(ctx context.Context, repositoryURL string, chartVersion *repo.ChartVersion) ([]byte, error) {
	chartURL := chartVersion.URLs[0]
	// no protocol => url is relative
	if !strings.Contains(chartURL, "://") {
		chartURL = fmt.Sprintf("%s/%s", repositoryURL, chartURL)
	}
	chartBytes, err := c.helmRemote.GetChart(ctx, chartURL)
	if err != nil {
		return nil, err
	}

	key := c.cacheKey(repositoryURL, chartVersion.Name, chartVersion.Version, chartVersion.Digest)
	if err = c.cache.Set(ctx, key, chartBytes, 12*time.Hour); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm chart with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm chart with key '%s'", key)
	}

	return chartBytes, nil
}

func (c *HelmChartCache) cacheKey(repositoryURL string, chartName string, version string, chartDigest string) string {
	return fmt.Sprintf("%s|%s|%s|%s", repositoryURL, chartName, version, chartDigest)
}
