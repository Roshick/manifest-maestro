package client

import (
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

type GithubAuthTransport struct {
	base http.RoundTripper

	appID             int64
	appInstallationID int64
	privateKey        *rsa.PrivateKey
}

func NewGitHubAuthTransport(rt http.RoundTripper, appID int64, appInstallationID int64, privateKey *rsa.PrivateKey) *GithubAuthTransport {
	if rt == nil {
		rt = http.DefaultTransport
	}

	return &GithubAuthTransport{
		base:              rt,
		appID:             appID,
		appInstallationID: appInstallationID,
		privateKey:        privateKey,
	}
}

func (t *GithubAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	rt1 := ghinstallation.NewAppsTransportFromPrivateKey(t.base, t.appID, t.privateKey)
	if strings.HasPrefix(req.URL.Path, "/app/installations") {
		return rt1.RoundTrip(req)
	}

	rt2 := ghinstallation.NewFromAppsTransport(rt1, t.appInstallationID)
	return rt2.RoundTrip(req)
}
