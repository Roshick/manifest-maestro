package cache

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/targz"
	"github.com/go-git/go-git/v5"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	aulogging "github.com/StephanHCB/go-autumn-logging"
)

type Git interface {
	CloneCommit(context.Context, string, string) (*git.Repository, error)

	ToHash(ctx context.Context, repositoryURL string, gitReference string) (string, error)
}

type GitRepositoryCache struct {
	git   Git
	cache cache.Cache[[]byte]
}

func NewGitRepositoryCache(git Git, cache cache.Cache[[]byte]) *GitRepositoryCache {
	return &GitRepositoryCache{
		git:   git,
		cache: cache,
	}
}

func (c *GitRepositoryCache) RetrieveRepository(
	ctx context.Context, repositoryURL string, gitReference string,
) ([]byte, error) {
	commitHash, err := c.git.ToHash(ctx, repositoryURL, gitReference)
	if err != nil {
		return nil, err
	}

	key := c.cacheKey(repositoryURL, commitHash)
	cached, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for git repository with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for git repository with key '%s', retrieving from remote", key)
	return c.refreshRepository(ctx, repositoryURL, commitHash)
}

func (c *GitRepositoryCache) RetrieveRepositoryToFileSystem(
	ctx context.Context, repositoryURL string, gitReference string, fileSystem *filesystem.FileSystem,
) error {
	commitHash, err := c.git.ToHash(ctx, repositoryURL, gitReference)
	if err != nil {
		return err
	}

	tarball, err := c.RetrieveRepository(ctx, repositoryURL, commitHash)
	if err != nil {
		return err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(tarball), fileSystem.Root); err != nil {
		return err
	}
	return nil
}

func (c *GitRepositoryCache) refreshRepository(ctx context.Context, repositoryURL string, gitReference string) ([]byte, error) {
	commitHash, err := c.git.ToHash(ctx, repositoryURL, gitReference)
	if err != nil {
		return nil, err
	}

	key := c.cacheKey(repositoryURL, commitHash)
	tarball, err := c.fetchAsTarball(ctx, repositoryURL, commitHash)
	if err != nil {
		return nil, err
	}
	if err = c.cache.Set(ctx, key, tarball, 5*time.Minute); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache git repository with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached git repository with key '%s'", key)
	}

	return tarball, nil
}

func (c *GitRepositoryCache) fetchAsTarball(ctx context.Context, repositoryURL string, commitHash string) ([]byte, error) {
	repo, err := c.git.CloneCommit(ctx, repositoryURL, commitHash)
	if err != nil {
		return nil, err
	}

	tree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	fileSystem := filesystem.New()
	if err = filesystem.CopyFromBillyToFileSystem(tree.Filesystem, fileSystem); err != nil {
		return nil, err
	}

	repoBuffer := new(bytes.Buffer)
	if err = targz.Compress(ctx, fileSystem, fileSystem.Root, "", repoBuffer); err != nil {
		return nil, err
	}
	return repoBuffer.Bytes(), nil
}

func (c *GitRepositoryCache) cacheKey(repositoryURL string, gitReference string) string {
	return fmt.Sprintf("%s|%s", repositoryURL, gitReference)
}
