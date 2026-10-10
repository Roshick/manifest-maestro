package client

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewHTTPClientDoesNotLeakBasicAuthOnCrossOriginRedirect(t *testing.T) {
	var leaked atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Add(1)
		}
	}))
	defer foreign.Close()

	var authenticated atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, password, ok := r.BasicAuth(); ok && user == "user" && password == "secret" {
			authenticated.Add(1)
		}
		switch r.URL.Path {
		case "/cross-origin":
			http.Redirect(w, r, foreign.URL+"/target", http.StatusFound)
		case "/same-origin":
			http.Redirect(w, r, "/target", http.StatusFound)
		}
	}))
	defer origin.Close()

	opts := DefaultHTTPClientOptions()
	opts.BasicAuthOptions = &BasicAuthOptions{Username: "user", Password: "secret"}
	httpClient, err := NewFactory().NewHTTPClient("test", opts)
	require.NoError(t, err)

	resp, err := httpClient.Get(origin.URL + "/same-origin")
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, int32(2), authenticated.Load())

	_, err = httpClient.Get(origin.URL + "/cross-origin")
	require.ErrorContains(t, err, "cross-origin redirect")
	require.Zero(t, leaked.Load())
}
