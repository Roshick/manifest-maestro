package client

import (
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
)

// GithubAuthTransport authenticates requests as the GitHub App (for /app/installations endpoints) or as the
// configured installation. The underlying transports are created once, so that the installation token is
// cached and only refreshed shortly before it expires.
type GithubAuthTransport struct {
	appsTransport         *ghinstallation.AppsTransport
	installationTransport *ghinstallation.Transport
}

func NewGitHubAuthTransport(
	rt http.RoundTripper,
	appID int64,
	appInstallationID int64,
	privateKey *rsa.PrivateKey,
) *GithubAuthTransport {
	if rt == nil {
		rt = http.DefaultTransport
	}

	appsTransport := ghinstallation.NewAppsTransportFromPrivateKey(rt, appID, privateKey)
	return &GithubAuthTransport{
		appsTransport:         appsTransport,
		installationTransport: ghinstallation.NewFromAppsTransport(appsTransport, appInstallationID),
	}
}

func (t *GithubAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasPrefix(req.URL.Path, "/app/installations") {
		return t.appsTransport.RoundTrip(req)
	}
	return t.installationTransport.RoundTrip(req)
}
