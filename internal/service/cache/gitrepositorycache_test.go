package cache

import (
	"context"
	"testing"

	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/Roshick/manifest-maestro/test/mock/cachemock"
	"github.com/Roshick/manifest-maestro/test/mock/gitmock"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitRepositoryCache_RetrieveRepository_CacheMiss(t *testing.T) {
	ctx := context.Background()

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, ref string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		}).
		WithCloneCommit(func(_ context.Context, _ string, _ string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir("../../../test/resources/mocks/git-repositories/test")
		})
	cacheMock := cachemock.New[[]byte]()

	gitRepoCache := NewGitRepositoryCache(gitMock, cacheMock)

	tarball, err := gitRepoCache.RetrieveRepository(ctx, "https://example.com/repo.git", "refs/heads/main")
	require.NoError(t, err)
	assert.NotEmpty(t, tarball, "tarball should not be empty")
}

func TestGitRepositoryCache_RetrieveRepository_CacheHit(t *testing.T) {
	ctx := context.Background()

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, ref string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		})
	cacheMock := cachemock.New[[]byte]()

	// Pre-populate cache
	_ = cacheMock.Set(ctx, "https://example.com/repo.git|abc123commithashthatisfortycharactersss", []byte("cached-tarball"), 0)

	gitRepoCache := NewGitRepositoryCache(gitMock, cacheMock)

	tarball, err := gitRepoCache.RetrieveRepository(ctx, "https://example.com/repo.git", "refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, []byte("cached-tarball"), tarball)

	// CloneCommit should NOT have been called since we had a cache hit
	assert.Equal(t, int32(0), gitMock.CloneCommitCallCount.Load(), "CloneCommit should not be called on cache hit")
}

func TestGitRepositoryCache_RetrieveRepository_ToHashCalledOnce(t *testing.T) {
	ctx := context.Background()

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, ref string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		}).
		WithCloneCommit(func(_ context.Context, _ string, _ string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir("../../../test/resources/mocks/git-repositories/test")
		})
	cacheMock := cachemock.New[[]byte]()

	gitRepoCache := NewGitRepositoryCache(gitMock, cacheMock)

	_, err := gitRepoCache.RetrieveRepository(ctx, "https://example.com/repo.git", "refs/heads/main")
	require.NoError(t, err)

	// ToHash should be called exactly once per request.
	toHashCalls := gitMock.ToHashCallCount.Load()
	t.Logf("ToHash was called %d times", toHashCalls)
	assert.Equal(t, int32(1), toHashCalls, "ToHash should be called exactly once")
}

func TestGitRepositoryCache_RetrieveRepositoryToFileSystem_ToHashCallCount(t *testing.T) {
	ctx := context.Background()

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, ref string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		}).
		WithCloneCommit(func(_ context.Context, _ string, _ string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir("../../../test/resources/mocks/git-repositories/test")
		})
	cacheMock := cachemock.New[[]byte]()

	gitRepoCache := NewGitRepositoryCache(gitMock, cacheMock)

	fs := filesystem.New()
	err := gitRepoCache.RetrieveRepositoryToFileSystem(ctx, "https://example.com/repo.git", "refs/heads/main", fs)
	require.NoError(t, err)

	// ToHash should be called exactly once, even through RetrieveRepositoryToFileSystem.
	toHashCalls := gitMock.ToHashCallCount.Load()
	t.Logf("ToHash was called %d times for RetrieveRepositoryToFileSystem", toHashCalls)
	assert.Equal(t, int32(1), toHashCalls, "ToHash should be called exactly once")

	// Verify the filesystem has content
	assert.True(t, fs.Exists(fs.Join(fs.Root, "Chart.yaml")), "Chart.yaml should exist in filesystem")
}

func TestGitRepositoryCache_RetrieveRepositoryToFileSystem_CacheHit(t *testing.T) {
	ctx := context.Background()

	gitMock := gitmock.NewMock().
		WithToHash(func(_ context.Context, _ string, ref string) (string, error) {
			return "abc123commithashthatisfortycharactersss", nil
		}).
		WithCloneCommit(func(_ context.Context, _ string, _ string) (*git.Repository, error) {
			return gitmock.CreateRepoFromDir("../../../test/resources/mocks/git-repositories/test")
		})
	cacheMock := cachemock.New[[]byte]()

	gitRepoCache := NewGitRepositoryCache(gitMock, cacheMock)

	// First call: cache miss, should clone
	fs1 := filesystem.New()
	err := gitRepoCache.RetrieveRepositoryToFileSystem(ctx, "https://example.com/repo.git", "refs/heads/main", fs1)
	require.NoError(t, err)
	assert.Equal(t, int32(1), gitMock.CloneCommitCallCount.Load(), "CloneCommit should be called once on cache miss")

	// Second call: cache hit, should NOT clone
	fs2 := filesystem.New()
	err = gitRepoCache.RetrieveRepositoryToFileSystem(ctx, "https://example.com/repo.git", "refs/heads/main", fs2)
	require.NoError(t, err)
	assert.Equal(t, int32(1), gitMock.CloneCommitCallCount.Load(), "CloneCommit should still be 1 on cache hit")

	// Both filesystems should have the same content
	assert.True(t, fs2.Exists(fs2.Join(fs2.Root, "Chart.yaml")), "Chart.yaml should exist in second filesystem")
}



