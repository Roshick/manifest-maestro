package helm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/service/gitmanager"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/maputils"
	"github.com/Roshick/manifest-maestro/pkg/utils/targz"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/ignore"
	"helm.sh/helm/v3/pkg/releaseutil"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/strvals"
	"sigs.k8s.io/yaml"
)

type Remote interface {
	GetIndex(context.Context, string) ([]byte, error)

	GetChart(context.Context, string) ([]byte, error)
}

type Helm struct {
	appConfig *config.ApplicationConfig

	helmRemote Remote

	gitManager *gitmanager.GitManager

	indexCache cache.Cache[[]byte]
	chartCache cache.Cache[[]byte]
}

func New(
	configuration *config.ApplicationConfig,
	helmRemote Remote,
	gitManager *gitmanager.GitManager,
	indexCache cache.Cache[[]byte],
	chartCache cache.Cache[[]byte],
) *Helm {
	return &Helm{
		appConfig:  configuration,
		helmRemote: helmRemote,
		gitManager: gitManager,
		indexCache: indexCache,
		chartCache: chartCache,
	}
}

func (h *Helm) RetrieveIndex(ctx context.Context, repositoryURL string) (*repo.IndexFile, error) {
	cached, err := h.indexCache.Get(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm repository index with key '%s'", repositoryURL)
		return parseIndex(*cached)
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm repository index with key '%s', retrieving from remote", repositoryURL)
	return h.RefreshIndex(ctx, repositoryURL)
}

func (h *Helm) RefreshCachedIndexes(ctx context.Context) error {
	repositoryURLs, err := h.indexCache.Keys(ctx)
	if err != nil {
		return err
	}
	return h.RefreshIndexes(ctx, repositoryURLs)
}

func (h *Helm) RefreshIndexes(ctx context.Context, repositoryURLs []string) error {
	errs := make([]error, 0)
	for _, repositoryURL := range repositoryURLs {
		if _, innerErr := h.RefreshIndex(ctx, repositoryURL); innerErr != nil {
			errs = append(errs, innerErr)
		}
	}
	return errors.Join(errs...)
}

func (h *Helm) RefreshIndex(ctx context.Context, repositoryURL string) (*repo.IndexFile, error) {
	indexBytes, err := h.helmRemote.GetIndex(ctx, repositoryURL)
	if err != nil {
		return nil, err
	}

	if err = h.indexCache.Set(ctx, repositoryURL, indexBytes, 10*time.Minute); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm repository index with key '%s'", repositoryURL)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm repository index with key '%s'", repositoryURL)
	}

	return parseIndex(indexBytes)
}

func (h *Helm) RetrieveChart(ctx context.Context, chartReference apimodel.ChartReference) ([]byte, error) {
	index, err := h.RetrieveIndex(ctx, chartReference.RepositoryURL)
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
	cached, err := h.chartCache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for helm chart with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for helm chart with key '%s', retrieving from remote", key)
	return h.RefreshChart(ctx, chartReference.RepositoryURL, chartVersion)
}

func (h *Helm) RefreshChart(ctx context.Context, repositoryURL string, chartVersion *repo.ChartVersion) ([]byte, error) {
	chartURL := chartVersion.URLs[0]
	// no protocol => url is relative
	if !strings.Contains(chartURL, "://") {
		chartURL = fmt.Sprintf("%s/%s", repositoryURL, chartURL)
	}
	chartBytes, err := h.helmRemote.GetChart(ctx, chartURL)
	if err != nil {
		return nil, err
	}

	key := chartCacheKey(repositoryURL, chartVersion.Name, chartVersion.Version, chartVersion.Digest)
	if err = h.chartCache.Set(ctx, key, chartBytes, 24*time.Hour); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache helm chart with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached helm chart with key '%s'", key)
	}

	return chartBytes, nil
}

