package kustomize

import (
	"encoding/json"
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

// TestKustomizationRenderer_Render_ValueTypes pins the JSON representation of rendered values that YAML parsers
// commonly treat differently (anchors, merge keys, numbers, booleans, timestamps, nulls, quoted scalars).
// Integers beyond float64 precision are kept exactly.
func TestKustomizationRenderer_Render_ValueTypes(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{
		"kustomization.yaml": `resources:
  - configmap.yaml
  - injected.yaml
labels:
  - pairs:
      team: platform
`,
		"configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: values
  annotations:
    quoted-number: "8080"
    quoted-bool: "true"
data:
  plain: text
  multiline: |
    line one
    line two
---
apiVersion: example.com/v1
kind: Values
metadata:
  name: typed
spec:
  defaults: &defaults
    replicas: 3
    enabled: true
  merged:
    <<: *defaults
    replicas: 5
  alias: *defaults
  int: 42
  negative: -7
  bigInt: 12345678901234567
  float: 1.5
  wholeFloat: 2.0
  exponent: 1e3
  octal: 0o17
  hex: 0x1F
  isYes: yes
  isOn: on
  bool: false
  isNull: null
  tilde: ~
  empty:
  timestamp: 2024-01-02T03:04:05Z
  date: 2024-01-02
  version: 1.10
  emptyList: []
  emptyMap: {}
  list:
    - 1
    - two
    - 3.5
    - null
`,
	})

	parameters := &openapi.KustomizeRenderParameters{
		ManifestInjections: []openapi.KustomizeManifestInjection{{
			FileName: "injected.yaml",
			Manifests: []openapi.Manifest{{Content: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "injected"},
				"data": map[string]any{
					"number":  json.Number("1"),
					"float":   1.25,
					"bool":    true,
					"isYes":   "yes",
					"isNull":  nil,
					"version": "1.10",
				},
			}}},
		}},
	}

	manifests, err := NewKustomizationRenderer().Render(t.Context(), kustomization, parameters)
	require.NoError(t, err)
	actual, err := json.Marshal(manifests)
	require.NoError(t, err)
	assert.Equal( //nolint:testifylint // exact bytes pin number formatting, which JSONEq would ignore
		t,
		`[{"content":{"apiVersion":"v1","data":{"multiline":"line one\nline two\n","plain":"text"},"kind":"ConfigMap","metadata":{"annotations":{"quoted-bool":"true","quoted-number":"8080"},"labels":{"team":"platform"},"name":"values"}}},{"content":{"apiVersion":"example.com/v1","kind":"Values","metadata":{"labels":{"team":"platform"},"name":"typed"},"spec":{"alias":{"enabled":true,"replicas":3},"bigInt":12345678901234567,"bool":false,"date":"2024-01-02T00:00:00Z","defaults":{"enabled":true,"replicas":3},"empty":null,"emptyList":[],"emptyMap":{},"exponent":1000,"float":1.5,"hex":31,"int":42,"isNull":null,"isOn":"on","isYes":"yes","list":[1,"two",3.5,null],"merged":{"enabled":true,"replicas":5},"negative":-7,"octal":15,"tilde":null,"timestamp":"2024-01-02T03:04:05Z","version":1.1,"wholeFloat":2}}},{"content":{"apiVersion":"v1","data":{"bool":true,"float":1.25,"isNull":null,"isYes":"yes","number":1,"version":"1.10"},"kind":"ConfigMap","metadata":{"labels":{"team":"platform"},"name":"injected"}}}]`,
		string(actual),
	)
}
