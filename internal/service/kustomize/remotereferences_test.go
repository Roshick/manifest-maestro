package kustomize

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKustomizationRenderer_Render_RejectsRemoteReferences(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte("internal-secret"))
	}))
	defer srv.Close()

	kustomizations := map[string]map[string]string{
		"resource":       {"kustomization.yaml": "resources:\n  - " + srv.URL + "/r.yaml\n"},
		"configmap file": {"kustomization.yaml": "configMapGenerator:\n  - name: c\n    files:\n      - key=" + srv.URL + "/f\n"},
		"configmap env":  {"kustomization.yaml": "configMapGenerator:\n  - name: c\n    envs:\n      - " + srv.URL + "/e\n"},
		"patch":          {"kustomization.yaml": "patches:\n  - path: " + srv.URL + "/p.yaml\n"},
		"git":            {"kustomization.yaml": "resources:\n  - git@github.com:org/repo.git\n"},
		"nested base": {
			"kustomization.yaml":      "resources:\n  - base\n",
			"base/kustomization.yaml": "resources:\n  - HTTPS://example.com/r.yaml\n",
		},
	}
	for desc, files := range kustomizations {
		_, err := NewKustomizationRenderer().Render(t.Context(), newTestKustomization(t, files), nil)
		require.ErrorContains(t, err, "only local files are supported", desc)
	}
	require.Zero(t, requests.Load())
}

func TestKustomizationRenderer_Render_AllowsURLsInNonPathFields(t *testing.T) {
	kustomization := newTestKustomization(t, map[string]string{
		"kustomization.yaml": `commonAnnotations:
  link: https://example.com
resources:
  - deployment.yaml
`,
		"deployment.yaml": `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment
`,
	})

	manifests, err := NewKustomizationRenderer().Render(t.Context(), kustomization, nil)
	require.NoError(t, err)
	require.Len(t, manifests, 1)
}
