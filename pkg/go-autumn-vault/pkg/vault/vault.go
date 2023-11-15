package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	auconfigapi "github.com/StephanHCB/go-autumn-config-api"
)

type Client interface {
	ObtainSecrets(ctx context.Context, path string) (map[string]string, error)
}

type Vault struct {
	config Config
	client Client
}

func New(
	config Config,
	client Client,
) *Vault {
	return &Vault{
		config: config,
		client: client,
	}
}

func (v *Vault) ObtainSecrets(ctx context.Context) (map[string]string, error) {
	values := make(map[string]string)
	for path, secretsConfig := range v.config.SecretsConfig() {
		secrets, err := v.client.ObtainSecrets(ctx, path)
		if err != nil {
			return nil, err
		}
		for _, secretConfig := range secretsConfig {
			vaultKey := secretConfig.VaultKey
			if secret, ok := secrets[vaultKey]; ok {
				configKey := vaultKey
				if secretConfig.ConfigKey != nil && *secretConfig.ConfigKey != "" {
					configKey = *secretConfig.ConfigKey
				}
				if keys := strings.Split(configKey, "."); len(keys) > 1 {
					secretsMap, err := appendSecretToMap(values[keys[0]], keys[1], secret)
					if err != nil {
						return nil, fmt.Errorf("nested secret key %s from vault path %s is not valid", configKey, path)
					}
					values[keys[0]] = secretsMap
				} else {
					values[configKey] = secret
				}
			} else {
				return nil, fmt.Errorf("key %s does not exist at vault path %s", vaultKey, path)
			}
		}
	}
	return values, nil
}

func (v *Vault) ValuesProvider() func([]auconfigapi.ConfigItem) (map[string]string, error) {
	return func(configItems []auconfigapi.ConfigItem) (map[string]string, error) {
		return v.ObtainSecrets(context.Background())
	}
}

func appendSecretToMap(secretMapJson string, secretKey string, secretValue string) (string, error) {
	secretMap := make(map[string]string)
	if secretMapJson != "" {
		if err := json.Unmarshal([]byte(secretMapJson), &secretMap); err != nil {
			return "{}", err
		}
	}
	secretMap[secretKey] = secretValue
	result, err := json.Marshal(secretMap)
	return string(result), err
}