func (h *Helm) BuildChart(
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
		chartBytes, innerErr := h.RetrieveChart(ctx, apimodel.ChartReference{
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

func (h *Helm) RenderChart(
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
	capabilities.APIVersions = append(capabilities.APIVersions, h.appConfig.HelmDefaultKubernetesAPIVersions()...)
	capabilities.APIVersions = append(capabilities.APIVersions, parameters.ApiVersions...)
	valuesToRender, err := chartutil.ToRenderValues(helmChart, allValues, options, capabilities)
	if err != nil {
		if strings.HasPrefix(err.Error(), "values don't meet the specifications of the schema(h)") {
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

func (h *Helm) ChartMetadata(
	ctx context.Context,
	chartReference apimodel.ChartReference,
) (map[string]any, error) {
	chartBytes, err := h.RetrieveChart(ctx, chartReference)
	if err != nil {
		return nil, err
	}

	helmChart, err := loader.LoadArchive(bytes.NewBuffer(chartBytes))
	if err != nil {
		return nil, err
	}
	return helmChart.Values, nil
}

func (h *Helm) MergeValues(
	ctx context.Context, fileSystem *filesystem.FileSystem, targetPath string, parameters apimodel.HelmRenderParameters,
) (map[string]any, error) {
	values := maputils.DeepMerge(parameters.ComplexValues, make(map[string]any))

	for _, fileName := range parameters.ValueFiles {
		filePath := fileSystem.Join(targetPath, fileName)
		if fileSystem.Exists(filePath) {
			valueFile, err := fileSystem.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			tmpValues := make(map[string]any)
			if err = yaml.Unmarshal(valueFile, &tmpValues); err != nil {
				return nil, err
			}
			values = maputils.DeepMerge(tmpValues, values)
		} else if parameters.IgnoreMissingValueFiles == nil || !*parameters.IgnoreMissingValueFiles {
			return nil, fmt.Errorf(fmt.Sprintf("repository is missing value file at '%s'", filePath))
		}
	}

	for _, remoteValueFile := range parameters.RemoteGitValueFiles {
		repoFileSystem, err := h.gitManager.RetrieveRepositoryToFileSystem(ctx, remoteValueFile.Url, remoteValueFile.Reference)
		if err != nil {
			return nil, err
		}
		valueFile, err := repoFileSystem.ReadFile(remoteValueFile.Path)
		if err != nil {
			return nil, err
		}
		tmpValues := make(map[string]any)
		if err = yaml.Unmarshal(valueFile, &tmpValues); err != nil {
			return nil, err
		}
		values = maputils.DeepMerge(tmpValues, values)
	}

	for _, value := range append(flattenValues(parameters.Values), parameters.ValuesFlat...) {
		if err := strvals.ParseInto(value, values); err != nil {
			return nil, err
		}
	}

	for _, value := range append(flattenValues(parameters.StringValues), parameters.StringValuesFlat...) {
		if err := strvals.ParseIntoString(value, values); err != nil {
			return nil, err
		}
	}

	return values, nil
}

func (h *Helm) addRemoteValues(
	ctx context.Context, remoteValueFiles []apimodel.HelmChartRemoteGitValueFile, values map[string]any,
) (map[string]any, error) {
	newValues := maputils.DeepMerge(make(map[string]any), values)
	for _, remoteValueFile := range remoteValueFiles {
		tempFileSystem := filesystem.New()
		repoTarball, err := h.gitManager.RetrieveRepository(ctx, remoteValueFile.Url, remoteValueFile.Reference)
		if err != nil {
			return nil, err
		}
		if err = targz.Extract(ctx, tempFileSystem, bytes.NewBuffer(repoTarball), tempFileSystem.Root); err != nil {
			return nil, err
		}
		valueFile, err := tempFileSystem.ReadFile(remoteValueFile.Path)
		if err != nil {
			return nil, err
		}
		if err = yaml.Unmarshal(valueFile, &newValues); err != nil {
			return nil, err
		}
	}
	return newValues, nil
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
