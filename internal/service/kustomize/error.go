package kustomize

type KustomizationReferenceInvalidError struct{}

func (e *KustomizationReferenceInvalidError) Error() string {
	return "Kustomization reference is not a valid Git repository path reference"
}

func NewKustomizationReferenceInvalidError() *KustomizationReferenceInvalidError {
	return &KustomizationReferenceInvalidError{}
}
