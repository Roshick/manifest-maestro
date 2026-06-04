package helm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/targz"
	"github.com/Roshick/manifest-maestro/test/mock/cachemock"
	"github.com/Roshick/manifest-maestro/test/mock/gitmock"
	"github.com/Roshick/manifest-maestro/test/mock/helmremotemock"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupChartProvider(t *testing.T) (*ChartProvider, *helmremotemock.ChartMock, *gitmock.Mock) {
	t.Helper()

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, _ string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		}).
		WithCloneCommit(func(_ context.Context, _ string, _ string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir("../../../test/resources/mocks/git-repositories/test")
		})

	gitCacheMock := cachemock.New[[]byte]()
	gitRepoCache := cache.NewGitRepositoryCache(gitMock, gitCacheMock)

	chartRemoteMock := helmremotemock.NewChartMock()
	indexRemoteMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()

	indexCache := cache.NewHelmIndexCache(indexRemoteMock, indexCacheMock)
	helmChartCache := cache.NewHelmChartCache(chartRemoteMock, indexCache, chartCacheMock)

	provider := NewChartProvider(helmChartCache, gitRepoCache)
	return provider, chartRemoteMock, gitMock
}

func TestChartProvider_GetHelmChart_InvalidReference(t *testing.T) {
	provider, _, _ := setupChartProvider(t)

	_, err := provider.GetHelmChart(context.Background(), openapi.HelmChartReference{})
	assert.Error(t, err)
	assert.IsType(t, &ChartReferenceInvalidError{}, err)
}

func TestChartProvider_GetHelmChart_GitReference_NoDependencies(t *testing.T) {
	ctx := context.Background()

	tmpDir := t.TempDir()
	writeChartToDir(t, tmpDir, "testchart", "0.1.0", nil)

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, _ string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		}).
		WithCloneCommit(func(_ context.Context, _ string, _ string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir(tmpDir)
		})

	gitCacheMock := cachemock.New[[]byte]()
	gitRepoCache := cache.NewGitRepositoryCache(gitMock, gitCacheMock)

	chartRemoteMock := helmremotemock.NewChartMock()
	indexRemoteMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()
	indexCache := cache.NewHelmIndexCache(indexRemoteMock, indexCacheMock)
	helmChartCache := cache.NewHelmChartCache(chartRemoteMock, indexCache, chartCacheMock)

	provider := NewChartProvider(helmChartCache, gitRepoCache)

	chart, err := provider.GetHelmChart(ctx, openapi.HelmChartReference{
		GitRepositoryPathReference: &openapi.GitRepositoryPathReference{
			RepositoryURL: "https://example.com/repo.git",
			Reference:     "refs/heads/main",
		},
	})
	require.NoError(t, err)
	require.NotNil(t, chart)

	metadata := chart.Metadata()
	assert.Equal(t, "testchart", metadata.Name)
	assert.Equal(t, "0.1.0", metadata.Version)
}

