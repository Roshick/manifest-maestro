package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Roshick/go-autumn-web/auth"
	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func newTestRouter(t *testing.T, authFns []auth.AuthorizationFn) chi.Router {
	t.Helper()
	r := chi.NewRouter()
	NewV1Controller(nil, authFns, nil, nil, nil, nil).WireUp(t.Context(), r)
	return r
}

func doRequest(r http.Handler, path string, body []byte, username string, password string) int {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func TestV1Controller_RequiresBasicAuthIfConfigured(t *testing.T) {
	r := newTestRouter(t, []auth.AuthorizationFn{auth.AllowBasicAuthUser(auth.AllowBasicAuthUserOptions{Username: "user", Password: "secret"})})
	path := "/rest/api/v1/helm/actions/render-chart"

	require.Equal(t, http.StatusUnauthorized, doRequest(r, path, []byte("{}"), "", ""))
	require.Equal(t, http.StatusUnauthorized, doRequest(r, path, []byte("{}"), "user", "wrong"))
	require.Equal(t, http.StatusBadRequest, doRequest(r, path, []byte("{"), "user", "secret"))
}

func TestV1Controller_RejectsOversizedBodies(t *testing.T) {
	r := newTestRouter(t, nil)
	body := []byte(`{"reference":{},"parameters":{"valuesFlat":["` + strings.Repeat("a", maxRequestBodySize) + `"]}}`)

	require.Equal(t, http.StatusBadRequest, doRequest(r, "/rest/api/v1/helm/actions/render-chart", body, "", ""))
}

func TestV1Controller_ListChartVersionsIsRouted(t *testing.T) {
	r := newTestRouter(t, nil)
	body := []byte(`{"reference":{}}`)

	status := doRequest(r, "/rest/api/v1/helm/actions/list-chart-versions", body, "", "")
	require.NotContains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, status)
}

func TestProfilerController_RequiresAuth(t *testing.T) {
	for desc, authFns := range map[string][]auth.AuthorizationFn{
		"disabled":   nil,
		"configured": {auth.AllowBasicAuthUser(auth.AllowBasicAuthUserOptions{Username: "user", Password: "secret"})},
	} {
		r := chi.NewRouter()
		NewProfilerController(authFns).WireUp(t.Context(), r)

		req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		require.Equal(t, http.StatusUnauthorized, rec.Code, desc)
	}

	r := chi.NewRouter()
	NewProfilerController([]auth.AuthorizationFn{auth.AllowBasicAuthUser(auth.AllowBasicAuthUserOptions{Username: "user", Password: "secret"})}).WireUp(t.Context(), r)
	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil)
	req.SetBasicAuth("user", "secret")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}
