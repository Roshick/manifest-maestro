package helmremote

import (
	"fmt"
	"net/url"
)

type RepositoryNotFoundError struct {
	repositoryURL string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("helm repository '%s' does not exist", e.repositoryURL)
}

func NewRepositoryNotFoundError(repositoryURL string) *RepositoryNotFoundError {
	return &RepositoryNotFoundError{
		repositoryURL: repositoryURL,
	}
}

func NewRepositoryNotFoundError2(repositoryURL url.URL) *RepositoryNotFoundError {
	return &RepositoryNotFoundError{
		repositoryURL: repositoryURL.String(),
	}
}

type RepositoryChartNotFoundError struct {
	chartURL string
}

func (e *RepositoryChartNotFoundError) Error() string {
	return fmt.Sprintf("helm chart '%s' does not exist", e.chartURL)
}

func NewRepositoryChartNotFoundError(
	repositoryURL string,
	chartName string,
	chartVersion string,
) *RepositoryChartNotFoundError {
	return &RepositoryChartNotFoundError{
		chartURL: fmt.Sprintf("%s/%s:%s", repositoryURL, chartName, chartVersion),
	}
}

func NewRepositoryChartNotFoundError2(chartURL url.URL) *RepositoryChartNotFoundError {
	return &RepositoryChartNotFoundError{
		chartURL: chartURL.String(),
	}
}

type MissingProviderError struct {
	host   string
	scheme string
}

func (e *MissingProviderError) Error() string {
	return fmt.Sprintf("missing provider for '%s' with scheme '%s'", e.host, e.scheme)
}

func NewMissingProviderError(
	chartURL url.URL,
) *MissingProviderError {
	return &MissingProviderError{
		host:   chartURL.Host,
		scheme: chartURL.Scheme,
	}
}
