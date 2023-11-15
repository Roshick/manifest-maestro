package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/maputils"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/ignore"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"sigs.k8s.io/yaml"
)

type Remote interface {
	GetIndex(context.Context, string) ([]byte, error)

	GetChart(context.Context, string) ([]byte, error)
}

type Helm struct {
	appConfig *config.ApplicationConfig

	helmRemote Remote

	indexCache cache.Cache[[]byte]
	chartCache cache.Cache[[]byte]
}

func New(
	configuration *config.ApplicationConfig,
	helmRemote Remote,
	indexCache cache.Cache[[]byte],
	chartCache cache.Cache[[]byte],
) *Helm {
	return &Helm{
		appConfig:  configuration,
		helmRemote: helmRemote,
		indexCache: indexCache,
		chartCache: chartCache,
	}
}

func (s *Helm) RetrieveIndex(ctx context.Context, repositoryURL string) (*repo.IndexFile, error) {
	cached, err := s.indexCache.Get(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm repository index with key '%s'", repositoryURL)
		return parseIndex(*cached)
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm repository index with key '%s', retrieving from remote", repositoryURL)
	return s.RefreshIndex(ctx, repositoryURL)
}

func (s *Helm) RefreshCachedIndexes(ctx context.Context) error {
	repositoryURLs, err := s.indexCache.Keys(ctx)
	if err != nil {
		return err
	}
	return s.RefreshIndexes(ctx, repositoryURLs)
}

func (s *Helm) RefreshIndexes(ctx context.Context, repositoryURLs []string) error {
	errs := make([]error, 0)
	for _, repositoryURL := range repositoryURLs {
		if _, innerErr := s.RefreshIndex(ctx, repositoryURL); innerErr != nil {
			errs = append(errs, innerErr)
		}
	}
	return errors.Join(errs...)
}

func (s *Helm) RefreshIndex(ctx context.Context, repositoryURL string) (*repo.IndexFile, error) {
	indexBytes, err := s.helmRemote.GetIndex(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}

	if err = s.indexCache.Set(ctx, repositoryURL, indexBytes, 10*time.Minute); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm repository index with key '%s'", repositoryURL)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm repository index with key '%s'", repositoryURL)
	}

	return parseIndex(indexBytes)
}

func (s *Helm) RetrieveChart(ctx context.Context, chartReference apimodel.ChartReference) ([]byte, error) {
	index, err := s.RetrieveIndex(ctx, chartReference.RepositoryURL)
	if err != nil {
		return nil, err
	}

	chartVersion, err := index.Get(chartReference.Name, commonutils.DefaultIfNil(chartReference.Version, ""))
	if err != nil {
		return nil, err
	}
	if len(chartVersion.URLs) == 0 {
		return nil, fmt.Errorf("ToDo: unprocessable")
	}

	key := chartCacheKey(chartReference.RepositoryURL, chartVersion.Name, chartVersion.Version, chartVersion.Digest)
	cached, err := s.chartCache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm chart with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm chart with key '%s', retrieving from remote", key)
	return s.RefreshChart(ctx, chartReference.RepositoryURL, chartVersion)
}

func (s *Helm) RefreshChart(ctx context.Context, repositoryURL string, chartVersion *repo.ChartVersion) ([]byte, error) {
	chartURL := chartVersion.URLs[0]
	// no protocol => url is relative
	if !strings.Contains(chartURL, "://") {
		chartURL = fmt.Sprintf("%s/%s", repositoryURL, chartURL)
	}
	chartBytes, err := s.helmRemote.GetChart(ctx, chartURL)
	if err != nil {
		return nil, err
	}

	key := chartCacheKey(repositoryURL, chartVersion.Name, chartVersion.Version, chartVersion.Digest)
	if err = s.chartCache.Set(ctx, key, chartBytes, 24*time.Hour); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm chart with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm chart with key '%s'", key)
	}

	return chartBytes, nil
}

