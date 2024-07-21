package kustomize

import "github.com/Roshick/manifest-maestro/pkg/utils/filesystem"

type Kustomization struct {
	fileSystem *filesystem.FileSystem
	targetPath string
}
