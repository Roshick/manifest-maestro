package kustomize

import (
	"context"
	"fmt"
	"strings"

	"github.com/Roshick/manifest-maestro/pkg/api"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/yaml"
)

type KustomizationRenderer struct{}

func NewKustomizationRenderer() *KustomizationRenderer {
	return &KustomizationRenderer{}
}

func (k *KustomizationRenderer) Render(_ context.Context, kustomization *Kustomization, parameters *api.KustomizeRenderParameters) ([]api.Manifest, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	for _, injection := range parameters.ManifestInjections {
		if injection.FileName == "" {
			return nil, fmt.Errorf("ToDo: filename cannot be empty")
		}
		if strings.Contains(injection.FileName, kustomization.fileSystem.Separator) {
			return nil, fmt.Errorf("ToDo: filename cannot contain %s", kustomization.fileSystem.Separator)
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

		if err := kustomization.fileSystem.WriteFile(kustomization.fileSystem.Join(kustomization.targetPath, injection.FileName), fileContent); err != nil {
			return nil, err
		}
	}

	manifests, err := kustomizer.Run(kustomization.fileSystem, kustomization.targetPath)
	if err != nil {
		err = fmt.Errorf("ToDo: %w", err)
		return nil, err
	}

	parsedManifests := make([]api.Manifest, 0)
	for _, content := range manifests.Resources() {
		contentBytes, innerErr := content.AsYAML()
		if innerErr != nil {
			return nil, innerErr
		}

		parsedContent := make(map[string]any)
		if innerErr = yaml.Unmarshal(contentBytes, &parsedContent); innerErr != nil {
			return nil, innerErr
		}

		parsedManifests = append(parsedManifests, api.Manifest{
			Content: parsedContent,
		})
	}
	return parsedManifests, nil
}