func (s *Helm) BuildChart(
	ctx context.Context, fileSystem *filesystem.FileSystem, targetPath string, parameters apimodel.HelmRenderParameters,
) (*chart.Chart, error) {
	aulogging.Logger.Ctx(ctx).Info().Printf("building chart at %s", targetPath)

	helmChart, err := loadChart(ctx, fileSystem, targetPath)
	if err != nil {
		return nil, err
	}

	chartsPath := fileSystem.Join(targetPath, "charts")
	if err = fileSystem.MkdirAll(chartsPath); err != nil {
		return nil, err
	}

	isPatchTarget := func(dependency *chart.Dependency, patch apimodel.HelmChartDependencyPatch) bool {
		return patch.Target == nil ||
			((patch.Target.RepositoryURL == nil || *patch.Target.RepositoryURL == dependency.Repository) &&
				(patch.Target.Name == nil || *patch.Target.Name == dependency.Name) &&
				(patch.Target.Version == nil || *patch.Target.Version == dependency.Version) &&
				(patch.Target.Alias == nil || *patch.Target.Alias == dependency.Alias))
	}

	for _, dependency := range helmChart.Metadata.Dependencies {
		for _, patch := range parameters.DependencyPatches {
			if isPatchTarget(dependency, patch) {
				if commonutils.DefaultIfEmpty(patch.Values.Version, "") != "" {
					dependency.Version = *patch.Values.Version
				}
				if commonutils.DefaultIfEmpty(patch.Values.RepositoryURL, "") != "" {
					dependency.Repository = *patch.Values.RepositoryURL
				}
			}
		}

		if innerErr := dependency.Validate(); innerErr != nil {
			return nil, innerErr
		}

		if path := fileSystem.Join(chartsPath, fmt.Sprintf("%s-%s.tgz", dependency.Name, dependency.Version)); fileSystem.Exists(path) {
			dependencyChart, innerErr := loadChart(ctx, fileSystem, path)
			if innerErr != nil {
				return nil, innerErr
			}
			if dependencyChart.Metadata.Version == dependency.Version {
				helmChart.AddDependency(dependencyChart)
				continue
			}
		} else if path = fileSystem.Join(chartsPath, fmt.Sprintf("%s", dependency.Name)); fileSystem.Exists(path) {
			dependencyChart, innerErr := loadChart(ctx, fileSystem, path)
			if innerErr != nil {
				return nil, innerErr
			}
			if dependencyChart.Metadata.Version == dependency.Version {
				helmChart.AddDependency(dependencyChart)
				continue
			}
		}
		chartBytes, innerErr := s.RetrieveChart(ctx, apimodel.ChartReference{
			Name:          dependency.Name,
			RepositoryURL: dependency.Repository,
			Version:       commonutils.Ptr(dependency.Version),
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

	return helmChart, nil
}

func (s *Helm) RenderChart(
	_ context.Context, helmChart *chart.Chart, allValues map[string]any, parameters apimodel.HelmRenderParameters,
) ([]apimodel.HelmManifest, *apimodel.HelmRenderMetadata, error) {
	if err := chartutil.ProcessDependencies(helmChart, allValues); err != nil {
		return nil, nil, err
	}

	options := chartutil.ReleaseOptions{
		Name:      commonutils.DefaultIfEmpty(parameters.ReleaseName, "RELEASE-NAME"),
		Namespace: commonutils.DefaultIfEmpty(parameters.Namespace, "default"),
	}

	capabilities := chartutil.DefaultCapabilities.Copy()
	capabilities.APIVersions = append(capabilities.APIVersions, s.appConfig.HelmDefaultKubernetesAPIVersions()...)
	capabilities.APIVersions = append(capabilities.APIVersions, parameters.ApiVersions...)
	valuesToRender, err := chartutil.ToRenderValues(helmChart, allValues, options, capabilities)
	if err != nil {
		if strings.HasPrefix(err.Error(), "values don't meet the specifications of the schema(s)") {
			return nil, nil, fmt.Errorf("ToDo: VALIDATION ERROR: %w", err)
		}
		return nil, nil, fmt.Errorf("ToDo: customize errors: %w", err)
	}

	mergedValues := make(map[string]any)
	if _, ok := valuesToRender["Values"]; ok {
		if cMergedValues, ok2 := valuesToRender["Values"].(chartutil.Values); ok2 {
			mergedValues = cMergedValues
		}
	}

	metadata := apimodel.HelmRenderMetadata{
		ReleaseName:  options.Name,
		Namespace:    options.Namespace,
		ApiVersions:  capabilities.APIVersions,
		HelmVersion:  capabilities.HelmVersion.Version,
		MergedValues: mergedValues,
	}

	var renderEngine engine.Engine
	files, err := renderEngine.Render(helmChart, valuesToRender)
	if err != nil {
		return nil, nil, fmt.Errorf("ToDo: customize errors")
	}
	if commonutils.DefaultIfNil(parameters.IncludeCRDs, true) {
		for _, crd := range helmChart.CRDObjects() {
			files[crd.Filename] = string(crd.File.Data)
		}
	}
	templateFiles := make(map[string]string)
	for key, value := range files {
		if (strings.HasSuffix(key, ".yaml") || strings.HasSuffix(key, ".yml")) && value != "" {
			templateFiles[key] = value
		}
	}

	hooks, manifests, err := releaseutil.SortManifests(templateFiles, capabilities.APIVersions, releaseutil.InstallOrder)
	if err != nil {
		return nil, nil, err
	}

	parsedManifests := make([]apimodel.HelmManifest, 0)
	for _, manifest := range manifests {
		parsedContent := make(map[string]any)
		if err = yaml.Unmarshal([]byte(manifest.Content), &parsedContent); err != nil {
			return nil, nil, err
		}
		parsedManifests = append(parsedManifests, apimodel.HelmManifest{
			Source:  manifest.Name,
			Content: parsedContent,
		})
	}
	for _, hook := range hooks {
		parsedContent := make(map[string]any)
		if err = yaml.Unmarshal([]byte(hook.Manifest), &parsedContent); err != nil {
			return nil, nil, err
		}
		parsedManifests = append(parsedManifests, apimodel.HelmManifest{
			Source:  hook.Name,
			Content: parsedContent,
		})
	}

	return parsedManifests, &metadata, nil
}

func (s *Helm) ChartMetadata(
	ctx context.Context,
	chartReference apimodel.ChartReference,
) (map[string]any, error) {
	chartBytes, err := s.RetrieveChart(ctx, chartReference)
	if err != nil {
		return nil, err
	}

	helmChart, err := loader.LoadArchive(bytes.NewBuffer(chartBytes))
	if err != nil {
		return nil, err
	}
	return helmChart.Values, nil
}

func (s *Helm) MergeValues(
	_ context.Context, fileSystem *filesystem.FileSystem, targetPath string, parameters apimodel.HelmRenderParameters,
) (map[string]any, error) {
	valueFiles := make([]string, 0)
	for _, fileName := range parameters.ValueFiles {
		filePath := fileSystem.Join(targetPath, fileName)
		if fileSystem.Exists(filePath) {
			valueFiles = append(valueFiles, filePath)
		} else if parameters.IgnoreMissingValueFiles == nil || !*parameters.IgnoreMissingValueFiles {
			return nil, fmt.Errorf(fmt.Sprintf("repository is missing value file at '%s'", filePath))
		}
	}

	valueOpts := values.Options{
		ValueFiles:   valueFiles,
		Values:       append(flattenValues(parameters.Values), parameters.ValuesFlat...),
		StringValues: append(flattenValues(parameters.StringValues), parameters.StringValuesFlat...),
	}

	allValues, err := valueOpts.MergeValues(s.appConfig.HelmProviders())
	if err != nil {
		return nil, err
	}
	return maputils.DeepMerge(parameters.ComplexValues, allValues), nil
}

func loadChart(ctx context.Context, fileSystem *filesystem.FileSystem, targetPath string) (*chart.Chart, error) {
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
		iRules, err := ignore.Parse(ignoreFile)
		if err != nil {
			return nil, err
		}
		rules = iRules
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

		fileName := strings.TrimPrefix(filePath, targetPath)
		fileName = strings.TrimPrefix(fileName, fileSystem.Separator)
		if fileName == "" {
			return nil
		}

		if fileInfo.IsDir() {
			if rules.Ignore(fileName, fileInfo) {
				return fileSystem.SkipDir()
			}
			return nil
		}
		if rules.Ignore(fileName, fileInfo) {
			return nil
		}

		data, err := fileSystem.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("error reading %s: %w", fileName, err)
		}
		files = append(files, &loader.BufferedFile{Name: fileName, Data: data})
		return nil
	}

	if err := fileSystem.Walk(targetPath, walk); err != nil {
		return nil, err
	}

	return loader.LoadFiles(files)
}

func parseIndex(data []byte) (*repo.IndexFile, error) {
	index := &repo.IndexFile{}

	if len(data) == 0 {
		return nil, repo.ErrEmptyIndexYaml
	}
	if err := jsonOrYamlUnmarshal(data, index); err != nil {
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

func flattenValues(values *map[string]string) []string {
	if values == nil {
		return nil
	}
	flattenedValues := make([]string, 0)
	for key, value := range *values {
		flattenedValues = append(flattenedValues, fmt.Sprintf("%s=%s", key, value))
	}
	return flattenedValues
}

func jsonOrYamlUnmarshal(unknownBytes []byte, obj any) error {
	if json.Valid(unknownBytes) {
		return json.Unmarshal(unknownBytes, obj)
	}
	return yaml.UnmarshalStrict(unknownBytes, obj)
}

func chartCacheKey(repositoryURL string, chartName string, version string, chartDigest string) string {
	return fmt.Sprintf("%s|%s|%s|%s", repositoryURL, chartName, version, chartDigest)
}
