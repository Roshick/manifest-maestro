package wiring

import (
	"context"
	"crypto/rsa"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	aucache "github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/internal/client"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/internal/web/controller"
	"github.com/google/go-github/v80/github"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/trace"

	"github.com/Roshick/go-autumn-slog/pkg/logging"
	"github.com/Roshick/go-autumn-vault"
	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/repository/clock"
	augit "github.com/Roshick/manifest-maestro/internal/repository/git"
	"github.com/Roshick/manifest-maestro/internal/repository/helmremote"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/internal/web"
	aulogging "github.com/StephanHCB/go-autumn-logging"
)

type Clock interface {
	Now() time.Time
}

type ClientFactory interface {
	NewHTTPClient(clientName string, opts *client.HTTPClientOptions) (*http.Client, error)

	NewGitHubClient(appID int64, appInstallationID int64, privateKey *rsa.PrivateKey, opts *client.GitHubClientOptions) (*github.Client, error)
}

type Git interface {
	cache.Git
}

type HelmRemote interface {
	cache.HelmIndexRemote
	cache.HelmChartRemote
}

type Application struct {
	// bootstrap
	Clock          Clock
	Logger         *slog.Logger
	ClientFactory  ClientFactory
	ApplicationCfg *config.ApplicationConfig

	// telemetry
	MetricsExporter *prometheus.Exporter
	MeterProvider   *metric.MeterProvider
	TracesExporter  *otlptrace.Exporter
	TracerProvider  *trace.TracerProvider

	// repositories (outgoing connectors)
	GitHubClient *github.Client
	Git          Git
	HelmRemote   HelmRemote

	// services (business logic)
	GitRepositoryCache    *cache.GitRepositoryCache
	HelmIndexCache        *cache.HelmIndexCache
	HelmChartCache        *cache.HelmChartCache
	HelmChartProvider     *helm.ChartProvider
	HelmChartRenderer     *helm.ChartRenderer
	KustomizationProvider *kustomize.KustomizationProvider
	KustomizationRenderer *kustomize.KustomizationRenderer

	// web stack
	// controllers (incoming connectors)
	HealthCtl   *controller.HealthController
	SwaggerCtl  *controller.SwaggerController
	MetricsCtl  *controller.MetricsController
	ProfilerCtl *controller.ProfilerController
	V1Ctl       *controller.V1Controller

	// server
	Server *web.Server
}

func NewApplication() *Application {
	return &Application{}
}

func (a *Application) Create(ctx context.Context) error {
	// bootstrap
	a.createClock(ctx)
	a.createClientFactory(ctx)
	if err := a.createLogging(ctx); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}
	if err := a.fetchVaultSecrets(ctx); err != nil {
		return fmt.Errorf("failed to set up vault: %w", err)
	}
	if err := a.loadApplicationConfig(ctx); err != nil {
		return fmt.Errorf("failed to load application config: %w", err)
	}
	if err := a.setupTelemetry(ctx); err != nil {
		return fmt.Errorf("failed to set up telemetry: %w", err)
	}

	// repositories (outgoing connectors)
	if err := a.createGitHub(ctx); err != nil {
		return fmt.Errorf("failed to set up github: %w", err)
	}
	if err := a.createGit(ctx); err != nil {
		return fmt.Errorf("failed to set up git: %w", err)
	}
	if err := a.createHelmRemote(ctx); err != nil {
		return fmt.Errorf("failed to set up helm-remote: %w", err)
	}

	// services (business logic)
	if err := a.createGitRepositoryCache(ctx); err != nil {
		return fmt.Errorf("failed to set up git repository cache: %w", err)
	}
	if err := a.createHelmIndexCache(ctx); err != nil {
		return fmt.Errorf("failed to set up helm index cache: %w", err)
	}
	if err := a.createHelmChartCache(ctx); err != nil {
		return fmt.Errorf("failed to set up helm chart cache: %w", err)
	}
	if err := a.createHelmChartProvider(ctx); err != nil {
		return fmt.Errorf("failed to set up helm chart provider: %w", err)
	}
	if err := a.createHelmChartRenderer(ctx); err != nil {
		return fmt.Errorf("failed to set up helm chart renderer: %w", err)
	}
	if err := a.createKustomizationProvider(ctx); err != nil {
		return fmt.Errorf("failed to set up kustomization provider: %w", err)
	}
	if err := a.createKustomizationRenderer(ctx); err != nil {
		return fmt.Errorf("failed to set up kustomization renderer: %w", err)
	}

	// web stack
	a.createHealthController(ctx)
	a.createSwaggerController(ctx)
	a.createMetricsController(ctx)
	a.createProfilerController(ctx)
	a.createV1Controller(ctx)

	if err := a.createServer(ctx); err != nil {
		return fmt.Errorf("failed to set up server: %w", err)
	}
	return nil
}

