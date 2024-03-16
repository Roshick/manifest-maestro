package helmremotemock

import (
	"context"

	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

type Impl struct {
}

func New() *Impl {
	return &Impl{}
}

func (r *Impl) GetIndex(
	_ context.Context,
	repositoryURL string,
	providers []getter.Provider,
) (*repo.IndexFile, error) {
	panic("ToDo: implement")
}

func (r *Impl) RetrieveChart(
	_ context.Context,
	chartRef string,
	providers []getter.Provider,
) ([]byte, error) {
	panic("ToDo: implement")
}
