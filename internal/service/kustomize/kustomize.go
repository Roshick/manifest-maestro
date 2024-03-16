package kustomize

import (
	"context"
	"fmt"
	"strings"

	apimodel "github.com/Roshick/manifest-maestro/api"
	"github.com/Roshick/manifest-maestro/pkg/utils/filesystem"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/yaml"
)

type Kustomize struct{}

func New() *Kustomize {
	return &Kustomize{}
}

func (k *Kustomize) Render(
	_ context.Context, fileSystem *filesystem.FileSystem, targetPath string, parameters *apimodel.KustomizeRenderParameters,
) ([]apimodel.Manifest, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	for _, injection := range parameters.ManifestInjections {
		if injection.FileName == "" {
			return nil, fmt.Errorf("ToDo: filename cannot be empty")
		}
		if strings.Contains(injection.FileName, fileSystem.Separator) {
			return nil, fmt.Errorf("ToDo: filename cannot contain %k", fileSystem.Separator)
		}

		yamlDocs := make([]string, 0)
		for _, manifest := range injection.Manifests {
			yamlBytes, innerErr := yaml.Marshal(manifest)
			if innerErr != nil {
				return nil, innerErr
			}
			yamlDocs = append(yamlDocs, string(yamlBytes))
		}
		fileContent := []byte(strings.Join(yamlDocs, "---"))

		if err := fileSystem.WriteFile(fileSystem.Join(targetPath, injection.FileName), fileContent); err != nil {
			return nil, err
		}
	}

	manifests, err := kustomizer.Run(fileSystem, targetPath)
	if err != nil {
		// ToDo: customize errors
		err = fmt.Errorf("ToDo: %w", err)
		return nil, err
	}

	parsedManifests := make([]apimodel.Manifest, 0)
	for _, content := range manifests.Resources() {
		contentBytes, innerErr := content.AsYAML()
		if innerErr != nil {
			return nil, innerErr
		}

		parsedContent := make(map[string]any)
		if innerErr = yaml.Unmarshal(contentBytes, &parsedContent); innerErr != nil {
			return nil, innerErr
		}

		parsedManifests = append(parsedManifests, apimodel.Manifest{
			Content: parsedContent,
		})
	}
	return parsedManifests, nil
}
