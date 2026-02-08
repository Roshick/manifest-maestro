package helmremotemock

import (
	"context"

	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/repo/v1"
)

type Impl struct {
}

func New() *Impl {
	return &Impl{}
}

func (r *Impl) GetIndex(_ context.Context, _ string, _ []getter.Provider) (*repo.IndexFile, error) {
	panic("ToDo: implement")
}

func (r *Impl) RetrieveChart(_ context.Context, _ string, _ []getter.Provider) ([]byte, error) {
	panic("ToDo: implement")
}
