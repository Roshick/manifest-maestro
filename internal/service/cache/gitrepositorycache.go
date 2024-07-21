package cache

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/targz"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Git interface {
	FetchReferences(context.Context, string) ([]*plumbing.Reference, error)

	CloneCommit(context.Context, string, string) (*git.Repository, error)
}

type GitRepositoryCache struct {
	git   Git
	cache cache.Cache[[]byte]

	commitHashRegex *regexp.Regexp
}

func NewGitRepositoryCache(git Git, cache cache.Cache[[]byte]) *GitRepositoryCache {
	return &GitRepositoryCache{
		git:             git,
		cache:           cache,
		commitHashRegex: regexp.MustCompile("[[:xdigit:]]{40}"),
	}
}

func (c *GitRepositoryCache) RetrieveRepository(ctx context.Context, url string, referenceOrHash string) ([]byte, error) {
	hash, err := c.toHash(ctx, url, referenceOrHash)
	if err != nil {
		return nil, err
	}

	key := c.cacheKey(url, hash)
	cached, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for git repository with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for git repository with key '%s', retrieving from remote", key)
	return c.RefreshRepository(ctx, url, hash)
}

func (c *GitRepositoryCache) RetrieveRepositoryToFileSystem(
	ctx context.Context, url string, referenceOrHash string, fileSystem *filesystem.FileSystem,
) error {
	tarball, err := c.RetrieveRepository(ctx, url, referenceOrHash)
	if err != nil {
		return err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(tarball), fileSystem.Root); err != nil {
		return err
	}
	return nil
}

func (c *GitRepositoryCache) RefreshRepository(ctx context.Context, url string, reference string) ([]byte, error) {
	hash, err := c.toHash(ctx, url, reference)
	if err != nil {
		return nil, err
	}

	tarball, err := c.fetchAsTarball(ctx, url, hash)
	key := c.cacheKey(url, hash)
	if err = c.cache.Set(ctx, key, tarball, time.Hour); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache git repository with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached git repository with key '%s'", key)
	}

	return tarball, nil
}

func (c *GitRepositoryCache) toHash(ctx context.Context, url string, referenceOrHash string) (string, error) {
	if !c.isFullyQualifiedReference(referenceOrHash) && !c.isCommitHash(referenceOrHash) {
		return "", fmt.Errorf("ToDo invalid reference %s", referenceOrHash)
	}

	if c.isCommitHash(referenceOrHash) {
		return referenceOrHash, nil
	}

	remoteReferences, err := c.git.FetchReferences(ctx, url)
	if err != nil {
		return "", err
	}
	for _, ref := range remoteReferences {
		if ref.Name().String() == referenceOrHash {
			return ref.Hash().String(), nil
		}
	}
	return "", fmt.Errorf("ToDo: reference %q not found", referenceOrHash)
}

func (c *GitRepositoryCache) fetchAsTarball(ctx context.Context, url string, hash string) ([]byte, error) {
	repo, err := c.git.CloneCommit(ctx, url, hash)
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

func (c *GitRepositoryCache) isFullyQualifiedReference(reference string) bool {
	return reference == "HEAD" ||
		strings.HasPrefix(reference, "refs/heads/") ||
		strings.HasPrefix(reference, "refs/tags/")
}

func (c *GitRepositoryCache) isCommitHash(reference string) bool {
	return c.commitHashRegex.MatchString(reference)
}

func (c *GitRepositoryCache) cacheKey(gitURL string, gitReference string) string {
	return fmt.Sprintf("%s|%s", gitURL, gitReference)
}
