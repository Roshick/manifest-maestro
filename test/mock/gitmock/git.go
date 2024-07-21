package gitmock

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
)

type Impl struct {
	currentRef string
	refsFs     map[string]billy.Filesystem
}

func New() *Impl {
	return &Impl{
		refsFs: make(map[string]billy.Filesystem),
	}
}

func (c *Impl) SetupFilesystem() error {
	root := "../resources/git-mocks"
	rootFileSystem := osfs.New(root)
	files, err := rootFileSystem.ReadDir("")
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath := filepath.Join(root, strings.ReplaceAll(file.Name(), "%", "%%"))
		if file.IsDir() {
			origin := osfs.New(fmt.Sprintf(filePath))
			fileSystem, innerErr := c.copyToMemory(origin)
			if innerErr != nil {
				return err
			}
			reference, innerErr := url.QueryUnescape(file.Name())
			if innerErr != nil {
				return innerErr
			}
			c.refsFs[reference] = fileSystem
		}
	}
	return nil
}

func (c *Impl) CloneCommit(
	_ context.Context,
	gitURL string,
	gitReference string,
	targetPath string,
) (*git.Repository, error) {
	c.currentRef = gitReference
	fsRef, ok := c.refsFs[c.currentRef]
	if !ok {
		return nil, fmt.Errorf("ToDo")
	}
	return git.Init(memory.NewStorage(), fsRef)
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
