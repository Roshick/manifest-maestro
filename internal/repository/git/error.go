package git

import "fmt"

type RepositoryNotFoundError struct {
	repositoryURL string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("git repository '%s' does not exist", e.repositoryURL)
}

func NewRepositoryNotFoundError(repositoryURL string) *RepositoryNotFoundError {
	return &RepositoryNotFoundError{
		repositoryURL: repositoryURL,
	}
}

type RepositoryReferenceNotFoundError struct {
	repositoryURL string
	gitReference  string
}

func (e *RepositoryReferenceNotFoundError) Error() string {
	return fmt.Sprintf("reference '%s' does not exist in git repository '%s'", e.gitReference, e.repositoryURL)
}

func NewRepositoryReferenceNotFoundError(
	repositoryURL string,
	gitReference string,
) *RepositoryReferenceNotFoundError {
	return &RepositoryReferenceNotFoundError{
		repositoryURL: repositoryURL,
		gitReference:  gitReference,
	}
}

// RepositoryURLInvalidError indicates a repository url that is malformed or does not point to a permitted host.
type RepositoryURLInvalidError struct {
	inner error
}

func (e *RepositoryURLInvalidError) Error() string {
	return e.inner.Error()
}

func (e *RepositoryURLInvalidError) Unwrap() error {
	return e.inner
}

func NewRepositoryURLInvalidError(inner error) *RepositoryURLInvalidError {
	return &RepositoryURLInvalidError{
		inner: inner,
	}
}
