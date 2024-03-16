package wiring

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/Roshick/manifest-maestro/internal/service/gitmanager"
	healthcontroller "github.com/Roshick/manifest-maestro/internal/web/controller/health"
	metricscontroller "github.com/Roshick/manifest-maestro/internal/web/controller/metrics"
	swaggercontroller "github.com/Roshick/manifest-maestro/internal/web/controller/swagger"
	v1controller "github.com/Roshick/manifest-maestro/internal/web/controller/v1"
	"github.com/go-git/go-git/v5/plumbing"

	"github.com/Roshick/go-autumn-configloader/pkg/configloader"
	"github.com/Roshick/go-autumn-slog/pkg/logging"
	"github.com/Roshick/go-autumn-synchronisation/pkg/cache"
	"github.com/Roshick/go-autumn-vault/pkg/vault"
	"github.com/Roshick/manifest-maestro/internal/repository/clock"
	augit "github.com/Roshick/manifest-maestro/internal/repository/git"
	"github.com/Roshick/manifest-maestro/internal/repository/helmremote"
	"github.com/Roshick/manifest-maestro/internal/service/helm"
	"github.com/Roshick/manifest-maestro/internal/service/kustomize"
	"github.com/Roshick/manifest-maestro/internal/service/manifestrenderer"
	aulogging "github.com/StephanHCB/go-autumn-logging"
	"github.com/go-git/go-git/v5"

	"github.com/Roshick/manifest-maestro/internal/config"
	"github.com/Roshick/manifest-maestro/internal/web"
)

var (
	LocalConfigFilename = "local-config.yaml"
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

	// repositories (outgoing connectors)
	Clock      Clock
	Git        Git
	HelmRemote HelmRemote

	// services (business logic)
	GitManager       *gitmanager.GitManager
	Helm             *helm.Helm
	Kustomize        *kustomize.Kustomize
	ManifestRenderer *manifestrenderer.ManifestRenderer

	// web stack
	// controllers (incoming connectors)
	HealthController  *healthcontroller.Controller
	SwaggerController *swaggercontroller.Controller
	MetricsController *metricscontroller.Controller
	V1Controller      *v1controller.Controller

	// server
	Server *web.Server
}

func NewApplication() *Application {
	return &Application{}
}

