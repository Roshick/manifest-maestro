package manifestrenderer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/targz"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
)

type Git interface {
	CloneCommit(context.Context, string, string) (*git.Repository, error)
}

type ManifestRenderer struct {
	configuration *config.ApplicationConfig

	git Git

	helm      *helm.Helm
	kustomize *kustomize.Kustomize

	cache cache.Cache[[]byte]
}

func New(
	configuration *config.ApplicationConfig,
	git Git,
	helm *helm.Helm,
	kustomize *kustomize.Kustomize,
	cache cache.Cache[[]byte],
) *ManifestRenderer {
	return &ManifestRenderer{
		configuration: configuration,
		git:           git,
		helm:          helm,
		kustomize:     kustomize,
		cache:         cache,
	}
}

func (r *ManifestRenderer) RenderHelmFromGitRepository(
	ctx context.Context,
	gitSource apimodel.GitSource,
	parameters *apimodel.HelmRenderParameters,
) ([]apimodel.HelmManifest, *apimodel.HelmRenderMetadata, error) {
	fileSystem := filesystem.New()

	targetPath := fileSystem.Root
	if gitSource.SubPath != nil {
		if fileSystem.IsAbs(*gitSource.SubPath) {
			return nil, nil, fmt.Errorf("git repository sub-path cannot be absolute")
		}
		targetPath = fileSystem.Join(targetPath, *gitSource.SubPath)
	}

	repoTarball, err := r.RetrieveGitRepository(ctx, gitSource)
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
	if gitSource.SubPath != nil {
		if fileSystem.IsAbs(*gitSource.SubPath) {
			return nil, fmt.Errorf("git repository sub-path cannot be absolute")
		}
		targetPath = fileSystem.Join(targetPath, *gitSource.SubPath)
	}

	repoTarball, err := r.RetrieveGitRepository(ctx, gitSource)
	if err != nil {
		return nil, err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(repoTarball), fileSystem.Root); err != nil {
		return nil, err
	}

	return r.kustomize.Render(ctx, fileSystem, targetPath, parameters)
}

func (r *ManifestRenderer) RetrieveGitRepository(ctx context.Context, gitSource apimodel.GitSource) ([]byte, error) {
	key := cacheKey(gitSource.Url, gitSource.Reference)
	cached, err := r.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for git repository with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for git repository with key '%s', retrieving from remote", key)
	return r.RefreshGitRepository(ctx, gitSource)
}

func (r *ManifestRenderer) RefreshGitRepository(ctx context.Context, gitSource apimodel.GitSource) ([]byte, error) {
	gitRepo, err := r.git.CloneCommit(ctx, gitSource.Url, gitSource.Reference)
	if err != nil {
		return nil, err
	}
	tree, err := gitRepo.Worktree()
	if err != nil {
		return nil, err
	}
	fileSystem := filesystem.New()
	if err = copyToFileSystem(tree.Filesystem, fileSystem); err != nil {
		return nil, err
	}
	repoBuffer := new(bytes.Buffer)
	if err = targz.Compress(ctx, fileSystem, fileSystem.Root, "", repoBuffer); err != nil {
		return nil, err
	}
	repoBytes := repoBuffer.Bytes()

	key := cacheKey(gitSource.Url, gitSource.Reference)
	if err = r.cache.Set(ctx, key, repoBytes, time.Hour); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache git repository with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached git repository with key '%s'", key)
	}

	return repoBytes, nil
}

func copyToFileSystem(origin billy.Filesystem, fileSystem *filesystem.FileSystem) error {
	var copyRecursively func(currentPath string) error
	copyRecursively = func(currentPath string) error {
		files, err := origin.ReadDir(currentPath)
		if err != nil {
			return err
		}

		for _, file := range files {
			fileName := fileSystem.Join(currentPath, file.Name())
			if file.IsDir() {
				if innerErr := fileSystem.MkdirAll(fileName); innerErr != nil {
					return innerErr
				}
				if innerErr := copyRecursively(fileName); innerErr != nil {
					return innerErr
				}
			} else {
				src, innerErr := origin.Open(fileName)
				if innerErr != nil {
					return innerErr
				}

				dst, innerErr := fileSystem.Create(fileName)
				if innerErr != nil {
					return innerErr
				}

				if _, innerErr = io.Copy(dst, src); innerErr != nil {
					return innerErr
				}
				if innerErr = dst.Close(); innerErr != nil {
					return innerErr
				}
				if innerErr = src.Close(); innerErr != nil {
					return innerErr
				}
			}
		}
		return nil
	}
	return copyRecursively("")
}

func cacheKey(gitURL string, gitReference string) string {
	return fmt.Sprintf("%s|%s", gitURL, gitReference)
}
