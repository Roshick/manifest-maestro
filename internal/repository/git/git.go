package git

import (
	"context"
	"fmt"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/storage/memory"

	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/go-git/go-git/v5"
	gitConfig "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

type Git struct {
	sshAuth *ssh.PublicKeys
}

func New(configuration *config.ApplicationConfig) (*Git, error) {
	var sshAuth *ssh.PublicKeys
	if configuration.SSHPrivateKey() != "" {
		var err error
		sshAuth, err = ssh.NewPublicKeys("git",
			[]byte(configuration.SSHPrivateKey()), configuration.SSHPrivateKeyPassword())
		if err != nil {
			return nil, err
		}
	}

	return &Git{
		sshAuth: sshAuth,
	}, nil
}

func (r *Git) CloneCommit(_ context.Context, gitURL string, gitReferenceOrHash string) (*git.Repository, error) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		return nil, err
	}

	if _, err = repo.CreateRemote(&gitConfig.RemoteConfig{
		Name: "origin",
		URLs: []string{gitURL},
	}); err != nil {
		return nil, err
	}

	localBranch := "refs/heads/local"
	refSpec := fmt.Sprintf("%s:%s", gitReferenceOrHash, localBranch)
	if err = repo.Fetch(&git.FetchOptions{
		Auth:     r.sshAuth,
		RefSpecs: []gitConfig.RefSpec{gitConfig.RefSpec(refSpec)},
		Depth:    1,
	}); err != nil {
		return nil, err
	}

	tree, err := repo.Worktree()
	if err != nil {
		return nil, err
	}

	if err = tree.Checkout(&git.CheckoutOptions{
		Branch: plumbing.ReferenceName(localBranch),
	}); err != nil {
		return nil, err
	}

	return repo, nil
}
