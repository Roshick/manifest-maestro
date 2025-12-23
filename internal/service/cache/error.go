package cache

import (
	"fmt"
)

type InvalidHelmRepositoryURLError struct {
	repositoryURL string
}

func (e *InvalidHelmRepositoryURLError) Error() string {
	return fmt.Sprintf("Helm repository URL '%s' is invalid", e.repositoryURL)
}

func NewInvalidHelmRepositoryURLError(repositoryURL string) *InvalidHelmRepositoryURLError {
	return &InvalidHelmRepositoryURLError{
		repositoryURL: repositoryURL,
	}
}
