package kustomize

import (
	"fmt"
	"regexp"
	"strings"

	"sigs.k8s.io/kustomize/api/konfig"
	"sigs.k8s.io/kustomize/api/types"

	"github.com/Roshick/manifest-maestro/pkg/filesystem"
)

var urlSchemeRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// rejectRemoteReferences ensures that neither the kustomization at rootPath nor any local kustomization
// it includes references remote files or repositories. Kustomize fetches such references with an
// unrestricted HTTP client (or git), which would allow requests to arbitrary, e.g. internal, hosts.
func rejectRemoteReferences(fileSystem *filesystem.FileSystem, rootPath string) error {
	visited := make(map[string]bool)
	var check func(dir string) error
	check = func(dir string) error {
		if visited[dir] {
			return nil
		}
		visited[dir] = true

		kustomization, filePath, err := readKustomization(fileSystem, dir)
		if err != nil || kustomization == nil {
			// unreadable kustomizations are reported by kustomize itself
			return nil //nolint:nilerr // see above
		}
		for _, path := range referencedPaths(kustomization) {
			if isRemoteReference(path) {
				return fmt.Errorf("kustomization '%s' references remote location '%s', only local files are supported", filePath, path)
			}
		}
		for _, path := range localKustomizationPaths(kustomization) {
			subDir := fileSystem.Join(dir, path)
			if fileSystem.IsDir(subDir) {
				if err = check(subDir); err != nil {
					return err
				}
			}
		}
		return nil
	}
	return check(rootPath)
}

func readKustomization(fileSystem *filesystem.FileSystem, dir string) (*types.Kustomization, string, error) {
	for _, fileName := range konfig.RecognizedKustomizationFileNames() {
		filePath := fileSystem.Join(dir, fileName)
		if !fileSystem.Exists(filePath) || fileSystem.IsDir(filePath) {
			continue
		}
		content, err := fileSystem.ReadFile(filePath)
		if err != nil {
			return nil, filePath, err
		}
		kustomization := &types.Kustomization{}
		if err = kustomization.Unmarshal(content); err != nil {
			return nil, filePath, err
		}
		kustomization.FixKustomization()
		return kustomization, filePath, nil
	}
	return nil, "", nil
}

// localKustomizationPaths returns all entries that may refer to further kustomizations.
func localKustomizationPaths(k *types.Kustomization) []string {
	paths := make([]string, 0, len(k.Resources)+len(k.Components)+len(k.Bases))
	paths = append(paths, k.Resources...)
	paths = append(paths, k.Components...)
	return append(paths, k.Bases...)
}

// referencedPaths returns all entries of a kustomization that kustomize loads as file or directory.
func referencedPaths(k *types.Kustomization) []string {
	paths := localKustomizationPaths(k)
	paths = append(paths, k.Crds...)
	paths = append(paths, k.Configurations...)
	paths = append(paths, k.Generators...)
	paths = append(paths, k.Transformers...)
	paths = append(paths, k.Validators...)
	for _, patch := range k.PatchesStrategicMerge {
		paths = append(paths, string(patch))
	}
	for _, patch := range append(k.Patches, k.PatchesJson6902...) {
		paths = append(paths, patch.Path)
	}
	for _, replacement := range k.Replacements {
		paths = append(paths, replacement.Path)
	}
	if path, ok := k.OpenAPI["path"]; ok {
		paths = append(paths, path)
	}
	generatorArgs := make([]types.GeneratorArgs, 0, len(k.ConfigMapGenerator)+len(k.SecretGenerator))
	for _, args := range k.ConfigMapGenerator {
		generatorArgs = append(generatorArgs, args.GeneratorArgs)
	}
	for _, args := range k.SecretGenerator {
		generatorArgs = append(generatorArgs, args.GeneratorArgs)
	}
	for _, args := range generatorArgs {
		for _, source := range args.FileSources {
			// file sources have the form [{key}=]{path}
			if _, path, found := strings.Cut(source, "="); found {
				paths = append(paths, path)
			}
			paths = append(paths, source)
		}
		paths = append(paths, args.EnvSources...)
		paths = append(paths, args.EnvSource)
	}
	return paths
}

// isRemoteReference reports paths that kustomize would fetch via HTTP or clone via git.
func isRemoteReference(path string) bool {
	path = strings.ToLower(strings.TrimSpace(path))
	return urlSchemeRegex.MatchString(path) || strings.HasPrefix(path, "git@") || strings.HasPrefix(path, "git::")
}
