package git

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v78/github"
	"time"
)

type GitHubAppAuthProvider struct {
	client            *github.Client
	appInstallationID int64

	token *github.InstallationToken
}

func NewGitHubAppAuthProvider(
	client *github.Client,
	appInstallationID int64,
) *GitHubAppAuthProvider {
	return &GitHubAppAuthProvider{
		client:            client,
		appInstallationID: appInstallationID,
	}
}

func (p *GitHubAppAuthProvider) GetAuth(ctx context.Context) (transport.AuthMethod, error) {
	if p.token == nil || p.token.GetExpiresAt().Before(time.Now().Add(-30*time.Second)) {
		var err error
		p.token, _, err = p.client.Apps.CreateInstallationToken(ctx, p.appInstallationID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create installation token: %w", err)
		}
	}

	return &http.BasicAuth{
		Username: "x-access-token",
		Password: p.token.GetToken(),
	}, nil
}
