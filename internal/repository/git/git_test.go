package git

import (
	"context"
	"testing"

	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/require"
)

func TestRejectsNonGitHubURLsBeforeAuthentication(t *testing.T) {
	authCalled := false
	g, err := New(func(_ context.Context) (transport.AuthMethod, error) {
		authCalled = true
		return nil, nil //nolint:nilnil // never reached
	})
	require.NoError(t, err)

	for _, repositoryURL := range []string{
		"https://attacker.example/org/repo.git",
		"https://github.com@attacker.example/org/repo.git",
		"ssh://git@attacker.example/org/repo.git",
		"file:///etc",
	} {
		_, err = g.RemoteReferences(context.Background(), repositoryURL)
		require.ErrorAs(t, err, new(*RepositoryURLInvalidError), repositoryURL)
		_, err = g.CloneCommit(context.Background(), repositoryURL, "0123456789012345678901234567890123456789")
		require.ErrorAs(t, err, new(*RepositoryURLInvalidError), repositoryURL)
	}
	require.False(t, authCalled)
}

func TestIsCommitHashRequiresExactHash(t *testing.T) {
	g, err := New(nil)
	require.NoError(t, err)

	require.True(t, g.isCommitHash("0123456789abcdef0123456789abcdef01234567"))
	require.False(t, g.isCommitHash("refs/heads/0123456789abcdef0123456789abcdef01234567"))
	require.False(t, g.isCommitHash("0123456789abcdef0123456789abcdef01234567:refs/heads/x"))
}

func TestCloneCommitRejectsNonHashReference(t *testing.T) {
	g, err := New(nil, WithURLResolver(func(repositoryURL string) (string, error) { return repositoryURL, nil }))
	require.NoError(t, err)

	_, err = g.CloneCommit(context.Background(), "file:///does-not-exist", "refs/heads/main")
	require.ErrorContains(t, err, "is not a commit hash")
}
