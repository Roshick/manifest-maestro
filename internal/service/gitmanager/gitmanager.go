package gitmanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"github.com/Roshick/manifest-maestro/pkg/utils/targz"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Git interface {
	FetchReferences(context.Context, string) ([]*plumbing.Reference, error)

	CloneCommit(context.Context, string, string) (*git.Repository, error)
}

type GitManager struct {
	git Git

	cache cache.Cache[[]byte]
}

func New(
	git Git,
	cache cache.Cache[[]byte],
) *GitManager {
	return &GitManager{
		git:   git,
		cache: cache,
	}
}

func (m *GitManager) RetrieveRepositoryToFileSystem(ctx context.Context, url string, referenceOrHash string) (*filesystem.FileSystem, error) {
	hash, err := m.ToHash(ctx, url, referenceOrHash)
	if err != nil {
		return nil, err
	}

	fileSystem := filesystem.New()
	repoTarball, err := m.RetrieveRepository(ctx, url, hash)
	if err != nil {
		return nil, err
	}
	if err = targz.Extract(ctx, fileSystem, bytes.NewBuffer(repoTarball), fileSystem.Root); err != nil {
		return nil, err
	}
	return fileSystem, nil
}

func (m *GitManager) RetrieveRepository(ctx context.Context, url string, referenceOrHash string) ([]byte, error) {
	hash, err := m.ToHash(ctx, url, referenceOrHash)
	if err != nil {
		return nil, err
	}

	key := cacheKey(url, hash)
	cached, err := m.cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if cached != nil {
		aulogging.Logger.Ctx(ctx).Info().Printf("cache hit for git repository with key '%s'", key)
		return *cached, nil
	}
	aulogging.Logger.Ctx(ctx).Info().Printf("cache miss for git repository with key '%s', retrieving from remote", key)
	return m.RefreshRepository(ctx, url, hash)
}

func (m *GitManager) RefreshRepository(ctx context.Context, url string, reference string) ([]byte, error) {
	hash, err := m.ToHash(ctx, url, reference)
	if err != nil {
		return nil, err
	}

	repoTarball, err := m.fetchAsTarball(ctx, url, hash)
	key := cacheKey(url, hash)
	if err = m.cache.Set(ctx, key, repoTarball, time.Hour); err != nil {
		aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to cache git repository with key '%s'", key)
	} else {
		aulogging.Logger.Ctx(ctx).Info().Printf("successfully cached git repository with key '%s'", key)
	}

	return repoTarball, nil
}

func (m *GitManager) ToHash(ctx context.Context, url string, referenceOrHash string) (string, error) {
	if !IsFullyQualifiedReference(referenceOrHash) && !IsCommitHash(referenceOrHash) {
		return "", fmt.Errorf("ToDo invalid reference %s", referenceOrHash)
	}

	if IsCommitHash(referenceOrHash) {
		return referenceOrHash, nil
	}

	remoteReferences, err := m.git.FetchReferences(ctx, url)
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

func (m *GitManager) fetchAsTarball(ctx context.Context, url string, hash string) ([]byte, error) {
	repo, err := m.git.CloneCommit(ctx, url, hash)
	if err != nil {
		return nil, err
	}
	tree, err := repo.Worktree()
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
	return repoBuffer.Bytes(), nil
}

func IsFullyQualifiedReference(reference string) bool {
	return reference == "HEAD" ||
		strings.HasPrefix(reference, "refs/heads/") ||
		strings.HasPrefix(reference, "refs/tags/")
}

func IsCommitHash(reference string) bool {
	// ToDo: auslagern
	r := regexp.MustCompile("[[:xdigit:]]{40}")
	return r.MatchString(reference)
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
