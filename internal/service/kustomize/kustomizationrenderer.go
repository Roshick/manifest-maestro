package kustomize

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	openapi "github.com/Roshick/manifest-maestro-api"
	"sigs.k8s.io/kustomize/api/krusty"
)

type KustomizationRenderer struct{}

func NewKustomizationRenderer() *KustomizationRenderer {
	return &KustomizationRenderer{}
}

func (k *KustomizationRenderer) Render(
	ctx context.Context,
	kustomization *Kustomization,
	parameters *openapi.KustomizeRenderParameters,
) ([]openapi.Manifest, error) {
	manifest, err := k.render(ctx, kustomization, parameters)
	if err != nil {
		return nil, NewKustomizationRenderError(err)
	}
	return manifest, nil
}

func (k *KustomizationRenderer) render(
	_ context.Context,
	kustomization *Kustomization,
	parameters *openapi.KustomizeRenderParameters,
) ([]openapi.Manifest, error) {
	actualParameters := openapi.KustomizeRenderParameters{}
	if parameters != nil {
		actualParameters = *parameters
	}

	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	for _, injection := range actualParameters.ManifestInjections {
		if injection.FileName == "" {
			return nil, errors.New("filename cannot be empty")
		}
		if injection.FileName == "." || injection.FileName == ".." {
			return nil, fmt.Errorf("filename cannot be '%s'", injection.FileName)
		}
		if strings.Contains(injection.FileName, kustomization.fileSystem.Separator) {
			return nil, fmt.Errorf("filename cannot contain %s", kustomization.fileSystem.Separator)
		}

		// JSON is valid YAML and much cheaper to produce than YAML
		var fileContent []byte
		for i, manifest := range injection.Manifests {
			if i > 0 {
				fileContent = append(fileContent, "\n---\n"...)
			}
			jsonBytes, innerErr := json.Marshal(manifest.Content)
			if innerErr != nil {
				return nil, innerErr
			}
			fileContent = append(fileContent, jsonBytes...)
		}

		if err := kustomization.fileSystem.WriteFile(
			kustomization.fileSystem.Join(kustomization.targetPath, injection.FileName),
			fileContent,
		); err != nil {
			return nil, err
		}
	}

	if err := rejectRemoteReferences(kustomization.fileSystem, kustomization.targetPath); err != nil {
		return nil, err
	}

	manifests, err := kustomizer.Run(kustomization.fileSystem, kustomization.targetPath)
	if err != nil {
		return nil, err
	}

	parsedManifests := make([]openapi.Manifest, 0)
	for _, content := range manifests.Resources() {
		// decodes the YAML node directly instead of converting it to YAML text and back
		parsedContent, innerErr := content.Map()
		if innerErr != nil {
			return nil, innerErr
		}
		if len(parsedContent) == 0 {
			continue
		}
		parsedManifests = append(parsedManifests, openapi.Manifest{
			Content: parsedContent,
		})
	}
	return parsedManifests, nil
}
