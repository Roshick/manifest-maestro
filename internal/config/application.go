package config

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"reflect"
	"strings"

	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"

	"github.com/caarlos0/env/v11"
)

const (
	ExitCodeSuccess       = 0
	ExitCodeDirtyShutdown = 10
	ExitCodeCreateFailed  = 20
	ExitCodeRunFailed     = 30
)

type SynchronizationMethod int64

const (
	SynchronizationMethodUnknown SynchronizationMethod = iota
	SynchronizationMethodMemory
	SynchronizationMethodRedis
)

type HelmHostProviders map[string]getter.Providers

type ApplicationConfig struct {
	ApplicationName string `env:"APPLICATION_NAME" envDefault:"manifest-maestro"`

	ServerAddress     string `env:"SERVER_ADDRESS"`
	ServerPrimaryPort uint   `env:"SERVER_PRIMARY_PORT" envDefault:"8080"`

	HelmDefaultReleaseName           string            `env:"HELM_DEFAULT_RELEASE_NAME"            envDefault:"RELEASE-NAME"`
	HelmDefaultKubernetesNamespace   string            `env:"HELM_DEFAULT_KUBERNETES_NAMESPACE"    envDefault:"default"`
	HelmDefaultKubernetesAPIVersions []string          `env:"HELM_DEFAULT_KUBERNETES_API_VERSIONS" envDefault:"[]"`
	HelmHostProviders                HelmHostProviders `env:"HELM_HOST_PROVIDERS"                  envDefault:"{}"`

	GitHubAppID             int64          `env:"GITHUB_APP_ID"`
	GitHubAppInstallationID int64          `env:"GITHUB_APP_INSTALLATION_ID"`
	GitHubAppPrivateKey     rsa.PrivateKey `env:"GITHUB_APP_PRIVATE_KEY"`

	SynchronizationMethod        SynchronizationMethod `env:"SYNCHRONIZATION_METHOD"         envDefault:"MEMORY"`
	SynchronizationRedisURL      string                `env:"SYNCHRONIZATION_REDIS_URL"`
	SynchronizationRedisPassword string                `env:"SYNCHRONIZATION_REDIS_PASSWORD"`
}

func NewApplicationConfig() *ApplicationConfig {
	return &ApplicationConfig{}
}

func (c *ApplicationConfig) ObtainValuesFromEnv() error {
	return env.ParseWithOptions(c, env.Options{
		FuncMap: map[reflect.Type]env.ParserFunc{
			reflect.TypeOf(rsa.PrivateKey{}): func(v string) (any, error) {
				return parseGithubPrivateKey(v)
			},
			reflect.TypeOf(SynchronizationMethod(0)): func(v string) (any, error) {
				return parseSynchronizationMethod(v)
			},
			reflect.TypeOf(HelmHostProviders{}): func(v string) (any, error) {
				return parseHelmHostProviders(v)
			},
		},
	})
}

func parseGithubPrivateKey(value string) (rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		return rsa.PrivateKey{}, fmt.Errorf("failed to decode PEM block containing private key")
	}
	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return rsa.PrivateKey{}, fmt.Errorf("failed to parse RSA private key: %w", err)
	}
	return *privateKey, nil
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

type helmHostProvidersRaw = map[string][]helmHostProviderRaw

type helmHostProviderRaw struct {
	Type      string        `json:"type"`
	Schemes   []string      `json:"schemes"`
	BasicAuth *basicAuthRaw `json:"basicAuth"`
}

type basicAuthRaw struct {
	Username       *string `json:"username"`
	Password       *string `json:"password"`
	UsernameEnvVar *string `json:"usernameEnvVar"`
	PasswordEnvVar *string `json:"passwordEnvVar"`
}

func parseHelmHostProviders(raw string) (HelmHostProviders, error) {
	var raws helmHostProvidersRaw
	if err := json.Unmarshal([]byte(raw), &raws); err != nil {
		return nil, fmt.Errorf("invalid HELM_HOST_PROVIDERS: %w", err)
	}
	helmHostProviders := make(map[string]getter.Providers)
	for host, r := range raws {
		providers := make(getter.Providers, 0, len(raws))
		for i, p := range r {
			ptype := strings.ToLower(strings.TrimSpace(p.Type))
			if ptype == "" {
				return nil, fmt.Errorf("helm provider at index %d missing type", i)
			}
			switch ptype {
			case "http", "https":
				providers = append(providers, buildHTTPProviderFromRaw(p))
			case "oci":
				providers = append(providers, buildOCIProviderFromRaw(p))
			default:
				return nil, fmt.Errorf("unsupported helm provider type '%s' at index %d", p.Type, i)
			}
		}
		helmHostProviders[host] = providers
	}
	return helmHostProviders, nil
}

func buildHTTPProviderFromRaw(raw helmHostProviderRaw) getter.Provider {
	schemes := raw.Schemes
	if len(schemes) == 0 {
		schemes = []string{"http", "https"}
	}
	username, password := extractCredentials(raw.BasicAuth)
	return getter.Provider{Schemes: schemes, New: func(options ...getter.Option) (getter.Getter, error) {
		if username != "" && password != "" {
			options = append(options, getter.WithBasicAuth(username, password))
		}
		return getter.NewHTTPGetter(options...)
	}}
}

func buildOCIProviderFromRaw(raw helmHostProviderRaw) getter.Provider {
	schemes := raw.Schemes
	if len(schemes) == 0 {
		schemes = []string{"oci"}
	}
	username, password := extractCredentials(raw.BasicAuth)
	return getter.Provider{Schemes: schemes, New: func(options ...getter.Option) (getter.Getter, error) {
		if username != "" && password != "" {
			client, err := registry.NewClient(
				registry.ClientOptBasicAuth(username, password),
			)
			if err != nil {
				return nil, err
			}
			options = append(options, getter.WithRegistryClient(client))
		}
		return getter.NewOCIGetter(options...)
	}}
}

func extractCredentials(auth *basicAuthRaw) (string, string) {
	if auth == nil {
		return "", ""
	}
	username := ""
	if auth.UsernameEnvVar != nil && *auth.UsernameEnvVar != "" {
		username = os.Getenv(*auth.UsernameEnvVar)
	}
	if auth.Username != nil && *auth.Username != "" {
		username = *auth.Username
	}
	password := ""
	if auth.Password != nil && *auth.Password != "" {
		password = *auth.Password
	}
	if auth.PasswordEnvVar != nil && *auth.PasswordEnvVar != "" {
		password = os.Getenv(*auth.PasswordEnvVar)
	}
	return username, password
}
