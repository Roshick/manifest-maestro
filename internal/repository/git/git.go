package git

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
)

type AuthProviderFn func(context.Context) (transport.AuthMethod, error)

type Git struct {
	authProviderFn AuthProviderFn

	commitHashRegex *regexp.Regexp
}

func New(
	authProviderFn AuthProviderFn,
) (*Git, error) {
	return &Git{
		authProviderFn:  authProviderFn,
		commitHashRegex: regexp.MustCompile("[[:xdigit:]]{40}"),
	}, nil
}

func (g *Git) RemoteReferences(ctx context.Context, repositoryURL string) ([]*plumbing.Reference, error) {
	repositoryURL = mapURL(repositoryURL)

	auth, err := g.authProviderFn(ctx)
	if err != nil {
		return nil, err
	}

	rem := git.NewRemote(memory.NewStorage(), &gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{repositoryURL},
	})

	references, err := rem.List(&git.ListOptions{
		Auth: auth,
	})
	if err != nil {
		if errors.As(err, new(*url.Error)) ||
			strings.HasPrefix(err.Error(), "authentication required") ||
			strings.HasPrefix(err.Error(), "unsupported scheme") ||
			strings.HasPrefix(err.Error(), "repository not found") {
			return nil, NewRepositoryNotFoundError(repositoryURL)
		}
		return nil, err
	}
	return references, nil
}

func (g *Git) CloneCommit(ctx context.Context, repositoryURL string, reference string) (*git.Repository, error) {
	repositoryURL = mapURL(repositoryURL)

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
	if err = repo.Fetch(&git.FetchOptions{
		Auth:     auth,
		RefSpecs: []gitConfig.RefSpec{gitConfig.RefSpec(refSpec)},
		Depth:    1,
	}); err != nil {
		if errors.As(err, new(*url.Error)) ||
			strings.HasPrefix(err.Error(), "unsupported scheme") ||
			strings.HasPrefix(err.Error(), "repository not found") {
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

func mapURL(url string) string {
	return strings.Replace(url, "git@github.com:", "https://github.com/", 1)
}
