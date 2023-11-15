package helmremote

import (
	"context"
	"io"

	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/google/uuid"
	"helm.sh/helm/v3/pkg/downloader"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
)

type HelmRemote struct {
	appConfig config.ApplicationConfig
}

func New() *HelmRemote {
	return &HelmRemote{}
}

func (r *HelmRemote) GetIndex(
	_ context.Context,
	repositoryURL string,
) ([]byte, error) {
	c := repo.Entry{
		URL:  repositoryURL,
		Name: uuid.NewString(),
	}

	chartRepository, err := repo.NewChartRepository(&c, r.appConfig.HelmProviders())
	if err != nil {
		return nil, err
	}

	indexURL, err := repo.ResolveReferenceURL(chartRepository.Config.URL, "index.yaml")
	if err != nil {
		return nil, err
	}

	resp, err := chartRepository.Client.Get(indexURL,
		getter.WithURL(chartRepository.Config.URL),
		getter.WithInsecureSkipVerifyTLS(chartRepository.Config.InsecureSkipTLSverify),
		getter.WithTLSClientConfig(chartRepository.Config.CertFile, chartRepository.Config.KeyFile, chartRepository.Config.CAFile),
		getter.WithBasicAuth(chartRepository.Config.Username, chartRepository.Config.Password),
		getter.WithPassCredentialsAll(chartRepository.Config.PassCredentialsAll),
	)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(resp)
}

func (r *HelmRemote) GetChart(
	_ context.Context,
	chartRef string,
) ([]byte, error) {
	chartDownloader := downloader.ChartDownloader{
		Out:     r,
		Verify:  downloader.VerifyNever,
		Getters: r.appConfig.HelmProviders(),
	}

	chartURL, err := chartDownloader.ResolveChartVersion(chartRef, "")
	if err != nil {
		return nil, err
	}

	urlGetter, err := chartDownloader.Getters.ByScheme(chartURL.Scheme)
	if err != nil {
		return nil, err
	}

	chartBuffer, err := urlGetter.Get(chartURL.String(), chartDownloader.Options...)
	if err != nil {
		return nil, err
	}

	return chartBuffer.Bytes(), nil
}

func (r *HelmRemote) Write(_ []byte) (int, error) {
	return 0, nil
}
