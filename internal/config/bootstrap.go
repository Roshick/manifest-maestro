package config

import (
	"fmt"

	auconfigapi "github.com/StephanHCB/go-autumn-config-api"
)

type LogType int64

const (
	LogStylePlain LogType = iota
	LogStyleJSON
)

const (
	keyLogStyle = "LOG_STYLE"

	keyVaultEnabled = "VAULT_ENABLED"
)

type BootstrapConfig struct {
	vLogType LogType

	vVaultEnabled bool
}

func NewBootstrapConfig() *BootstrapConfig {
	return &BootstrapConfig{}
}

func (c *BootstrapConfig) LogType() LogType {
	return c.vLogType
}

func (c *BootstrapConfig) VaultEnabled() bool {
	return c.vVaultEnabled
}

func (c *BootstrapConfig) ConfigItems() []auconfigapi.ConfigItem {
	return []auconfigapi.ConfigItem{
		{
			Key:         keyLogStyle,
			EnvName:     keyLogStyle,
			Default:     "PLAIN",
			Description: "",
		}, {
			Key:         keyVaultEnabled,
			EnvName:     keyVaultEnabled,
			Default:     "false",
			Description: "",
		},
	}
}

func (c *BootstrapConfig) ObtainValues(getter func(string) string) error {
	if vLogType, err := parseLogType(getter(keyLogStyle)); err != nil {
		return err
	} else {
		c.vLogType = vLogType
	}

	if vVaultEnabled, err := parseBoolean(getter(keyVaultEnabled)); err != nil {
		return err
	} else {
		c.vVaultEnabled = vVaultEnabled
	}
	return nil
}

func parseLogType(value string) (LogType, error) {
	switch value {
	case "JSON":
		return LogStyleJSON, nil
	case "PLAIN":
		return LogStylePlain, nil
	default:
		return LogStylePlain, fmt.Errorf("invalid log type: '%s'", value)
	}
}