func (a *Application) Create(ctx context.Context) error {
	if err := a.createConfigLoader(ctx); err != nil {
		return fmt.Errorf("failed to set up configuration loader: %w", err)
	}
	if err := a.createConfig(ctx); err != nil {
		return fmt.Errorf("failed to set up configuration: %w", err)
	}
	if err := a.loadBootstrapConfig(ctx); err != nil {
		return fmt.Errorf("failed to load bootstrap config: %w", err)
	}

	if err := a.createLogging(ctx); err != nil {
		return fmt.Errorf("failed to set up logging: %w", err)
	}

	if err := a.createVault(ctx); err != nil {
		return fmt.Errorf("failed to set up vault: %w", err)
	}
	if err := a.loadApplicationConfig(ctx); err != nil {
		return fmt.Errorf("failed to load application config: %w", err)
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
	if err := a.createGitManager(ctx); err != nil {
		return fmt.Errorf("failed to set up git manager: %w", err)
	}
	if err := a.createHelm(ctx); err != nil {
		return fmt.Errorf("failed to set up helm: %w", err)
	}
	if err := a.createKustomize(ctx); err != nil {
		return fmt.Errorf("failed to set up kustomize: %w", err)
	}
	if err := a.createManifestRenderer(ctx); err != nil {
		return fmt.Errorf("failed to set up manifest-renderer: %w", err)
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

func (a *Application) Teardown(_ context.Context, cancel context.CancelFunc) {
	cancel()
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

func (a *Application) loadBootstrapConfig(_ context.Context) error {
	providers := defaultProviders(LocalConfigFilename)

	return a.ConfigLoader.LoadConfig(a.Config.Bootstrap(), providers...)
}

func (a *Application) createLogging(_ context.Context) error {
	if a.Logger == nil {
		providers := defaultProviders(LocalConfigFilename)

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

func (a *Application) createVault(ctx context.Context) error {
	if a.Config.Bootstrap().VaultEnabled() && a.Vault == nil {
		providers := defaultProviders(LocalConfigFilename)

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

func (a *Application) loadApplicationConfig(_ context.Context) error {
	providers := defaultProviders(LocalConfigFilename)
	if a.Config.Bootstrap().VaultEnabled() && a.Vault != nil {
		providers = append(providers, a.Vault.ValuesProvider())
	}

	if err := a.ConfigLoader.LoadConfig(a.Config.Application(), providers...); err != nil {
		return err
	}

	slog.SetDefault(slog.Default().With("application", a.Config.Application().ApplicationName()))

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

func (a *Application) createGitManager(ctx context.Context) error {
	if a.GitManager == nil {
		gitRepositoryCache, err := a.createByteSliceCache(ctx, "git-repository")
		if err != nil {
			return err
		}
		a.GitManager = gitmanager.New(a.Git, gitRepositoryCache)
	}
	return nil
}

func (a *Application) createHelm(ctx context.Context) error {
	if a.Helm == nil {
		indexCache, err := a.createByteSliceCache(ctx, "helm-repository-index")
		if err != nil {
			return err
		}
		chartCache, err := a.createByteSliceCache(ctx, "helm-chart")
		if err != nil {
			return err
		}
		a.Helm = helm.New(a.Config.Application(), a.HelmRemote, a.GitManager, indexCache, chartCache)
	}
	return nil
}

func (a *Application) createKustomize(_ context.Context) error {
	if a.Kustomize == nil {
		a.Kustomize = kustomize.New()
	}
	return nil
}

func (a *Application) createManifestRenderer(_ context.Context) error {
	if a.ManifestRenderer == nil {
		a.ManifestRenderer = manifestrenderer.New(a.Config.Application(), a.GitManager, a.Helm, a.Kustomize)
	}
	return nil
}

func (a *Application) createHealthController(_ context.Context) error {
	if a.HealthController == nil {
		a.HealthController = healthcontroller.New()
	}
	return nil
}

func (a *Application) createSwaggerController(_ context.Context) error {
	if a.SwaggerController == nil {
		a.SwaggerController = swaggercontroller.New()
	}
	return nil
}

func (a *Application) createMetricsController(_ context.Context) error {
	if a.MetricsController == nil {
		a.MetricsController = metricscontroller.New()
	}
	return nil
}

func (a *Application) createV1Controller(_ context.Context) error {
	if a.V1Controller == nil {
		a.V1Controller = v1controller.New(a.Clock, a.Helm, a.ManifestRenderer)
	}
	return nil
}

func (a *Application) createServer(ctx context.Context) error {
	if a.Server == nil {
		server, err := web.NewServer(ctx, a.Config.Application(), a.HealthController, a.SwaggerController, a.MetricsController, a.V1Controller)
		if err != nil {
			return err
		}
		a.Server = server
	}
	return nil
}

func (a *Application) createByteSliceCache(_ context.Context, cacheKey string) (cache.Cache[[]byte], error) {
	switch a.Config.Application().SynchronisationMethod() {
	case config.SynchronisationMethodRedis:
		return cache.NewRueidisCache[[]byte](
			a.Config.Application().SynchronisationRedisURL(),
			a.Config.Application().SynchronisationRedisPassword(),
			cacheKey,
		)
	default:
		return cache.NewMemoryCache[[]byte](), nil
	}
}

func defaultProviders(configPath string) []configloader.Provider {
	return []configloader.Provider{
		configloader.CreateDefaultValuesProvider(),
		configloader.CreateYAMLConfigFileProvider(configPath),
		configloader.CreateEnvironmentVariablesProvider(),
	}
}
