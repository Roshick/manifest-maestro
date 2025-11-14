package helm

import "fmt"

type ValueFileMissingError struct {
	path string
}

func (e *ValueFileMissingError) Error() string {
	return fmt.Sprintf("repository is missing value file at '%s'", e.path)
}

func NewValueFileMissingError(path string) *ValueFileMissingError {
	return &ValueFileMissingError{
		path: path,
	}
}

type RepositoryReferenceInvalidError struct{}

func (e *RepositoryReferenceInvalidError) Error() string {
	return "repository reference is neither a valid Helm chart nor a Git repository reference"
}

func NewRepositoryReferenceInvalidError() *RepositoryReferenceInvalidError {
	return &RepositoryReferenceInvalidError{}
}

type ChartReferenceInvalidError struct{}

func (e *ChartReferenceInvalidError) Error() string {
	return "Helm chart reference is neither a valid Helm chart repository nor a Git repository path chart reference"
}

func NewChartReferenceInvalidError() *ChartReferenceInvalidError {
	return &ChartReferenceInvalidError{}
}

type InvalidRenderValuesError struct {
	err error
}

func (e *InvalidRenderValuesError) Error() string {
	return fmt.Sprintf("render values are invalid: %v", e.err)
}

func NewInvalidRenderValuesError(err error) *InvalidRenderValuesError {
	return &InvalidRenderValuesError{
		err: err,
	}
}

type RenderError struct {
	err error
}

func (e *RenderError) Error() string {
	return fmt.Sprintf("failed to render chart: %v", e.err)
}

func NewRenderError(err error) *RenderError {
	return &RenderError{
		err: err,
	}
}
