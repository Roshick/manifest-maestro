package kustomize

import (
	"context"
	"fmt"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/pkg/api"
	"github.com/Roshick/manifest-maestro/pkg/utils/commonutils"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
)

type KustomizationProvider struct {
	gitRepositoryCache *cache.GitRepositoryCache
}

func NewKustomizationProvider(gitRepositoryCache *cache.GitRepositoryCache) *KustomizationProvider {
	return &KustomizationProvider{
		gitRepositoryCache: gitRepositoryCache,
	}
}

func (p *KustomizationProvider) GetKustomization(ctx context.Context, abstractReference api.KustomizationReference) (*Kustomization, error) {
	if reference := abstractReference.GitRepositoryPathReference; reference != nil {
		return p.getKustomizationFromGitRepositoryPathReference(ctx, *reference)
	}
	return nil, fmt.Errorf("ToDo")
}

func (p *KustomizationProvider) getKustomizationFromGitRepositoryPathReference(ctx context.Context, reference api.GitRepositoryPathReference) (*Kustomization, error) {
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

	return p.buildKustomization(ctx, fileSystem, targetPath)
}

func (p *KustomizationProvider) buildKustomization(_ context.Context, fileSystem *filesystem.FileSystem, targetPath string) (*Kustomization, error) {
	return &Kustomization{
		fileSystem: fileSystem,
		targetPath: targetPath,
	}, nil
}
