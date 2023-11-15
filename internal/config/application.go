package config

import (
	"fmt"
	"strconv"

	"helm.sh/helm/v3/pkg/getter"

	"helm.sh/helm/v3/pkg/chartutil"

	"github.com/Roshick/manifest-maestro/pkg/utils/stringutils"
	auconfigapi "github.com/StephanHCB/go-autumn-config-api"
)

const (
	keyApplicationName = "APPLICATION_NAME"

	keyServerAddress     = "SERVER_ADDRESS"
	keyServerPrimaryPort = "SERVER_PRIMARY_PORT"
	keyServerMetricsPort = "SERVER_METRICS_PORT"

	keySSHPrivateKey         = "SSH_PRIVATE_KEY"
	keySSHPrivateKeyPassword = "SSH_PRIVATE_KEY_PASSWORD"

	keyHelmDefaultReleaseName           = "HELM_DEFAULT_RELEASE_NAME"
	keyHelmDefaultKubernetesAPIVersions = "HELM_DEFAULT_KUBERNETES_API_VERSIONS"
	keyHelmDefaultKubernetesNamespace   = "HELM_DEFAULT_KUBERNETES_NAMESPACE"

	keySynchronisationMethod        = "SYNCHRONISATION_METHOD"
	keySynchronisationRedisURL      = "SYNCHRONISATION_REDIS_URL"
	keySynchronisationRedisPassword = "SYNCHRONISATION_REDIS_PASSWORD"
)

type SynchronisationMethod int64

const (
	SynchronisationMethodUnknown SynchronisationMethod = iota
	SynchronisationMethodMemory
	SynchronisationMethodRedis
)

type ApplicationConfig struct {
	vApplicationName string

	vServerAddress     string
	vServerPrimaryPort uint
	vServerMetricsPort uint

	vSSHPrivateKey         string
	vSSHPrivateKeyPassword string

	vHelmDefaultReleaseName           string
	vHelmDefaultKubernetesVersion     *chartutil.KubeVersion
	vHelmDefaultKubernetesAPIVersions []string
	vHelmDefaultKubernetesNamespace   string

	vSynchronisationMethod        SynchronisationMethod
	vSynchronisationRedisURL      string
	vSynchronisationRedisPassword string
}

func NewApplicationConfig() *ApplicationConfig {
	return &ApplicationConfig{}
}

func (c *ApplicationConfig) ApplicationName() string {
	return c.vApplicationName
}

func (c *ApplicationConfig) ServerAddress() string {
	return c.vServerAddress
}

func (c *ApplicationConfig) ServerPrimaryPort() uint {
	return c.vServerPrimaryPort
}

func (c *ApplicationConfig) ServerMetricsPort() uint {
	return c.vServerMetricsPort
}

func (c *ApplicationConfig) SSHPrivateKey() string {
	return c.vSSHPrivateKey
}

func (c *ApplicationConfig) SSHPrivateKeyPassword() string {
	return c.vSSHPrivateKeyPassword
}

func (c *ApplicationConfig) HelmProviders() []getter.Provider {
	return []getter.Provider{
		{
			Schemes: []string{"http", "https"},
			New:     getter.NewHTTPGetter,
		},
	}
}

func (c *ApplicationConfig) HelmDefaultReleaseName() string {
	return c.vHelmDefaultReleaseName
}

func (c *ApplicationConfig) HelmDefaultKubernetesAPIVersions() []string {
	return c.vHelmDefaultKubernetesAPIVersions
}

func (c *ApplicationConfig) HelmDefaultKubernetesNamespace() string {
	return c.vHelmDefaultKubernetesNamespace
}

func (c *ApplicationConfig) SynchronisationMethod() SynchronisationMethod {
	return c.vSynchronisationMethod
}

func (c *ApplicationConfig) SynchronisationRedisURL() string {
	return c.vSynchronisationRedisURL
}

func (c *ApplicationConfig) SynchronisationRedisPassword() string {
	return c.vSynchronisationRedisPassword
}

