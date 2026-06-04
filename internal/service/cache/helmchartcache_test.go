package cache

import (
	"bytes"
	"context"
	"testing"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/targz"
	"github.com/Roshick/manifest-maestro/test/mock/cachemock"
	"github.com/Roshick/manifest-maestro/test/mock/helmremotemock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelmChartCache_RetrieveChart_InvalidScheme(t *testing.T) {
	chartMock := helmremotemock.NewChartMock()
	indexMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()

	indexCache := NewHelmIndexCache(indexMock, indexCacheMock)
	chartCache := NewHelmChartCache(chartMock, indexCache, chartCacheMock)

	_, err := chartCache.RetrieveChart(context.Background(), openapi.HelmChartRepositoryChartReference{
		RepositoryURL: "ftp://example.com/charts",
		ChartName:     "mychart",
		ChartVersion:  utils.Ptr("1.0.0"),
	})
	assert.Error(t, err)
	assert.IsType(t, &InvalidHelmRepositoryURLError{}, err)
}

func TestHelmChartCache_RetrieveChart_OCI_CacheMiss(t *testing.T) {
	ctx := context.Background()

	chartMock := helmremotemock.NewChartMock()
	chartMock.AddChart("oci://example.com/charts/mychart:1.0.0", []byte("chart-data"))

	indexMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()

	indexCache := NewHelmIndexCache(indexMock, indexCacheMock)
	chartCache := NewHelmChartCache(chartMock, indexCache, chartCacheMock)

	chartBytes, err := chartCache.RetrieveChart(ctx, openapi.HelmChartRepositoryChartReference{
		RepositoryURL: "oci://example.com/charts",
		ChartName:     "mychart",
		ChartVersion:  utils.Ptr("1.0.0"),
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("chart-data"), chartBytes)
	assert.Equal(t, int32(1), chartMock.GetChartCallCount.Load())
}

func TestHelmChartCache_RetrieveChart_OCI_CacheHit(t *testing.T) {
	ctx := context.Background()

	chartMock := helmremotemock.NewChartMock()
	chartMock.AddChart("oci://example.com/charts/mychart:1.0.0", []byte("chart-data"))

	indexMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()

	indexCache := NewHelmIndexCache(indexMock, indexCacheMock)
	chartCache := NewHelmChartCache(chartMock, indexCache, chartCacheMock)

	ref := openapi.HelmChartRepositoryChartReference{
		RepositoryURL: "oci://example.com/charts",
		ChartName:     "mychart",
		ChartVersion:  utils.Ptr("1.0.0"),
	}

	// First call: cache miss
	_, err := chartCache.RetrieveChart(ctx, ref)
	require.NoError(t, err)

	// Second call: cache hit
	chartBytes, err := chartCache.RetrieveChart(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, []byte("chart-data"), chartBytes)

	// Remote should only be called once
	assert.Equal(t, int32(1), chartMock.GetChartCallCount.Load())
}

func TestHelmChartCache_RetrieveChartToFileSystem(t *testing.T) {
	ctx := context.Background()

	// Create a valid tgz from a minimal chart
	tarball := createTestChartTarball(t)

	chartMock := helmremotemock.NewChartMock()
	chartMock.AddChart("oci://example.com/charts/mychart:1.0.0", tarball)

	indexMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()

	indexCache := NewHelmIndexCache(indexMock, indexCacheMock)
	chartCache := NewHelmChartCache(chartMock, indexCache, chartCacheMock)

	destFS := filesystem.New()
	err := chartCache.RetrieveChartToFileSystem(ctx, openapi.HelmChartRepositoryChartReference{
		RepositoryURL: "oci://example.com/charts",
		ChartName:     "mychart",
		ChartVersion:  utils.Ptr("1.0.0"),
	}, destFS)
	require.NoError(t, err)

	// Verify extracted content
	assert.True(t, destFS.Exists(destFS.Join(destFS.Root, "mychart", "Chart.yaml")))
}

func TestHelmChartCache_RetrieveChart_HTTP_CacheMiss(t *testing.T) {
	ctx := context.Background()

	indexMock := helmremotemock.NewIndexMock()
	indexMock.AddIndex("https://example.com/charts", []byte(`apiVersion: v1
entries:
  mychart:
    - name: mychart
      version: 1.0.0
      apiVersion: v2
      digest: sha256abc123
      urls:
        - https://example.com/charts/mychart-1.0.0.tgz
`))

	chartMock := helmremotemock.NewChartMock()
	chartMock.AddChart("https://example.com/charts/mychart-1.0.0.tgz", []byte("http-chart-data"))

	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()

	indexCache := NewHelmIndexCache(indexMock, indexCacheMock)
	chartCache := NewHelmChartCache(chartMock, indexCache, chartCacheMock)

	chartBytes, err := chartCache.RetrieveChart(ctx, openapi.HelmChartRepositoryChartReference{
		RepositoryURL: "https://example.com/charts",
		ChartName:     "mychart",
		ChartVersion:  utils.Ptr("1.0.0"),
	})
	require.NoError(t, err)
	assert.Equal(t, []byte("http-chart-data"), chartBytes)
}

// createTestChartTarball creates a minimal valid chart tarball.
func createTestChartTarball(t *testing.T) []byte {
	t.Helper()

	fs := filesystem.New()
	chartDir := fs.Join(fs.Root, "mychart")
	require.NoError(t, fs.MkdirAll(chartDir))

	chartYAML := `apiVersion: v2
name: mychart
version: 1.0.0
`
	f, err := fs.Create(fs.Join(chartDir, "Chart.yaml"))
	require.NoError(t, err)
	_, err = f.Write([]byte(chartYAML))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	var buf bytes.Buffer
	require.NoError(t, targz.Compress(context.Background(), fs, fs.Root, "", &buf))
	return buf.Bytes()
}
