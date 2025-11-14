package gitmock

import (
	"context"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"io"
	"os"
	"path/filepath"
)

type Impl struct {
	mockRemoteReferencesValues map[string][]*plumbing.Reference
	mockCloneCommitValues      map[string]*git.Repository
}

func New() *Impl {
	return &Impl{}
}

func (c *Impl) SetupFilesystem() error {
	panic("implement me")
}

func (c *Impl) RemoteReferences(_ context.Context, _ string) ([]*plumbing.Reference, error) {
	panic("implement me")
}

func (c *Impl) CloneCommit(_ context.Context, _ string, _ string) (*git.Repository, error) {
	panic("implement me")
}

func (c *Impl) copyToMemory(origin billy.Filesystem) (billy.Filesystem, error) {
	var copyRecursively func(origin, memory billy.Filesystem, currentPath string) error
	copyRecursively = func(origin, memory billy.Filesystem, currentPath string) error {
		files, err := origin.ReadDir(currentPath)
		if err != nil {
			return err
		}

		for _, file := range files {
			fileName := filepath.Join(currentPath, file.Name())
			if file.IsDir() {
				if innerErr := memory.MkdirAll(fileName, os.ModePerm); innerErr != nil {
					return innerErr
				}
				if innerErr := copyRecursively(origin, memory, fileName); innerErr != nil {
					return innerErr
				}
			} else {
				src, innerErr := origin.Open(fileName)
				if innerErr != nil {
					return innerErr
				}

				dst, innerErr := memory.Create(fileName)
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

	newFs := memfs.New()
	if err := copyRecursively(origin, newFs, ""); err != nil {
		return nil, err
	}
	return newFs, nil
}
