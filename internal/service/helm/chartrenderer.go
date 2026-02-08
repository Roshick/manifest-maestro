package helm

import (
	"context"
	"fmt"
	"strings"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/internal/utils"
	"github.com/mitchellh/copystructure"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	v2cutil "helm.sh/helm/v4/pkg/chart/v2/util"
	"helm.sh/helm/v4/pkg/engine"
	v1rutil "helm.sh/helm/v4/pkg/release/v1/util"
	"sigs.k8s.io/yaml"
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

func (r *ChartRenderer) Render(
	ctx context.Context,
	helmChart *Chart,
	parameters *openapi.HelmRenderParameters,
) ([]openapi.Manifest, *openapi.HelmRenderMetadata, error) {
	manifests, metadata, err := r.render(ctx, helmChart, parameters)
	if err != nil {
		return nil, nil, NewChartRenderError(err)
	}
	return manifests, metadata, nil
}

func (r *ChartRenderer) render(
	ctx context.Context,
	helmChart *Chart,
	parameters *openapi.HelmRenderParameters,
) ([]openapi.Manifest, *openapi.HelmRenderMetadata, error) {
	actualParameters := openapi.HelmRenderParameters{}
	if parameters != nil {
		actualParameters = *parameters
	}

	allValues, err := helmChart.MergeValues(actualParameters)
	if err != nil {
		return nil, nil, err
	}

	if err = v2cutil.ProcessDependencies(helmChart.chart, allValues); err != nil {
		return nil, nil, err
	}

	options := common.ReleaseOptions{
		Name:      utils.DefaultIfEmpty(actualParameters.ReleaseName, defaultReleaseName),
		Namespace: utils.DefaultIfEmpty(actualParameters.Namespace, defaultNamespace),
	}

	capabilities := common.DefaultCapabilities.Copy()
	capabilities.APIVersions = append(capabilities.APIVersions, r.defaultKubernetesAPIVersions...)
	capabilities.APIVersions = append(capabilities.APIVersions, actualParameters.ApiVersions...)
	renderValues, err := util.ToRenderValues(helmChart.chart, allValues, options, capabilities)
	if err != nil {
		return nil, nil, err
	}

	var mergedValues map[string]any
	if values, ok := renderValues.AsMap()["Values"]; ok {
		if typedValues, innerOk := values.(common.Values); innerOk {
			valuesCopy, innerErr := copystructure.Copy(typedValues)
			if innerErr != nil {
				return nil, nil, fmt.Errorf("failed to copy values: %w", innerErr)
			}
			mergedValues = valuesCopy.(common.Values).AsMap()
		}
	}
	metadata := &openapi.HelmRenderMetadata{
		ReleaseName:   options.Name,
		Namespace:     options.Namespace,
		ApiVersions:   capabilities.APIVersions,
		KubeVersion:   capabilities.KubeVersion.String(),
		HelmVersion:   capabilities.HelmVersion.Version,
		MergedValues:  mergedValues,
		ChartMetadata: helmChart.Metadata(),
	}

	var renderEngine engine.Engine
	files, err := renderEngine.Render(helmChart.chart, renderValues)
	if err != nil {
		return nil, nil, err
	}
	if utils.DefaultIfNil(actualParameters.IncludeCRDs, true) {
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

	hooks, manifests, err := v1rutil.SortManifests(
		templateFiles,
		capabilities.APIVersions,
		v1rutil.InstallOrder,
	)
	if err != nil {
		return nil, nil, err
	}
	parsedManifests := make([]openapi.Manifest, 0)
	for _, manifest := range manifests {
		parsedContent := make(map[string]any)
		if err = yaml.Unmarshal([]byte(manifest.Content), &parsedContent); err != nil {
			return nil, nil, err
		}
		if parsedContent == nil || len(parsedContent) == 0 {
			continue
		}
		parsedManifests = append(parsedManifests, openapi.Manifest{
			Source:  utils.Ptr(manifest.Name),
			Content: parsedContent,
		})
	}
	if utils.DefaultIfNil(actualParameters.IncludeHooks, true) {
		for _, hook := range hooks {
			parsedContent := make(map[string]any)
			if err = yaml.Unmarshal([]byte(hook.Manifest), &parsedContent); err != nil {
				return nil, nil, err
			}
			if parsedContent == nil || len(parsedContent) == 0 {
				continue
			}
			parsedManifests = append(parsedManifests, openapi.Manifest{
				Source:  utils.Ptr(hook.Name),
				Content: parsedContent,
			})
		}
	}

	return parsedManifests, metadata, nil
}
