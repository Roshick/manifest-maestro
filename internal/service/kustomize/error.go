package kustomize

import "fmt"

type KustomizationReferenceInvalidError struct{}

func (e *KustomizationReferenceInvalidError) Error() string {
	return "Kustomization reference is not a valid Git repository path reference"
}

func NewKustomizationReferenceInvalidError() *KustomizationReferenceInvalidError {
	return &KustomizationReferenceInvalidError{}
}

type InvalidKustomizationParameterError struct {
	err error
}

func (e *InvalidKustomizationParameterError) Error() string {
	return fmt.Sprintf("kustomization parameters are invalid: %v", e.err)
}

func NewInvalidKustomizationParameterError(err error) *InvalidKustomizationParameterError {
	return &InvalidKustomizationParameterError{
		err: err,
	}
}
