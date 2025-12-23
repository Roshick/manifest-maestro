package helm

import "fmt"

type ChartReferenceInvalidError struct{}

func (e *ChartReferenceInvalidError) Error() string {
	return "Helm chart reference is neither a valid Helm chart repository nor a Git repository path chart reference"
}

func NewChartReferenceInvalidError() *ChartReferenceInvalidError {
	return &ChartReferenceInvalidError{}
}

type ChartBuildError struct {
	err error
}

func (e *ChartBuildError) Error() string {
	return fmt.Sprintf("failed to build Helm chart: %v", e.err)
}

func NewChartBuildError(err error) *ChartBuildError {
	return &ChartBuildError{
		err: err,
	}
}

type ChartRenderError struct {
	err error
}

func (e *ChartRenderError) Error() string {
	return fmt.Sprintf("failed to render Helm chart: %v", e.err)
}

func NewChartRenderError(err error) *ChartRenderError {
	return &ChartRenderError{
		err: err,
	}
}
