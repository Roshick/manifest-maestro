package vault

import (
	"encoding/json"
	"os"

	auconfigapi "github.com/StephanHCB/go-autumn-config-api"
)

const (
	defaultKeyServer                  = "VAULT_SERVER"
	defaultKeyCertificateFilePath     = "VAULT_CERTIFICATE_FILE_PATH"
	defaultKeyAuthToken               = "VAULT_AUTH_TOKEN"
	defaultKeyAuthKubernetesRole      = "VAULT_AUTH_KUBERNETES_ROLE"
	defaultKeyAuthKubernetesTokenPath = "VAULT_AUTH_KUBERNETES_TOKEN_PATH"
	defaultKeyAuthKubernetesBackend   = "VAULT_AUTH_KUBERNETES_BACKEND"
	defaultKeySecretsConfig           = "VAULT_SECRETS_CONFIG"
)

type Config interface {
	Server() string
	PublicCertificate() []byte
	AuthToken() string
	AuthKubernetesRole() string
	AuthKubernetesTokenPath() string
	AuthKubernetesBackend() string
	SecretsConfig() SecretsConfig
}

type SecretsConfig map[string][]SecretConfig

type SecretConfig struct {
	VaultKey  string  `json:"vaultKey"`
	ConfigKey *string `json:"configKey,omitempty"`
}

type DefaultConfigImpl struct {
	vServer                  string
	vPublicCertificate       []byte
	vAuthToken               string
	vAuthKubernetesRole      string
	vAuthKubernetesTokenPath string
	vAuthKubernetesBackend   string
	vSecretsConfig           SecretsConfig
}

func NewDefaultConfig() *DefaultConfigImpl {
	return &DefaultConfigImpl{}
}

func (c *DefaultConfigImpl) Server() string {
	return c.vServer
}

func (c *DefaultConfigImpl) PublicCertificate() []byte {
	return c.vPublicCertificate
}

func (c *DefaultConfigImpl) AuthToken() string {
	return c.vAuthToken
}

func (c *DefaultConfigImpl) AuthKubernetesRole() string {
	return c.vAuthKubernetesRole
}

func (c *DefaultConfigImpl) AuthKubernetesTokenPath() string {
	return c.vAuthKubernetesTokenPath
}

func (c *DefaultConfigImpl) AuthKubernetesBackend() string {
	return c.vAuthKubernetesBackend
}

func (c *DefaultConfigImpl) SecretsConfig() SecretsConfig {
	return c.vSecretsConfig
}

func (c *DefaultConfigImpl) ConfigItems() []auconfigapi.ConfigItem {
	return []auconfigapi.ConfigItem{
		{
			Key:         defaultKeyServer,
			EnvName:     defaultKeyServer,
			Default:     "http://localhost",
			Description: "",
			Validate:    auconfigapi.ConfigNeedsNoValidation,
		},
		{
			Key:         defaultKeyAuthToken,
			EnvName:     defaultKeyAuthToken,
			Default:     "",
			Description: "authentication token used to fetch secrets.",
			Validate:    auconfigapi.ConfigNeedsNoValidation,
		},
		{
			Key:         defaultKeyAuthKubernetesRole,
			EnvName:     defaultKeyAuthKubernetesRole,
			Default:     "",
			Description: "role binding to use for vault kubernetes authentication.",
			Validate:    auconfigapi.ConfigNeedsNoValidation,
		},
		{
			Key:         defaultKeyAuthKubernetesTokenPath,
			EnvName:     defaultKeyAuthKubernetesTokenPath,
			Default:     "/var/run/secrets/kubernetes.io/serviceaccount/token",
			Description: "file path to the service-account token",
			Validate:    auconfigapi.ConfigNeedsNoValidation,
		},
		{
			Key:         defaultKeyAuthKubernetesBackend,
			EnvName:     defaultKeyAuthKubernetesBackend,
			Default:     "",
			Description: "authentication path for the kubernetes cluster",
			Validate:    auconfigapi.ConfigNeedsNoValidation,
		},
		{
			Key:         defaultKeySecretsConfig,
			EnvName:     defaultKeySecretsConfig,
			Default:     "{}",
			Description: "config consisting of vault paths and keys to fetch from the corresponding path. values will be written to the global config object.",
			Validate:    auconfigapi.ConfigNeedsNoValidation,
		},
	}
}

func (c *DefaultConfigImpl) ObtainValues(getter func(string) string) error {
	c.vServer = getter(defaultKeyServer)
	if vPublicCertificate, err := loadPublicCertificate(getter(defaultKeyCertificateFilePath)); err != nil {
		return err
	} else {
		c.vPublicCertificate = vPublicCertificate
	}
	c.vAuthToken = getter(defaultKeyAuthToken)
	c.vAuthKubernetesRole = getter(defaultKeyAuthKubernetesRole)
	c.vAuthKubernetesTokenPath = getter(defaultKeyAuthKubernetesTokenPath)
	c.vAuthKubernetesBackend = getter(defaultKeyAuthKubernetesBackend)
	if vSecretsConfig, err := parseSecretsConfig(getter(defaultKeySecretsConfig)); err != nil {
		return err
	} else {
		c.vSecretsConfig = vSecretsConfig
	}

	return nil
}

func parseSecretsConfig(value string) (SecretsConfig, error) {
	var secretsConfig SecretsConfig
	if err := json.Unmarshal([]byte(value), &secretsConfig); err != nil {
		return nil, err
	}
	return secretsConfig, nil
}

func loadPublicCertificate(filepath string) ([]byte, error) {
	if filepath != "" {
		publicCertBytes, err := os.ReadFile(filepath)
		if err != nil {
			return nil, err
		}
		return publicCertBytes, nil
	} else {
		return nil, nil
	}
}