func TestChartProvider_GetHelmChart_HelmRepoReference_NoDependencies(t *testing.T) {
	ctx := context.Background()

	tarball := createChartTarball(t, "mychart", "1.0.0", nil)

	chartRemoteMock := helmremotemock.NewChartMock()
	chartRemoteMock.AddChart("oci://example.com/charts/mychart:1.0.0", tarball)

	indexRemoteMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()
	gitCacheMock := cachemock.New[[]byte]()
	gitMock := gitmock.NewMock()

	indexCache := cache.NewHelmIndexCache(indexRemoteMock, indexCacheMock)
	helmChartCache := cache.NewHelmChartCache(chartRemoteMock, indexCache, chartCacheMock)
	gitRepoCache := cache.NewGitRepositoryCache(gitMock, gitCacheMock)

	provider := NewChartProvider(helmChartCache, gitRepoCache)

	chart, err := provider.GetHelmChart(ctx, openapi.HelmChartReference{
		HelmChartRepositoryChartReference: &openapi.HelmChartRepositoryChartReference{
			RepositoryURL: "oci://example.com/charts",
			ChartName:     "mychart",
			ChartVersion:  utils.Ptr("1.0.0"),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, chart)

	metadata := chart.Metadata()
	assert.Equal(t, "mychart", metadata.Name)
	assert.Equal(t, "1.0.0", metadata.Version)
}

func TestChartProvider_GetHelmChart_WithDependencies(t *testing.T) {
	ctx := context.Background()

	depTarball := createChartTarball(t, "dep-chart", "2.0.0", nil)
	mainTarball := createChartTarball(t, "main-chart", "1.0.0", []chartDep{
		{Name: "dep-chart", Version: "2.0.0", Repository: "oci://example.com/deps"},
	})

	chartRemoteMock := helmremotemock.NewChartMock()
	chartRemoteMock.AddChart("oci://example.com/charts/main-chart:1.0.0", mainTarball)
	chartRemoteMock.AddChart("oci://example.com/deps/dep-chart:2.0.0", depTarball)

	indexRemoteMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()
	gitCacheMock := cachemock.New[[]byte]()
	gitMock := gitmock.NewMock()

	indexCache := cache.NewHelmIndexCache(indexRemoteMock, indexCacheMock)
	helmChartCache := cache.NewHelmChartCache(chartRemoteMock, indexCache, chartCacheMock)
	gitRepoCache := cache.NewGitRepositoryCache(gitMock, gitCacheMock)

	provider := NewChartProvider(helmChartCache, gitRepoCache)

	chart, err := provider.GetHelmChart(ctx, openapi.HelmChartReference{
		HelmChartRepositoryChartReference: &openapi.HelmChartRepositoryChartReference{
			RepositoryURL: "oci://example.com/charts",
			ChartName:     "main-chart",
			ChartVersion:  utils.Ptr("1.0.0"),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, chart)

	metadata := chart.Metadata()
	assert.Equal(t, "main-chart", metadata.Name)
	assert.Len(t, metadata.Dependencies, 1)
	assert.Equal(t, "dep-chart", metadata.Dependencies[0].Name)
	assert.Equal(t, "2.0.0", metadata.Dependencies[0].Version)
}

func TestChartProvider_GetHelmChart_MultipleDependencies(t *testing.T) {
	ctx := context.Background()

	dep1Tarball := createChartTarball(t, "dep1", "1.0.0", nil)
	dep2Tarball := createChartTarball(t, "dep2", "2.0.0", nil)
	dep3Tarball := createChartTarball(t, "dep3", "3.0.0", nil)

	mainTarball := createChartTarball(t, "main", "1.0.0", []chartDep{
		{Name: "dep1", Version: "1.0.0", Repository: "oci://example.com/deps"},
		{Name: "dep2", Version: "2.0.0", Repository: "oci://example.com/deps"},
		{Name: "dep3", Version: "3.0.0", Repository: "oci://example.com/deps"},
	})

	chartRemoteMock := helmremotemock.NewChartMock()
	chartRemoteMock.AddChart("oci://example.com/charts/main:1.0.0", mainTarball)
	chartRemoteMock.AddChart("oci://example.com/deps/dep1:1.0.0", dep1Tarball)
	chartRemoteMock.AddChart("oci://example.com/deps/dep2:2.0.0", dep2Tarball)
	chartRemoteMock.AddChart("oci://example.com/deps/dep3:3.0.0", dep3Tarball)

	indexRemoteMock := helmremotemock.NewIndexMock()
	indexCacheMock := cachemock.New[[]byte]()
	chartCacheMock := cachemock.New[[]byte]()
	gitCacheMock := cachemock.New[[]byte]()
	gitMock := gitmock.NewMock()

	indexCache := cache.NewHelmIndexCache(indexRemoteMock, indexCacheMock)
	helmChartCache := cache.NewHelmChartCache(chartRemoteMock, indexCache, chartCacheMock)
	gitRepoCache := cache.NewGitRepositoryCache(gitMock, gitCacheMock)

	provider := NewChartProvider(helmChartCache, gitRepoCache)

	chart, err := provider.GetHelmChart(ctx, openapi.HelmChartReference{
		HelmChartRepositoryChartReference: &openapi.HelmChartRepositoryChartReference{
			RepositoryURL: "oci://example.com/charts",
			ChartName:     "main",
			ChartVersion:  utils.Ptr("1.0.0"),
		},
	})
	require.NoError(t, err)
	require.NotNil(t, chart)

	metadata := chart.Metadata()
	assert.Equal(t, "main", metadata.Name)
	assert.Len(t, metadata.Dependencies, 3)

	depNames := make([]string, 0, 3)
	for _, d := range metadata.Dependencies {
		depNames = append(depNames, d.Name)
	}
	assert.Contains(t, depNames, "dep1")
	assert.Contains(t, depNames, "dep2")
	assert.Contains(t, depNames, "dep3")
}

// --- helpers ---

type chartDep struct {
	Name       string
	Version    string
	Repository string
}

func createChartTarball(t *testing.T, name, version string, deps []chartDep) []byte {
	t.Helper()

	memFS := filesystem.New()
	chartDir := memFS.Join(memFS.Root, name)
	require.NoError(t, memFS.MkdirAll(chartDir))

	chartYAML := "apiVersion: v2\nname: " + name + "\nversion: " + version + "\n"
	if len(deps) > 0 {
		chartYAML += "dependencies:\n"
		for _, d := range deps {
			chartYAML += "  - name: " + d.Name + "\n    version: " + d.Version + "\n    repository: " + d.Repository + "\n"
		}
	}

	writeMemFile(t, memFS, memFS.Join(chartDir, "Chart.yaml"), chartYAML)

	templatesDir := memFS.Join(chartDir, "templates")
	require.NoError(t, memFS.MkdirAll(templatesDir))
	writeMemFile(t, memFS, memFS.Join(templatesDir, "configmap.yaml"),
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: "+name+"\n")

	var buf bytes.Buffer
	require.NoError(t, targz.Compress(context.Background(), memFS, memFS.Root, "", &buf))
	return buf.Bytes()
}

func writeChartToDir(t *testing.T, dir, name, version string, deps []chartDep) {
	t.Helper()

	chartYAML := "apiVersion: v2\nname: " + name + "\nversion: " + version + "\n"
	if len(deps) > 0 {
		chartYAML += "dependencies:\n"
		for _, d := range deps {
			chartYAML += "  - name: " + d.Name + "\n    version: " + d.Version + "\n    repository: " + d.Repository + "\n"
		}
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "Chart.yaml"), []byte(chartYAML), 0644))

	templatesDir := filepath.Join(dir, "templates")
	require.NoError(t, os.MkdirAll(templatesDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(templatesDir, "configmap.yaml"),
		[]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: "+name+"\n"),
		0644,
	))
}

func writeMemFile(t *testing.T, memFS *filesystem.FileSystem, path, content string) {
	t.Helper()
	f, err := memFS.Create(path)
	require.NoError(t, err)
	_, err = f.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, f.Close())
}