func (a *Application) Teardown(ctx context.Context, cancel context.CancelFunc) {
	defer cancel()

	if a.MeterProvider != nil {
		if err := a.MeterProvider.Shutdown(ctx); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to shut down meter provider")
		}
	}

	if a.TracerProvider != nil {
		if err := a.TracerProvider.Shutdown(ctx); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to shut down tracer provider")
		}
	}
	if a.TracesExporter != nil {
		if err := a.TracesExporter.Shutdown(ctx); err != nil {
			aulogging.Logger.Ctx(ctx).Warn().WithErr(err).Printf("failed to shut down traces exporter")
		}
	}
}

func (a *Application) Run() int {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		a.Teardown(ctx, cancel)
	}()

	if err := a.Create(ctx); err != nil {
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Printf("failed to create application")
		return config.ExitCodeCreateFailed
	}

	if err := a.Server.Run(ctx); err != nil {
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Printf("failed to run application")
		return config.ExitCodeRunFailed
	}

	return config.ExitCodeSuccess
}

func (a *Application) createClock(_ context.Context) {
	a.Clock = clock.New()
}

func (a *Application) createLogging(_ context.Context) error {
	if a.Logger == nil {
		loggingCfg := config.NewLoggingConfig()
		if err := loggingCfg.ObtainValuesFromEnv(); err != nil {
			return fmt.Errorf("failed to obtain logging config values from environment: %w", err)
		}

		if loggingCfg.LogStyle == config.LogStyleJSON {
			a.Logger = slog.New(slog.NewJSONHandler(os.Stderr, loggingCfg.HandlerOptions()))
		} else {
			a.Logger = slog.New(slog.NewTextHandler(os.Stderr, loggingCfg.HandlerOptions()))
		}
	}

	slog.SetDefault(a.Logger)
	aulogging.Logger = logging.New()
	return nil
}

func (a *Application) fetchVaultSecrets(ctx context.Context) error {
	vaultCfg := vault.NewConfig()
	if err := vaultCfg.ObtainValuesFromEnv(); err != nil {
		return fmt.Errorf("failed to obtain vault config values from environment: %w", err)
	}
	if !vaultCfg.Disabled {
		vaultClient, err := a.ClientFactory.NewHTTPClient("vault", nil)
		if err != nil {
			return fmt.Errorf("failed to create vault http client: %w", err)
		}
		vaultInstance, err := vault.New(vaultCfg, vaultClient)
		if err != nil {
			return fmt.Errorf("failed to instantiate vault: %w", err)
		}
		return vaultInstance.FetchSecretsToEnv(ctx)
	}
	return nil
}

func (a *Application) loadApplicationConfig(_ context.Context) error {
	a.ApplicationCfg = config.NewApplicationConfig()
	if err := a.ApplicationCfg.ObtainValuesFromEnv(); err != nil {
		return fmt.Errorf("failed to obtain application config values from environment: %w", err)
	}

	slog.SetDefault(slog.Default().With("application", a.ApplicationCfg.ApplicationName))

	return nil
}

