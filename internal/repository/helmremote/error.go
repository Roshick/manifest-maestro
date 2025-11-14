package helmremote

import "fmt"

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

type RepositoryChartNotFoundError struct {
	repositoryURL string
	chartName     string
	chartVersion  string
}

func (e *RepositoryChartNotFoundError) Error() string {
	return fmt.Sprintf("chart '%s' at version '%s' does not exist in helm repository '%s'", e.chartName, e.chartVersion, e.repositoryURL)
}

func NewRepositoryChartNotFoundError(
	repositoryURL string,
	chartName string,
	chartVersion string,
) *RepositoryChartNotFoundError {
	return &RepositoryChartNotFoundError{
		repositoryURL: repositoryURL,
		chartName:     chartName,
		chartVersion:  chartVersion,
	}
}