func (c *ApplicationConfig) ConfigItems() []auconfigapi.ConfigItem {
	return []auconfigapi.ConfigItem{
		{
			Key:         keyApplicationName,
			EnvName:     keyApplicationName,
			Description: "Name of the application.",
			Default:     "manifest-maestro",
		},
		{
			Key:         keyServerAddress,
			EnvName:     keyServerAddress,
			Description: "Address all servers listen on.",
			Default:     "",
		},
		{
			Key:         keyServerPrimaryPort,
			EnvName:     keyServerPrimaryPort,
			Description: "Port used by the primary (i.e. application) server.",
			Default:     "8080",
		},
		{
			Key:         keyServerMetricsPort,
			EnvName:     keyServerMetricsPort,
			Description: "Port used by the metrics server.",
			Default:     "9090",
		},
		{
			Key:         keySSHPrivateKey,
			EnvName:     keySSHPrivateKey,
			Description: "SSH private key used to access git repositories. Requires read permissions.",
			Default:     "",
		},
		{
			Key:         keySSHPrivateKeyPassword,
			EnvName:     keySSHPrivateKeyPassword,
			Description: "SSH private key password.",
			Default:     "",
		},
		{
			Key:         keyHelmDefaultReleaseName,
			EnvName:     keyHelmDefaultReleaseName,
			Description: "Default release name used by helm.",
			Default:     "RELEASE-NAME",
		},
		{
			Key:         keyHelmDefaultKubernetesAPIVersions,
			EnvName:     keyHelmDefaultKubernetesAPIVersions,
			Description: "Comma separated list of default API versions used by helm.",
			Default:     "",
		},
		{
			Key:         keyHelmDefaultKubernetesNamespace,
			EnvName:     keyHelmDefaultKubernetesNamespace,
			Description: "Default Kubernetes namespace used by helm.",
			Default:     "default",
		},
		{
			Key:         keySynchronisationMethod,
			EnvName:     keySynchronisationMethod,
			Default:     "MEMORY",
			Description: "Type of synchronisation used between multiple instances of the service.",
		},
		{
			Key:         keySynchronisationRedisURL,
			EnvName:     keySynchronisationRedisURL,
			Default:     "",
			Description: "URL of the Redis instance used in case of synchronisation method 'REDIS'.",
		},
		{
			Key:         keySynchronisationRedisPassword,
			EnvName:     keySynchronisationRedisPassword,
			Default:     "",
			Description: "Password to the Redis instance used in case of synchronisation method 'REDIS'.",
		},
	}
}

func (c *ApplicationConfig) ObtainValues(getter func(string) string) error {
	c.vApplicationName = getter(keyApplicationName)

	c.vServerAddress = getter(keyServerAddress)
	vServerPrimaryPort, err := parseUint(getter(keyServerPrimaryPort))
	if err != nil {
		return err
	}
	c.vServerPrimaryPort = vServerPrimaryPort
	vServerMetricsPort, err := parseUint(getter(keyServerMetricsPort))
	if err != nil {
		return err
	}
	c.vServerMetricsPort = vServerMetricsPort

	c.vSSHPrivateKey = getter(keySSHPrivateKey)
	c.vSSHPrivateKeyPassword = getter(keySSHPrivateKeyPassword)

	c.vHelmDefaultReleaseName = getter(keyHelmDefaultReleaseName)
	c.vHelmDefaultKubernetesAPIVersions = stringutils.SplitUniqueNonEmpty(getter(keyHelmDefaultKubernetesAPIVersions), ",")
	c.vHelmDefaultKubernetesNamespace = getter(keyHelmDefaultKubernetesNamespace)

	vSynchronisationMethod, err := parseSynchronisationMethod(getter(keySynchronisationMethod))
	if err != nil {
		return err
	}
	c.vSynchronisationMethod = vSynchronisationMethod
	c.vSynchronisationRedisURL = getter(keySynchronisationRedisURL)
	c.vSynchronisationRedisPassword = getter(keySynchronisationRedisPassword)

	return nil
}

func parseBoolean(value string) (bool, error) {
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("value '%s' is not a valid boolean", value)
	}
	return boolValue, nil
}

func parseUint(value string) (uint, error) {
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("value '%s' is not a valid integer: %w", value, err)
	}
	if intValue < 0 {
		return 0, fmt.Errorf("value '%s' is not a valid unsigned integer", value)
	}
	return uint(intValue), nil
}

func parseSynchronisationMethod(value string) (SynchronisationMethod, error) {
	switch value {
	case "REDIS":
		return SynchronisationMethodRedis, nil
	case "MEMORY":
		return SynchronisationMethodMemory, nil
	default:
		return SynchronisationMethodUnknown, fmt.Errorf("invalid synchronisation method: '%s'", value)
	}
}
