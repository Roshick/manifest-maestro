package kustomize

import (
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
)

type Kustomization struct {
	fileSystem *filesystem.FileSystem
	targetPath string
}
