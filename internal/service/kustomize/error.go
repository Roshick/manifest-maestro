package kustomize

import "fmt"

type KustomizationRenderError struct {
	err error
}

func (e *KustomizationRenderError) Error() string {
	return fmt.Sprintf("failed to render Kustomize kustomization: %v", e.err)
}

func NewKustomizationRenderError(err error) *KustomizationRenderError {
	return &KustomizationRenderError{
		err: err,
	}
}
