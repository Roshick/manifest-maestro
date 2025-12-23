package kustomize

import (
	"context"
	"errors"
	"fmt"
	"strings"

	openapi "github.com/Roshick/manifest-maestro-api"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/yaml"
)

type KustomizationRenderer struct{}

func NewKustomizationRenderer() *KustomizationRenderer {
	return &KustomizationRenderer{}
}

func (k *KustomizationRenderer) render(ctx context.Context, kustomization *Kustomization, parameters *openapi.KustomizeRenderParameters) ([]openapi.Manifest, error) {
	manifest, err := k.Render(ctx, kustomization, parameters)
	if err != nil {
		return nil, NewKustomizationRenderError(err)
	}
	return manifest, nil
}

func (k *KustomizationRenderer) Render(_ context.Context, kustomization *Kustomization, parameters *openapi.KustomizeRenderParameters) ([]openapi.Manifest, error) {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	for _, injection := range parameters.ManifestInjections {
		if injection.FileName == "" {
			return nil, errors.New("filename cannot be empty")
		}
		if strings.Contains(injection.FileName, kustomization.fileSystem.Separator) {
			return nil, fmt.Errorf("filename cannot contain %s", kustomization.fileSystem.Separator)
		}

		yamlDocs := make([]string, 0)
		for _, manifest := range injection.Manifests {
			yamlBytes, innerErr := yaml.Marshal(manifest.Content)
			if innerErr != nil {
				return nil, innerErr
			}
			yamlDocs = append(yamlDocs, string(yamlBytes))
		}
		fileContent := []byte(strings.Join(yamlDocs, "---\n"))

		if err := kustomization.fileSystem.WriteFile(kustomization.fileSystem.Join(kustomization.targetPath, injection.FileName), fileContent); err != nil {
			return nil, err
		}
	}

	manifests, err := kustomizer.Run(kustomization.fileSystem, kustomization.targetPath)
	if err != nil {
		return nil, err
	}

	parsedManifests := make([]openapi.Manifest, 0)
	for _, content := range manifests.Resources() {
		contentBytes, innerErr := content.AsYAML()
		if innerErr != nil {
			return nil, innerErr
		}
		parsedContent := make(map[string]any)
		if innerErr = yaml.Unmarshal(contentBytes, &parsedContent); innerErr != nil {
			return nil, innerErr
		}
		if parsedContent == nil || len(parsedContent) == 0 {
			continue
		}
		parsedManifests = append(parsedManifests, openapi.Manifest{
			Content: parsedContent,
		})
	}
	return parsedManifests, nil
}
