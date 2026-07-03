package cache

import (
	"context"
	"testing"

	"github.com/Roshick/manifest-maestro/test/mock/cachemock"
	"github.com/Roshick/manifest-maestro/test/mock/helmremotemock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validIndexYAML() []byte {
	return []byte(`apiVersion: v1
entries:
  mychart:
    - name: mychart
      version: 0.1.0
      apiVersion: v2
      urls:
        - https://example.com/charts/mychart-0.1.0.tgz
    - name: mychart
      version: 0.2.0
      apiVersion: v2
      urls:
        - https://example.com/charts/mychart-0.2.0.tgz
  otherchart:
    - name: otherchart
      version: 1.0.0
      apiVersion: v2
      urls:
        - https://example.com/charts/otherchart-1.0.0.tgz
`)
}

func validIndexJSON() []byte {
	return []byte(`{
	"apiVersion": "v1",
	"entries": {
		"jsonchart": [
			{
				"name": "jsonchart",
				"version": "1.0.0",
				"apiVersion": "v2",
				"urls": ["https://example.com/charts/jsonchart-1.0.0.tgz"]
			}
		]
	}
}`)
}

func TestHelmIndexCache_RetrieveIndex_CacheMiss(t *testing.T) {
	ctx := context.Background()

	indexMock := helmremotemock.NewIndexMock()
	indexMock.AddIndex("https://example.com", validIndexYAML())

	cacheMock := cachemock.New[[]byte]()
	indexCache := NewHelmIndexCache(indexMock, cacheMock)

	index, err := indexCache.RetrieveIndex(ctx, "https://example.com")
	require.NoError(t, err)
	require.NotNil(t, index)

	// Verify index was parsed correctly
	chartVersions, err := index.Get("mychart", "0.2.0")
	require.NoError(t, err)
	assert.Equal(t, "mychart", chartVersions.Name)
	assert.Equal(t, "0.2.0", chartVersions.Version)

	// Verify remote was called
	assert.Equal(t, int32(1), indexMock.GetIndexCallCount.Load())
}

func TestHelmIndexCache_RetrieveIndex_CacheHit(t *testing.T) {
	ctx := context.Background()

	indexMock := helmremotemock.NewIndexMock()
	indexMock.AddIndex("https://example.com", validIndexYAML())

	cacheMock := cachemock.New[[]byte]()
	indexCache := NewHelmIndexCache(indexMock, cacheMock)

	// First call: cache miss
	_, err := indexCache.RetrieveIndex(ctx, "https://example.com")
	require.NoError(t, err)

	// Second call: cache hit
	index, err := indexCache.RetrieveIndex(ctx, "https://example.com")
	require.NoError(t, err)
	require.NotNil(t, index)

	// Remote should only be called once
	assert.Equal(t, int32(1), indexMock.GetIndexCallCount.Load())

	// Index should still be correct from cache
	chartVersions, err := index.Get("mychart", "0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "mychart", chartVersions.Name)
}

func TestHelmIndexCache_RetrieveIndex_CacheHitReparses(t *testing.T) {
	ctx := context.Background()

	indexMock := helmremotemock.NewIndexMock()
	indexMock.AddIndex("https://example.com", validIndexYAML())

	cacheMock := cachemock.New[[]byte]()
	indexCache := NewHelmIndexCache(indexMock, cacheMock)

	// First call
	index1, err := indexCache.RetrieveIndex(ctx, "https://example.com")
	require.NoError(t, err)

	// Second call (cache hit — currently re-parses YAML every time)
	index2, err := indexCache.RetrieveIndex(ctx, "https://example.com")
	require.NoError(t, err)

	// Both should return valid data
	cv1, err := index1.Get("mychart", "0.2.0")
	require.NoError(t, err)
	cv2, err := index2.Get("mychart", "0.2.0")
	require.NoError(t, err)

	assert.Equal(t, cv1.Version, cv2.Version)
}

func TestHelmIndexCache_ParseIndex_YAML(t *testing.T) {
	indexCache := &HelmIndexCache{}

	index, err := indexCache.parseIndex(validIndexYAML())
	require.NoError(t, err)
	require.NotNil(t, index)

	assert.Equal(t, "v1", index.APIVersion)

	chartVersions, err := index.Get("mychart", "0.1.0")
	require.NoError(t, err)
	assert.Equal(t, "mychart", chartVersions.Name)
	assert.Len(t, chartVersions.URLs, 1)
}

func TestHelmIndexCache_ParseIndex_JSON(t *testing.T) {
	indexCache := &HelmIndexCache{}

	index, err := indexCache.parseIndex(validIndexJSON())
	require.NoError(t, err)
	require.NotNil(t, index)

	chartVersions, err := index.Get("jsonchart", "1.0.0")
	require.NoError(t, err)
	assert.Equal(t, "jsonchart", chartVersions.Name)
}

func TestHelmIndexCache_ParseIndex_Empty(t *testing.T) {
	indexCache := &HelmIndexCache{}

	_, err := indexCache.parseIndex([]byte{})
	assert.Error(t, err)
}

func TestHelmIndexCache_ParseIndex_MultipleCharts(t *testing.T) {
	indexCache := &HelmIndexCache{}

	index, err := indexCache.parseIndex(validIndexYAML())
	require.NoError(t, err)

	// mychart has 2 versions
	myChartVersions := index.Entries["mychart"]
	assert.Len(t, myChartVersions, 2)

	// otherchart has 1 version
	otherChartVersions := index.Entries["otherchart"]
	assert.Len(t, otherChartVersions, 1)
}

func TestHelmIndexCache_ParseIndex_FiltersInvalidAndNilEntries(t *testing.T) {
	indexCache := &HelmIndexCache{}

	indexYAML := []byte(`apiVersion: v1
entries:
  mixedchart:
    - null
    - name: mixedchart
      version: 1.0.0
      apiVersion: v2
      urls:
        - https://example.com/charts/mixedchart-1.0.0.tgz
    - version: 2.0.0
      apiVersion: v2
      urls:
        - https://example.com/charts/unnamed-2.0.0.tgz
  brokenchart:
    - null
`)

	index, err := indexCache.parseIndex(indexYAML)
	require.NoError(t, err)
	require.NotNil(t, index)

	// Only the valid version remains; nil and invalid (missing name) entries are removed
	mixedChartVersions := index.Entries["mixedchart"]
	require.Len(t, mixedChartVersions, 1)
	assert.Equal(t, "mixedchart", mixedChartVersions[0].Name)
	assert.Equal(t, "1.0.0", mixedChartVersions[0].Version)

	// A chart consisting solely of nil entries ends up empty
	assert.Empty(t, index.Entries["brokenchart"])
}

func TestHelmIndexCache_ParseIndex_DefaultsMissingAPIVersion(t *testing.T) {
	indexCache := &HelmIndexCache{}

	indexYAML := []byte(`apiVersion: v1
entries:
  legacychart:
    - name: legacychart
      version: 1.0.0
      urls:
        - https://example.com/charts/legacychart-1.0.0.tgz
`)

	index, err := indexCache.parseIndex(indexYAML)
	require.NoError(t, err)

	legacyChartVersions := index.Entries["legacychart"]
	require.Len(t, legacyChartVersions, 1)
	assert.NotEmpty(t, legacyChartVersions[0].APIVersion)
}

func TestHelmIndexCache_ParseIndex_MissingAPIVersion(t *testing.T) {
	indexCache := &HelmIndexCache{}

	_, err := indexCache.parseIndex([]byte(`entries: {}`))
	assert.Error(t, err)
}

