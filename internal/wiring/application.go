package wiring

import (
	"context"
	"fmt"
	aucache "github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/manifest-maestro/internal/service/cache"
	"github.com/Roshick/manifest-maestro/internal/web/controller/health"
	"github.com/Roshick/manifest-maestro/internal/web/controller/metrics"
	"github.com/Roshick/manifest-maestro/internal/web/controller/swagger"
	"github.com/Roshick/manifest-maestro/internal/web/controller/v1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"log/slog"
	"os"
	"time"

	"github.com/go-git/go-git/v5/plumbing"

	"github.com/Roshick/go-autumn-configloader/pkg/configloader"
	"github.com/Roshick/go-autumn-slog/pkg/logging"
	"github.com/Roshick/go-autumn-vault/pkg/vault"
	"github.com/Roshick/manifest-maestro/internal/repository/clock"
	augit "github.com/Roshick/manifest-maestro/internal/repository/git"
	"github.com/Roshick/manifest-maestro/internal/repository/helmremote"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-git/go-git/v5"

	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/web"
)

type Clock interface {
	Now() time.Time
}

type Git interface {
	FetchReferences(context.Context, string) ([]*plumbing.Reference, error)

	CloneCommit(context.Context, string, string) (*git.Repository, error)
}

type HelmRemote interface {
	GetIndex(context.Context, string) ([]byte, error)

	GetChart(context.Context, string) ([]byte, error)
}

type Application struct {
	// bootstrap
	ConfigLoader *configloader.ConfigLoader
	Logger       *slog.Logger
	Config       *config.Config
	VaultClient  vault.Client
	Vault        *vault.Vault

	// metrics
	PrometheusExporter *prometheus.Exporter
	MeterProvider      *metric.MeterProvider

	// repositories (outgoing connectors)
	Clock      Clock
	Git        Git
	HelmRemote HelmRemote

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
	HealthController  *health.Controller
	SwaggerController *swagger.Controller
	MetricsController *metrics.Controller
	V1Controller      *v1.Controller

	// server
	Server *web.Server
}

func NewApplication() *Application {
	return &Application{}
}

func (a *Application) Create(ctx context.Context, configFilePath string) error {
	if err := a.createConfigLoader(ctx); err != nil {
		return fmt.Errorf("failed to set up configuration loader: %w", err)
	}
	if err := a.createConfig(ctx); err != nil {
		return fmt.Errorf("failed to set up configuration: %w", err)
	}
	if err := a.loadBootstrapConfig(ctx, configFilePath); err != nil {
		return fmt.Errorf("failed to load bootstrap config: %w", err)
	}

	if err := a.createLogging(ctx, configFilePath); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}

	if err := a.createVault(ctx, configFilePath); err != nil {
		return fmt.Errorf("failed to set up vault: %w", err)
	}
	if err := a.loadApplicationConfig(ctx, configFilePath); err != nil {
		return fmt.Errorf("failed to load application config: %w", err)
	}

	if err := a.setupOpenTelemetry(); err != nil {
		return fmt.Errorf("failed to setup open-telemetry: %w", err)
	}

	// repositories (outgoing connectors)
	if err := a.createClock(ctx); err != nil {
		return fmt.Errorf("failed to set up clock: %w", err)
	}
	if err := a.createGit(ctx); err != nil {
		return fmt.Errorf("failed to set up git: %w", err)
	}
	if err := a.createHelmRemote(ctx); err != nil {
		return fmt.Errorf("failed to set up helm-remote: %w", err)
	}

	// services (business logic)
	if err := a.createGitRepositoryCache(ctx); err != nil {
		return fmt.Errorf("failed to set up git-repository cache: %w", err)
	}
	if err := a.createHelmIndexCache(ctx); err != nil {
		return fmt.Errorf("failed to set up helm-index cache: %w", err)
	}
	if err := a.createHelmChartCache(ctx); err != nil {
		return fmt.Errorf("failed to set up helm-chart cache: %w", err)
	}
	if err := a.createHelmChartProvider(ctx); err != nil {
		return fmt.Errorf("failed to set up helm-chart provider: %w", err)
	}
	if err := a.createHelmChartRenderer(ctx); err != nil {
		return fmt.Errorf("failed to set up helm-chart renderer: %w", err)
	}
	if err := a.createKustomizationProvider(ctx); err != nil {
		return fmt.Errorf("failed to set up kustomization provider: %w", err)
	}
	if err := a.createKustomizationRenderer(ctx); err != nil {
		return fmt.Errorf("failed to set up kustomization renderer: %w", err)
	}

	// web stack
	if err := a.createHealthController(ctx); err != nil {
		return fmt.Errorf("failed to set up health controller: %w", err)
	}
	if err := a.createSwaggerController(ctx); err != nil {
		return fmt.Errorf("failed to set up swagger controller: %w", err)
	}
	if err := a.createMetricsController(ctx); err != nil {
		return fmt.Errorf("failed to set up metrics controller: %w", err)
	}
	if err := a.createV1Controller(ctx); err != nil {
		return fmt.Errorf("failed to set up v1 controller: %w", err)
	}
	if err := a.createServer(ctx); err != nil {
		return fmt.Errorf("failed to set up server: %w", err)
	}
	return nil
}

func (a *Application) Teardown(_ context.Context) {
}

func (a *Application) Run() int {
	ctx, cancel := context.WithCancel(context.Background())
	// call cancel before teardown because resources in teardown will most likely depend on context
	defer func() {
		cancel()
		a.Teardown(ctx)
	}()

	if err := a.Create(ctx, "local-config.yaml"); err != nil {
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Printf("failed to create application")
		return config.ExitCodeCreateFailed
	}

	if err := a.Server.Run(ctx); err != nil {
		aulogging.Logger.Ctx(ctx).Error().WithErr(err).Printf("failed to run application")
		return config.ExitCodeRunFailed
	}

	return config.ExitCodeSuccess
}
func (a *Application) createConfigLoader(_ context.Context) error {
	if a.ConfigLoader == nil {
		a.ConfigLoader = configloader.New()
	}
	return nil
}

