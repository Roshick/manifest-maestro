package kustomize

import (
	"errors"
	"testing"

	openapi "github.com/Roshick/manifest-maestro-api"
	"github.com/Roshick/manifest-maestro/pkg/filesystem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestKustomization(t *testing.T, files map[string]string) *Kustomization {
	t.Helper()
	fileSystem := filesystem.New()
	for path, content := range files {
		fullPath := fileSystem.Join(fileSystem.Root, path)
		require.NoError(t, fileSystem.MkdirAll(fileSystem.Dir(fullPath)))
		require.NoError(t, fileSystem.WriteFile(fullPath, []byte(content)))
	}
	return &Kustomization{
		fileSystem: fileSystem,
		targetPath: fileSystem.Root,
	}
}

func TestKustomizationRenderer_Render_NilParameters(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{
		"kustomization.yaml": `resources:
  - deployment.yaml
`,
		"deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
`,
	})

	renderer := NewKustomizationRenderer()
	manifests, err := renderer.Render(t.Context(), kustomization, nil)
	require.NoError(t, err)
	require.Len(t, manifests, 1)
	assert.Equal(t, "Deployment", manifests[0].Content["kind"])
}

func TestKustomizationRenderer_Render_WithManifestInjection(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{
		"kustomization.yaml": `resources:
  - deployment.yaml
  - injected.yaml
`,
		"deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
`,
	})

	parameters := &openapi.KustomizeRenderParameters{
		ManifestInjections: []openapi.KustomizeManifestInjection{
			{
				FileName: "injected.yaml",
				Manifests: []openapi.Manifest{
					{
						Content: map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata":   map[string]any{"name": "injected-config"},
						},
					},
				},
			},
		},
	}

	renderer := NewKustomizationRenderer()
	manifests, err := renderer.Render(t.Context(), kustomization, parameters)
	require.NoError(t, err)
	require.Len(t, manifests, 2)

	kinds := make([]string, 0, len(manifests))
	for _, manifest := range manifests {
		kinds = append(kinds, manifest.Content["kind"].(string))
	}
	assert.Contains(t, kinds, "Deployment")
	assert.Contains(t, kinds, "ConfigMap")
}

func TestKustomizationRenderer_Render_EmptyInjectionFileName(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{
		"kustomization.yaml": "resources: []\n",
	})

	parameters := &openapi.KustomizeRenderParameters{
		ManifestInjections: []openapi.KustomizeManifestInjection{
			{FileName: ""},
		},
	}

	renderer := NewKustomizationRenderer()
	_, err := renderer.Render(t.Context(), kustomization, parameters)
	require.Error(t, err)

	renderError, ok := errors.AsType[*KustomizationRenderError](err)
	require.True(t, ok)
	assert.Contains(t, renderError.Error(), "filename cannot be empty")
}

func TestKustomizationRenderer_Render_InjectionFileNameWithSeparator(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{
		"kustomization.yaml": "resources: []\n",
	})

	parameters := &openapi.KustomizeRenderParameters{
		ManifestInjections: []openapi.KustomizeManifestInjection{
			{FileName: "subdir/injected.yaml"},
		},
	}

	renderer := NewKustomizationRenderer()
	_, err := renderer.Render(t.Context(), kustomization, parameters)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filename cannot contain")
}

func TestKustomizationRenderer_Render_MissingKustomizationFile(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{})

	renderer := NewKustomizationRenderer()
	_, err := renderer.Render(t.Context(), kustomization, nil)
	require.Error(t, err)

	_, ok := errors.AsType[*KustomizationRenderError](err)
	assert.True(t, ok)
}
