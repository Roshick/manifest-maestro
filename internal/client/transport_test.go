package client

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGitHubAuthTransportCachesInstallationToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var tokenRequests, apiRequests, unauthenticated atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/access_tokens") {
			tokenRequests.Add(1)
			w.WriteHeader(http.StatusCreated)
			_, _ = fmt.Fprintf(w, `{"token":"installation-token","expires_at":"%s"}`, time.Now().Add(time.Hour).Format(time.RFC3339))
			return
		}
		apiRequests.Add(1)
		if r.Header.Get("Authorization") != "token installation-token" {
			unauthenticated.Add(1)
		}
	}))
	defer srv.Close()

	rt := NewGitHubAuthTransport(nil, 1, 2, key)
	rt.installationTransport.BaseURL = srv.URL
	client := &http.Client{Transport: rt}

	for range 5 {
		resp, getErr := client.Get(srv.URL + "/repos/o/r")
		require.NoError(t, getErr)
		_ = resp.Body.Close()
	}

	require.Equal(t, int32(5), apiRequests.Load())
	require.Equal(t, int32(1), tokenRequests.Load())
	require.Zero(t, unauthenticated.Load())
}
