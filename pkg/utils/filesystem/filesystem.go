package filesystem

import (
	"path/filepath"

	"sigs.k8s.io/kustomize/api/filesys"
)

func (*FileSystem) SkipDir() error {
	return filepath.SkipDir
}

func (*FileSystem) Dir(path string) string {
	return filepath.Dir(path)
}

func (*FileSystem) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (*FileSystem) IsAbs(path string) bool {
	return len(path) > 0 && path[0] == filepath.Separator
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
