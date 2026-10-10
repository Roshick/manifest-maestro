package git

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/Roshick/manifest-maestro/internal/utils"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type AuthProviderFn func(context.Context) (transport.AuthMethod, error)

// URLResolverFn validates a repository url and maps it to the url used for network access. It must
// reject any url whose host must not receive the credentials returned by the AuthProviderFn.
type URLResolverFn func(repositoryURL string) (string, error)

type Git struct {
	authProviderFn AuthProviderFn
	urlResolverFn  URLResolverFn

	commitHashRegex *regexp.Regexp
}

type Option func(*Git)

// WithURLResolver replaces the default resolver, which only permits repositories on github.com.
func WithURLResolver(fn URLResolverFn) Option {
	return func(g *Git) { g.urlResolverFn = fn }
}

func New(
	authProviderFn AuthProviderFn,
	opts ...Option,
) (*Git, error) {
	g := &Git{
		authProviderFn:  authProviderFn,
		urlResolverFn:   utils.NormalizeGitHubRepositoryURL,
		commitHashRegex: regexp.MustCompile("^[[:xdigit:]]{40}$"),
	}
	for _, opt := range opts {
		opt(g)
	}
	return g, nil
}

func (g *Git) RemoteReferences(ctx context.Context, repositoryURL string) ([]*plumbing.Reference, error) {
	repositoryURL, err := g.resolveURL(repositoryURL)
	if err != nil {
		return nil, err
	}

	auth, err := g.authProviderFn(ctx)
	if err != nil {
		return nil, err
	}

	rem := git.NewRemote(memory.NewStorage(), &gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{repositoryURL},
	})

	references, err := rem.ListContext(ctx, &git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		if isRepositoryNotAccessible(err) {
			return nil, NewRepositoryNotFoundError(repositoryURL)
		}
		return nil, err
	}
	return references, nil
}

func (g *Git) CloneCommit(ctx context.Context, repositoryURL string, reference string) (*git.Repository, error) {
	repositoryURL, err := g.resolveURL(repositoryURL)
	if err != nil {
		return nil, err
	}
	if !g.isCommitHash(reference) {
		return nil, fmt.Errorf("reference '%s' is not a commit hash", reference)
	}

	auth, err := g.authProviderFn(ctx)
	if err != nil {
		return nil, err
	}

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		return nil, err
	}

	if _, err = repo.CreateRemote(&gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{repositoryURL},
	}); err != nil {
		return nil, err
	}

	localBranch := "refs/heads/local"
	refSpec := fmt.Sprintf("%s:%s", reference, localBranch)
	if err = repo.FetchContext(ctx, &git.FetchOptions{
		Auth:     auth,
		RefSpecs: []gitConfig.RefSpec{gitConfig.RefSpec(refSpec)},
		Depth:    1,
	}); err != nil {
		if isRepositoryNotAccessible(err) {
			return nil, NewRepositoryNotFoundError(repositoryURL)
		}
		return nil, err
	}

	tree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	err = tree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(localBranch),
	})
	if err != nil {
		return nil, err
	}

	return repo, nil
}

func (g *Git) ToHash(ctx context.Context, repositoryURL string, gitReference string) (string, error) {
	if g.isCommitHash(gitReference) {
		return gitReference, nil
	}

	remoteReferences, err := g.RemoteReferences(ctx, repositoryURL)
	if err != nil {
		return "", err
	}
	for _, ref := range remoteReferences {
		if ref.Name().String() == gitReference {
			return ref.Hash().String(), nil
		}
	}
	return "", NewRepositoryReferenceNotFoundError(repositoryURL, gitReference)
}

func (g *Git) isCommitHash(gitReference string) bool {
	return g.commitHashRegex.MatchString(gitReference)
}

func (g *Git) resolveURL(repositoryURL string) (string, error) {
	resolvedURL, err := g.urlResolverFn(repositoryURL)
	if err != nil {
		return "", NewRepositoryURLInvalidError(err)
	}
	return resolvedURL, nil
}

func isRepositoryNotAccessible(err error) bool {
	return errors.Is(err, transport.ErrRepositoryNotFound) ||
		errors.Is(err, transport.ErrAuthenticationRequired) ||
		errors.Is(err, transport.ErrAuthorizationFailed) ||
		errors.Is(err, transport.ErrEmptyRemoteRepository)
}
