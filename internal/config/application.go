package config

import (
	"fmt"
	"strconv"

	"helm.sh/helm/v3/pkg/getter"

	"github.com/Roshick/manifest-maestro/pkg/utils/stringutils"
	auconfigapi "github.com/StephanHCB/go-autumn-config-api"
)

const (
	keyApplicationName = "APPLICATION_NAME"

	keyServerAddress     = "SERVER_ADDRESS"
	keyServerPrimaryPort = "SERVER_PRIMARY_PORT"

	keySSHPrivateKey         = "SSH_PRIVATE_KEY"
	keySSHPrivateKeyPassword = "SSH_PRIVATE_KEY_PASSWORD"

	keyHelmDefaultReleaseName           = "HELM_DEFAULT_RELEASE_NAME"
	keyHelmDefaultKubernetesAPIVersions = "HELM_DEFAULT_KUBERNETES_API_VERSIONS"
	keyHelmDefaultKubernetesNamespace   = "HELM_DEFAULT_KUBERNETES_NAMESPACE"

	keySynchronizationMethod        = "SYNCHRONIZATION_METHOD"
	keySynchronizationRedisURL      = "SYNCHRONIZATION_REDIS_URL"
	keySynchronizationRedisPassword = "SYNCHRONIZATION_REDIS_PASSWORD"
)

type SynchronizationMethod int64

const (
	SynchronizationMethodUnknown SynchronizationMethod = iota
	SynchronizationMethodMemory
	SynchronizationMethodRedis
)

type ApplicationConfig struct {
	vApplicationName string

	vServerAddress     string
	vServerPrimaryPort uint
	vServerMetricsPort uint

	vSSHPrivateKey         string
	vSSHPrivateKeyPassword string

	vHelmDefaultReleaseName           string
	vHelmDefaultKubernetesAPIVersions []string
	vHelmDefaultKubernetesNamespace   string

	vSynchronizationMethod        SynchronizationMethod
	vSynchronizationRedisURL      string
	vSynchronizationRedisPassword string
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

func (c *ApplicationConfig) SynchronizationMethod() SynchronizationMethod {
	return c.vSynchronizationMethod
}

func (c *ApplicationConfig) SynchronizationRedisURL() string {
	return c.vSynchronizationRedisURL
}

func (c *ApplicationConfig) SynchronizationRedisPassword() string {
	return c.vSynchronizationRedisPassword
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
			Key:         keySynchronizationMethod,
			EnvName:     keySynchronizationMethod,
			Default:     "MEMORY",
			Description: "Type of synchronization used between multiple instances of the service.",
		},
		{
			Key:         keySynchronizationRedisURL,
			EnvName:     keySynchronizationRedisURL,
			Default:     "",
			Description: "URL of the Redis instance used in case of synchronization method 'REDIS'.",
		},
		{
			Key:         keySynchronizationRedisPassword,
			EnvName:     keySynchronizationRedisPassword,
			Default:     "",
			Description: "Password to the Redis instance used in case of synchronization method 'REDIS'.",
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

	c.vSSHPrivateKey = getter(keySSHPrivateKey)
	c.vSSHPrivateKeyPassword = getter(keySSHPrivateKeyPassword)

	c.vHelmDefaultReleaseName = getter(keyHelmDefaultReleaseName)
	c.vHelmDefaultKubernetesAPIVersions = stringutils.SplitUniqueNonEmpty(getter(keyHelmDefaultKubernetesAPIVersions), ",")
	c.vHelmDefaultKubernetesNamespace = getter(keyHelmDefaultKubernetesNamespace)

	vSynchronizationMethod, err := parseSynchronizationMethod(getter(keySynchronizationMethod))
	if err != nil {
		return err
	}
	c.vSynchronizationMethod = vSynchronizationMethod
	c.vSynchronizationRedisURL = getter(keySynchronizationRedisURL)
	c.vSynchronizationRedisPassword = getter(keySynchronizationRedisPassword)

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

func parseSynchronizationMethod(value string) (SynchronizationMethod, error) {
	switch value {
	case "REDIS":
		return SynchronizationMethodRedis, nil
	case "MEMORY":
		return SynchronizationMethodMemory, nil
	default:
		return SynchronizationMethodUnknown, fmt.Errorf("invalid synchronization method: '%s'", value)
	}
}
