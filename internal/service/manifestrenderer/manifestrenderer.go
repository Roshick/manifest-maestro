package manifestrenderer

import (
	"bytes"
	"context"
	"fmt"

	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/service/gitmanager"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/targz"
)

type ManifestRenderer struct {
	configuration *config.ApplicationConfig

	gitManager *gitmanager.GitManager
	helm       *helm.Helm
	kustomize  *kustomize.Kustomize
}

func New(
	configuration *config.ApplicationConfig,
	gitManager *gitmanager.GitManager,
	helm *helm.Helm,
	kustomize *kustomize.Kustomize,
) *ManifestRenderer {
	return &ManifestRenderer{
		configuration: configuration,
		gitManager:    gitManager,
		helm:          helm,
		kustomize:     kustomize,
	}
}

func (r *ManifestRenderer) RenderHelmFromGitRepository(
	ctx context.Context,
	gitSource apimodel.GitSource,
	parameters *apimodel.HelmRenderParameters,
) ([]apimodel.HelmManifest, *apimodel.HelmRenderMetadata, error) {
	fileSystem := filesystem.New()

	targetPath := fileSystem.Root
	if gitSource.Path != "" {
		if fileSystem.IsAbs(gitSource.Path) {
			return nil, nil, fmt.Errorf("git source path cannot be absolute")
		}
		targetPath = fileSystem.Join(targetPath, gitSource.Path)
	}

	repoTarball, err := r.gitManager.RetrieveRepository(ctx, gitSource.Url, gitSource.Reference)
	if err != nil {
		return nil, nil, err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(repoTarball), fileSystem.Root); err != nil {
		return nil, nil, err
	}

	actualParameters := apimodel.HelmRenderParameters{}
	if parameters != nil {
		actualParameters = *parameters
	}

	helmChart, err := r.helm.BuildChart(ctx, fileSystem, targetPath, actualParameters)
	if err != nil {
		return nil, nil, err
	}

	allValues, err := r.helm.MergeValues(ctx, fileSystem, targetPath, actualParameters)
	if err != nil {
		return nil, nil, err
	}

	return r.helm.RenderChart(ctx, helmChart, allValues, actualParameters)
}

func (r *ManifestRenderer) RenderHelmFromChartRepository(
	ctx context.Context,
	chartReference apimodel.ChartReference,
	parameters *apimodel.HelmRenderParameters,
) ([]apimodel.HelmManifest, *apimodel.HelmRenderMetadata, error) {
	fileSystem := filesystem.New()

	chartTarball, err := r.helm.RetrieveChart(ctx, chartReference)
	if err != nil {
		return nil, nil, err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(chartTarball), fileSystem.Root); err != nil {
		return nil, nil, err
	}
	targetPath := fileSystem.Join(fileSystem.Root, chartReference.Name)

	actualParameters := apimodel.HelmRenderParameters{}
	if parameters != nil {
		actualParameters = *parameters
	}

	helmChart, err := r.helm.BuildChart(ctx, fileSystem, targetPath, actualParameters)
	if err != nil {
		return nil, nil, err
	}

	allValues, err := r.helm.MergeValues(ctx, fileSystem, targetPath, actualParameters)
	if err != nil {
		return nil, nil, err
	}

	return r.helm.RenderChart(ctx, helmChart, allValues, actualParameters)
}

func (r *ManifestRenderer) RenderKustomizeFromGitRepository(
	ctx context.Context,
	gitSource apimodel.GitSource,
	parameters *apimodel.KustomizeRenderParameters,
) ([]apimodel.Manifest, error) {
	fileSystem := filesystem.New()

	targetPath := fileSystem.Root
	if gitSource.Path != "" {
		if fileSystem.IsAbs(gitSource.Path) {
			return nil, fmt.Errorf("git source path cannot be absolute")
		}
		targetPath = fileSystem.Join(targetPath, gitSource.Path)
	}

	repoTarball, err := r.gitManager.RetrieveRepository(ctx, gitSource.Url, gitSource.Reference)
	if err != nil {
		return nil, err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(repoTarball), fileSystem.Root); err != nil {
		return nil, err
	}

	return r.kustomize.Render(ctx, fileSystem, targetPath, parameters)
}
