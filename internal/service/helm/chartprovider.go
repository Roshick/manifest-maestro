package helm

import (
	"bytes"
	"context"
	"fmt"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/pkg/api"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/ignore"
	"io/fs"
	"path/filepath"
	"strings"
)

const (
	chartsDir = "charts"
)

type ChartProvider struct {
	helmChartCache     *cache.HelmChartCache
	gitRepositoryCache *cache.GitRepositoryCache
}

func NewChartProvider(helmChartCache *cache.HelmChartCache, gitRepositoryCache *cache.GitRepositoryCache) *ChartProvider {
	return &ChartProvider{
		helmChartCache:     helmChartCache,
		gitRepositoryCache: gitRepositoryCache,
	}
}

func (p *ChartProvider) ListHelmCharts(ctx context.Context, abstractReference api.HelmRepositoryReference) ([]map[string]any, error) {
	if reference := abstractReference.HelmChartRepositoryReference; reference != nil {
		// ToDo
		return nil, nil
	}
	return nil, fmt.Errorf("ToDo")
}

func (p *ChartProvider) GetHelmChart(ctx context.Context, abstractReference api.HelmChartReference) (*Chart, error) {
	if reference := abstractReference.HelmChartRepositoryChartReference; reference != nil {
		return p.getHelmChartFromHelmRepositoryChartReference(ctx, *reference)
	}
	if reference := abstractReference.GitRepositoryPathReference; reference != nil {
		return p.getHelmChartFromGitRepositoryPathReference(ctx, *reference)
	}
	return nil, fmt.Errorf("ToDo")
}

func (p *ChartProvider) getHelmChartFromGitRepositoryPathReference(ctx context.Context, reference api.GitRepositoryPathReference) (*Chart, error) {
	fileSystem := filesystem.New()

	targetPath := fileSystem.Root
	if !commonutils.IsEmpty(reference.Path) {
		if fileSystem.IsAbs(*reference.Path) {
			return nil, fmt.Errorf("git source path cannot be absolute")
		}
		targetPath = fileSystem.Join(targetPath, *reference.Path)
	}

	err := p.gitRepositoryCache.RetrieveRepositoryToFileSystem(ctx, reference.RepositoryURL, reference.Reference, fileSystem)
	if err != nil {
		return nil, err
	}

	return p.buildChart(ctx, fileSystem, targetPath)
}

func (p *ChartProvider) getHelmChartFromHelmRepositoryChartReference(ctx context.Context, reference api.HelmChartRepositoryChartReference) (*Chart, error) {
	fileSystem := filesystem.New()

	err := p.helmChartCache.RetrieveChartToFileSystem(ctx, reference, fileSystem)
	if err != nil {
		return nil, err
	}

	targetPath := fileSystem.Join(fileSystem.Root, reference.ChartName)
	return p.buildChart(ctx, fileSystem, targetPath)
}

func (p *ChartProvider) buildChart(ctx context.Context, fileSystem *filesystem.FileSystem, targetPath string) (*Chart, error) {
	aulogging.Logger.Ctx(ctx).Info().Printf("building chart at %s", targetPath)

	helmChart, err := p.loadChart(ctx, fileSystem, targetPath)
	if err != nil {
		return nil, err
	}

	chartsPath := fileSystem.Join(targetPath, chartsDir)
	if err = fileSystem.MkdirAll(chartsPath); err != nil {
		return nil, err
	}

	for _, dependency := range helmChart.Metadata.Dependencies {
		if innerErr := dependency.Validate(); innerErr != nil {
			return nil, innerErr
		}

		if path := fileSystem.Join(chartsPath, fmt.Sprintf("%s-%s.tgz", dependency.Name, dependency.Version)); fileSystem.Exists(path) {
			dependencyChart, innerErr := p.loadChart(ctx, fileSystem, path)
			if innerErr != nil {
				return nil, innerErr
			}
			if dependencyChart.Metadata.Version == dependency.Version {
				helmChart.AddDependency(dependencyChart)
				continue
			}
		} else if path = fileSystem.Join(chartsPath, fmt.Sprintf("%s", dependency.Name)); fileSystem.Exists(path) {
			dependencyChart, innerErr := p.loadChart(ctx, fileSystem, path)
			if innerErr != nil {
				return nil, innerErr
			}
			if dependencyChart.Metadata.Version == dependency.Version {
				helmChart.AddDependency(dependencyChart)
				continue
			}
		}
		chartBytes, innerErr := p.helmChartCache.RetrieveChart(ctx, api.HelmChartRepositoryChartReference{
			RepositoryURL: dependency.Repository,
			ChartName:     dependency.Name,
			ChartVersion:  commonutils.Ptr(dependency.Version),
		})
		if innerErr != nil {
			return nil, innerErr
		}
		dependencyChart, innerErr := loader.LoadArchive(bytes.NewReader(chartBytes))
		if innerErr != nil {
			return nil, innerErr
		}
		helmChart.AddDependency(dependencyChart)
	}

	return &Chart{
		chart:      helmChart,
		fileSystem: fileSystem,
		targetPath: targetPath,
	}, nil
}

func (p *ChartProvider) loadChart(ctx context.Context, fileSystem *filesystem.FileSystem, targetPath string) (*chart.Chart, error) {
	if !fileSystem.IsAbs(targetPath) {
		return nil, fmt.Errorf("ToDo: only absolute paths allowed")
	}

	rules := ignore.Empty()
	ignoreFilePath := fileSystem.Join(targetPath, ignore.HelmIgnore)
	if fileSystem.Exists(ignoreFilePath) {
		ignoreFile, err := fileSystem.Open(ignoreFilePath)
		if err != nil {
			return nil, err
		}
		defer func() {
			if innerErr := ignoreFile.Close(); err != nil {
				aulogging.Logger.Ctx(ctx).Warn().WithErr(innerErr).Printf("failed to close '%s'", ignoreFilePath)
			}
		}()
		rules, err = ignore.Parse(ignoreFile)
		if err != nil {
			return nil, err
		}
	}
	rules.AddDefaults()

	files := make([]*loader.BufferedFile, 0)
	walk := func(filePath string, fileInfo fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !fileInfo.Mode().IsRegular() {
			return fmt.Errorf("cannot load irregular file '%s'", filePath)
		}

		newFilePath := strings.TrimPrefix(filePath, targetPath)
		newFilePath = strings.TrimPrefix(newFilePath, fileSystem.Separator)
		if newFilePath == "" {
			return nil
		}

		if fileInfo.IsDir() {
			if rules.Ignore(newFilePath, fileInfo) {
				return fileSystem.SkipDir()
			}
			return nil
		}
		if rules.Ignore(newFilePath, fileInfo) {
			return nil
		}

		data, err := fileSystem.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", newFilePath, err)
		}
		files = append(files, &loader.BufferedFile{Name: filepath.ToSlash(newFilePath), Data: data})
		return nil
	}

	if err := fileSystem.Walk(targetPath, walk); err != nil {
		return nil, err
	}

	return loader.LoadFiles(files)
}
