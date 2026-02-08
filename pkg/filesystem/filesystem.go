package filesystem

import (
	"io"
	"path/filepath"

	"github.com/go-git/go-billy/v5"

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func (f *FileSystem) SkipDir() error {
	return filepath.SkipDir
}

func (f *FileSystem) Dir(path string) string {
	return filepath.Dir(path)
}

func (f *FileSystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (f *FileSystem) IsAbs(path string) bool {
	cleanPath := filepath.Clean(path)
	return len(cleanPath) > 0 && string(cleanPath[0]) == f.Root
}

func New() *FileSystem {
	return &FileSystem{
		Root:       string(filepath.Separator),
		Separator:  string(filepath.Separator),
		FileSystem: filesys.MakeFsInMemory(),
	}
}

type FileSystem struct {
	Root      string
	Separator string
	filesys.FileSystem
}

func CopyFromBillyToFileSystem(origin billy.Filesystem, fileSystem *FileSystem) error {
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