func (a *Application) createConfig(_ context.Context) error {
	if a.Config == nil {
		a.Config = config.New()
	}
	return nil
}

func (a *Application) loadBootstrapConfig(_ context.Context, configFilePath string) error {
	providers := defaultProviders(configFilePath)

	return a.ConfigLoader.LoadConfig(a.Config.Bootstrap(), providers...)
}

func (a *Application) createLogging(_ context.Context, configFilePath string) error {
	if a.Logger == nil {
		providers := defaultProviders(configFilePath)

		loggingConfig := logging.NewConfig()
		if err := a.ConfigLoader.LoadConfig(loggingConfig, providers...); err != nil {
			return err
		}

		if a.Config.Bootstrap().LogType() == config.LogStyleJSON {
			a.Logger = slog.New(slog.NewJSONHandler(os.Stderr, loggingConfig.HandlerOptions()))
		} else {
			a.Logger = slog.New(slog.NewTextHandler(os.Stderr, loggingConfig.HandlerOptions()))
		}
	}

	slog.SetDefault(a.Logger)
	aulogging.Logger = logging.New()
	return nil
}

func (a *Application) createVault(ctx context.Context, configFilePath string) error {
	if a.Config.Bootstrap().VaultEnabled() && a.Vault == nil {
		providers := defaultProviders(configFilePath)

		if err := a.ConfigLoader.LoadConfig(a.Config.Vault(), providers...); err != nil {
			return err
		}
		if a.VaultClient == nil {
			if vaultClient, err := vault.NewClient(ctx, a.Config.Vault()); err != nil {
				return err
			} else {
				a.VaultClient = vaultClient
			}
		}
		a.Vault = vault.New(a.Config.Vault(), a.VaultClient)
	}
	return nil
}

func (a *Application) loadApplicationConfig(_ context.Context, configFilePath string) error {
	providers := defaultProviders(configFilePath)
	if a.Config.Bootstrap().VaultEnabled() && a.Vault != nil {
		providers = append(providers, a.Vault.ValuesProvider())
	}

	if err := a.ConfigLoader.LoadConfig(a.Config.Application(), providers...); err != nil {
		return err
	}

	slog.SetDefault(slog.Default().With("application", a.Config.Application().ApplicationName()))

	return nil
}

func (a *Application) setupOpenTelemetry() error {
	exporter, err := prometheus.New()
	if err != nil {
		return err
	}
	a.PrometheusExporter = exporter

	a.MeterProvider = metric.NewMeterProvider(
		metric.WithReader(exporter),
	)
	otel.SetMeterProvider(a.MeterProvider)

	return nil
}

func (a *Application) createClock(_ context.Context) error {
	if a.Clock == nil {
		a.Clock = clock.New()
	}
	return nil
}

func (a *Application) createGit(_ context.Context) error {
	if a.Git == nil {
		if iGit, err := augit.New(a.Config.Application()); err != nil {
			return err
		} else {
			a.Git = iGit
		}
	}
	return nil
}

func (a *Application) createHelmRemote(_ context.Context) error {
	if a.HelmRemote == nil {
		a.HelmRemote = helmremote.New()
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
		apiVersions := a.Config.Application().HelmDefaultKubernetesAPIVersions()
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

func (a *Application) createHealthController(_ context.Context) error {
	if a.HealthController == nil {
		a.HealthController = health.NewController()
	}
	return nil
}

func (a *Application) createSwaggerController(_ context.Context) error {
	if a.SwaggerController == nil {
		a.SwaggerController = swagger.NewController()
	}
	return nil
}

func (a *Application) createMetricsController(_ context.Context) error {
	if a.MetricsController == nil {
		a.MetricsController = metrics.NewController()
	}
	return nil
}

func (a *Application) createV1Controller(_ context.Context) error {
	if a.V1Controller == nil {
		a.V1Controller = v1.NewController(a.Clock, a.HelmChartProvider, a.HelmChartRenderer, a.KustomizationProvider, a.KustomizationRenderer)
	}
	return nil
}

func (a *Application) createServer(ctx context.Context) error {
	if a.Server == nil {
		server, err := web.NewServer(
			ctx, a.Config.Application(),
			a.HealthController, a.SwaggerController, a.MetricsController, a.V1Controller,
		)
		if err != nil {
			return err
		}
		a.Server = server
	}
	return nil
}

func (a *Application) createByteSliceCache(ctx context.Context, cacheKey string) (aucache.Cache[[]byte], error) {
	syncMethod := a.Config.Application().SynchronizationMethod()
	switch syncMethod {
	case config.SynchronizationMethodRedis:
		aulogging.Logger.Ctx(ctx).Info().Printf("instantiating redis cache for %s", cacheKey)
		redisURL := a.Config.Application().SynchronizationRedisURL()
		redisPassword := a.Config.Application().SynchronizationRedisPassword()
		return aucache.NewRedisCache[[]byte](redisURL, redisPassword, cacheKey)
	default:
		aulogging.Logger.Ctx(ctx).Info().Printf("instantiating memory cache for %s", cacheKey)
		return aucache.NewMemoryCache[[]byte](), nil
	}
}

func defaultProviders(configPath string) []configloader.Provider {
	return []configloader.Provider{
		configloader.CreateDefaultValuesProvider(),
		configloader.CreateYAMLConfigFileProvider(configPath),
		configloader.CreateEnvironmentVariablesProvider(),
	}
}
