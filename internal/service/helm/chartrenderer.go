package helm

import (
	"context"
	"fmt"
	"github.com/Roshick/manifest-maestro/pkg/api"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/mitchellh/copystructure"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/releaseutil"
	"sigs.k8s.io/yaml"
	"strings"
)

const (
	defaultReleaseName = "RELEASE-NAME"
	defaultNamespace   = "default"
)

type ChartRenderer struct {
	defaultKubernetesAPIVersions []string
}

func NewChartRenderer(defaultKubernetesAPIVersions []string) *ChartRenderer {
	return &ChartRenderer{
		defaultKubernetesAPIVersions: defaultKubernetesAPIVersions,
	}
}

func (r *ChartRenderer) Render(_ context.Context, helmChart *Chart, parameters *api.HelmRenderParameters) ([]api.Manifest, *api.HelmRenderMetadata, error) {
	actualParameters := api.HelmRenderParameters{}
	if parameters != nil {
		actualParameters = *parameters
	}

	allValues, err := helmChart.MergeValues(actualParameters)
	if err != nil {
		return nil, nil, err
	}

	if err = chartutil.ProcessDependencies(helmChart.chart, allValues); err != nil {
		return nil, nil, err
	}

	options := chartutil.ReleaseOptions{
		Name:      commonutils.DefaultIfEmpty(actualParameters.ReleaseName, defaultReleaseName),
		Namespace: commonutils.DefaultIfEmpty(actualParameters.Namespace, defaultNamespace),
	}

	capabilities := chartutil.DefaultCapabilities.Copy()
	capabilities.APIVersions = append(capabilities.APIVersions, r.defaultKubernetesAPIVersions...)
	capabilities.APIVersions = append(capabilities.APIVersions, actualParameters.ApiVersions...)
	renderValues, err := chartutil.ToRenderValues(helmChart.chart, allValues, options, capabilities)
	if err != nil {
		if strings.HasPrefix(err.Error(), "values don't meet the specifications of the schema(s)") {
			return nil, nil, fmt.Errorf("ToDo: VALIDATION ERROR: %w", err)
		}
		return nil, nil, fmt.Errorf("ToDo: customize errors: %w", err)
	}

	var mergedValues map[string]any
	if values, ok := renderValues.AsMap()["Values"]; ok {
		if typedValues, innerOk := values.(chartutil.Values); innerOk {
			valuesCopy, innerErr := copystructure.Copy(typedValues)
			if innerErr != nil {
				return nil, nil, fmt.Errorf("ToDo: customize errors: %w", err)
			}
			mergedValues = valuesCopy.(chartutil.Values).AsMap()
		}
	}
	metadata := &api.HelmRenderMetadata{
		ReleaseName:  options.Name,
		Namespace:    options.Namespace,
		ApiVersions:  capabilities.APIVersions,
		HelmVersion:  capabilities.HelmVersion.Version,
		MergedValues: mergedValues,
	}

	var renderEngine engine.Engine
	files, err := renderEngine.Render(helmChart.chart, renderValues)
	if err != nil {
		return nil, nil, fmt.Errorf("ToDo: customize errors")
	}
	if commonutils.DefaultIfNil(actualParameters.IncludeCRDs, true) {
		for _, crd := range helmChart.chart.CRDObjects() {
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
	parsedManifests := make([]api.Manifest, 0)
	for _, manifest := range manifests {
		parsedContent := make(map[string]any)
		if err = yaml.Unmarshal([]byte(manifest.Content), &parsedContent); err != nil {
			return nil, nil, err
		}
		parsedManifests = append(parsedManifests, api.Manifest{
			Source:  commonutils.Ptr(manifest.Name),
			Content: parsedContent,
		})
	}
	if commonutils.DefaultIfNil(actualParameters.IncludeHooks, true) {
		for _, hook := range hooks {
			parsedContent := make(map[string]any)
			if err = yaml.Unmarshal([]byte(hook.Manifest), &parsedContent); err != nil {
				return nil, nil, err
			}
			parsedManifests = append(parsedManifests, api.Manifest{
				Source:  commonutils.Ptr(hook.Name),
				Content: parsedContent,
			})
		}
	}

	return parsedManifests, metadata, nil
}
