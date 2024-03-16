package filesystem

import (
	"path/filepath"

	"sigs.k8s.io/kustomize/api/filesys"
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
