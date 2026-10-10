package git

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/google/go-github/v92/github"
)

type GitHubAppAuthProvider struct {
	client            *github.Client
	appInstallationID int64

	mu    sync.Mutex
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
	p.mu.Lock()
	defer p.mu.Unlock()

	next30Seconds := time.Now().Add(30 * time.Second)
	if p.token == nil || p.token.GetExpiresAt().Before(next30Seconds) {
		token, _, err := p.client.Apps.CreateInstallationToken(ctx, p.appInstallationID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create installation token: %w", err)
		}
		p.token = token
	}

	return &http.BasicAuth{
		Username: "x-access-token",
		Password: p.token.GetToken(),
	}, nil
}