func (a *Application) setupTelemetry(ctx context.Context) error {
	if a.MetricsExporter == nil {
		metricsExporter, err := prometheus.New()
		if err != nil {
			return err
		}
		a.MetricsExporter = metricsExporter
	}
	if a.MeterProvider == nil {
		a.MeterProvider = metric.NewMeterProvider(
			metric.WithReader(a.MetricsExporter),
		)
		otel.SetMeterProvider(a.MeterProvider)
	}

	if a.TracesExporter == nil {
		tracesExporter, err := otlptracegrpc.New(ctx)
		if err != nil {
			return err
		}
		a.TracesExporter = tracesExporter
	}
	if a.TracerProvider == nil {
		a.TracerProvider = trace.NewTracerProvider(
			trace.WithBatcher(a.TracesExporter),
		)
		otel.SetTracerProvider(a.TracerProvider)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

func (a *Application) createClientFactory(_ context.Context) {
	if a.ClientFactory == nil {
		a.ClientFactory = client.NewFactory()
	}
}

func (a *Application) createGitHub(_ context.Context) error {
	var err error

	if a.GitHubClient == nil {
		a.GitHubClient, err = a.ClientFactory.NewGitHubClient(
			a.ApplicationCfg.GitHubAppID, a.ApplicationCfg.GitHubAppInstallationID, &a.ApplicationCfg.GitHubAppPrivateKey, nil,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (a *Application) createGit(_ context.Context) error {
	if a.Git == nil {
		authProvider := augit.NewGitHubAppAuthProvider(a.GitHubClient, a.ApplicationCfg.GitHubAppInstallationID)
		if iGit, err := augit.New(authProvider.GetAuth); err != nil {
			return err
		} else {
			a.Git = iGit
		}
	}
	return nil
}

func (a *Application) createHelmRemote(_ context.Context) error {
	if a.HelmRemote == nil {
		a.HelmRemote = helmremote.New(a.ApplicationCfg.HelmHostProviders)
	}
	return nil
}

func (a *Application) createGitRepositoryCache(ctx context.Context) error {
	if a.GitRepositoryCache == nil {
		byteSliceCache, err := a.createByteSliceCache(ctx, "git-repository")
		if err != nil {
			return err
		}
		a.GitRepositoryCache = cache.NewGitRepositoryCache(a.Git, byteSliceCache)
	}
	return nil
}

func (a *Application) createHelmIndexCache(ctx context.Context) error {
	if a.HelmIndexCache == nil {
		byteSliceCache, err := a.createByteSliceCache(ctx, "helm-index")
		if err != nil {
			return err
		}
		a.HelmIndexCache = cache.NewHelmIndexCache(a.HelmRemote, byteSliceCache)
	}
	return nil
}

func (a *Application) createHelmChartCache(ctx context.Context) error {
	if a.HelmChartCache == nil {
		byteSliceCache, err := a.createByteSliceCache(ctx, "helm-chart")
		if err != nil {
			return err
		}
		a.HelmChartCache = cache.NewHelmChartCache(a.HelmRemote, a.HelmIndexCache, byteSliceCache)
	}
	return nil
}

func (a *Application) createHelmChartProvider(_ context.Context) error {
	if a.HelmChartProvider == nil {
		a.HelmChartProvider = helm.NewChartProvider(a.HelmChartCache, a.GitRepositoryCache)
	}
	return nil
}

func (a *Application) createHelmChartRenderer(_ context.Context) error {
	if a.HelmChartRenderer == nil {
		apiVersions := a.ApplicationCfg.HelmDefaultKubernetesAPIVersions
		a.HelmChartRenderer = helm.NewChartRenderer(apiVersions)
	}
	return nil
}

func (a *Application) createKustomizationProvider(_ context.Context) error {
	if a.KustomizationProvider == nil {
		a.KustomizationProvider = kustomize.NewKustomizationProvider(a.GitRepositoryCache)
	}
	return nil
}

func (a *Application) createKustomizationRenderer(_ context.Context) error {
	if a.KustomizationRenderer == nil {
		a.KustomizationRenderer = kustomize.NewKustomizationRenderer()
	}
	return nil
}

func (a *Application) createHealthController(_ context.Context) {
	a.HealthCtl = controller.NewHealthController()
}

func (a *Application) createSwaggerController(_ context.Context) {
	a.SwaggerCtl = controller.NewSwaggerController()
}

func (a *Application) createMetricsController(_ context.Context) {
	a.MetricsCtl = controller.NewMetricsController(a.GitHubClient, a.ApplicationCfg.GitHubAppID, a.ApplicationCfg.GitHubAppInstallationID)
}

func (a *Application) createProfilerController(_ context.Context) {
	a.ProfilerCtl = controller.NewProfilerController()
}

func (a *Application) createV1Controller(_ context.Context) {
	a.V1Ctl = controller.NewV1Controller(a.Clock, a.HelmChartProvider, a.HelmChartRenderer, a.KustomizationProvider, a.KustomizationRenderer)
}

func (a *Application) createServer(ctx context.Context) error {
	if a.Server == nil {
		server, err := web.NewServer(ctx,
			a.ApplicationCfg.ServerAddress, a.ApplicationCfg.ServerPrimaryPort,
			a.ApplicationCfg.ApplicationName,
			a.HealthCtl, a.SwaggerCtl, a.MetricsCtl, a.ProfilerCtl, a.V1Ctl,
		)
		if err != nil {
			return err
		}
		a.Server = server
	}
	return nil
}

func (a *Application) createByteSliceCache(_ context.Context, cacheKey string) (aucache.Cache[[]byte], error) {
	synchMethod := a.ApplicationCfg.SynchronizationMethod
	switch synchMethod {
	case config.SynchronizationMethodRedis:
		redisURL := a.ApplicationCfg.SynchronizationRedisURL
		redisPassword := a.ApplicationCfg.SynchronizationRedisPassword
		return aucache.NewRedisCache[[]byte](redisURL, redisPassword, cacheKey)
	default:
		return aucache.NewMemoryCache[[]byte](), nil
	}
}
