package gitmock

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

// Mock is a testable implementation of cache.Git.
type Mock struct {
	// ToHashCallCount tracks how many times ToHash was called.
	ToHashCallCount atomic.Int32

	// CloneCommitCallCount tracks how many times CloneCommit was called.
	CloneCommitCallCount atomic.Int32

	// toHashFn is the custom handler for ToHash.
	toHashFn func(ctx context.Context, repositoryURL string, gitReference string) (string, error)

	// cloneCommitFn is the custom handler for CloneCommit.
	cloneCommitFn func(ctx context.Context, repositoryURL string, reference string) (*git.Repository, error)
}

func NewMock() *Mock {
	return &Mock{}
}

// WithToHash sets a custom handler for ToHash calls.
func (m *Mock) WithToHash(fn func(ctx context.Context, repositoryURL string, gitReference string) (string, error)) *Mock {
	m.toHashFn = fn
	return m
}

// WithCloneCommit sets a custom handler for CloneCommit calls.
func (m *Mock) WithCloneCommit(fn func(ctx context.Context, repositoryURL string, reference string) (*git.Repository, error)) *Mock {
	m.cloneCommitFn = fn
	return m
}

func (m *Mock) ToHash(ctx context.Context, repositoryURL string, gitReference string) (string, error) {
	m.ToHashCallCount.Add(1)
	if m.toHashFn != nil {
		return m.toHashFn(ctx, repositoryURL, gitReference)
	}
	// Default: return the reference itself as a hash-like string
	return gitReference, nil
}

func (m *Mock) CloneCommit(ctx context.Context, repositoryURL string, reference string) (*git.Repository, error) {
	m.CloneCommitCallCount.Add(1)
	if m.cloneCommitFn != nil {
		return m.cloneCommitFn(ctx, repositoryURL, reference)
	}
	return createEmptyRepo()
}

// CreateRepoFromDir creates a git.Repository in memory with files from a local directory.
func CreateRepoFromDir(dir string) (*git.Repository, error) {
	fs := memfs.New()

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, innerErr := filepath.Rel(dir, path)
		if innerErr != nil {
			return innerErr
		}
		if relPath == "." {
			return nil
		}
		if info.IsDir() {
			return fs.MkdirAll(relPath, os.ModePerm)
		}
		return copyFile(path, relPath, fs)
	})
	if err != nil {
		return nil, err
	}

	store := memory.NewStorage()
	repo, err := git.Init(store, fs)
	if err != nil {
		return nil, err
	}

	tree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	if _, err = tree.Add("."); err != nil {
		return nil, err
	}

	if _, err = tree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	}); err != nil {
		return nil, err
	}

	return repo, nil
}

func createEmptyRepo() (*git.Repository, error) {
	store := memory.NewStorage()
	fs := memfs.New()
	repo, err := git.Init(store, fs)
	if err != nil {
		return nil, err
	}

	tree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	// Create a dummy file so we can commit
	f, err := fs.Create("README.md")
	if err != nil {
		return nil, err
	}
	if _, err = f.Write([]byte("# test")); err != nil {
		return nil, err
	}
	if err = f.Close(); err != nil {
		return nil, err
	}

	if _, err = tree.Add("README.md"); err != nil {
		return nil, err
	}

	if _, err = tree.Commit("initial", &git.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@test.com"},
	}); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return repo, nil
}

func copyFile(srcPath string, dstRelPath string, fs billy.Filesystem) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := fs.Create(dstRelPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}


