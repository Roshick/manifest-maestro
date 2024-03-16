package config

import (
	"github.com/Roshick/go-autumn-vault/pkg/vault"
)

const (
	ExitCodeSuccess       = 0
	ExitCodeDirtyShutdown = 10
	ExitCodeCreateFailed  = 20
	ExitCodeRunFailed     = 30
)

func New() *Config {
	return &Config{
		vBootstrap:   NewBootstrapConfig(),
		vVault:       vault.NewDefaultConfig(),
		vApplication: NewApplicationConfig(),
	}
}

type Config struct {
	vBootstrap   *BootstrapConfig
	vVault       *vault.DefaultConfigImpl
	vApplication *ApplicationConfig
}

func (c *Config) Bootstrap() *BootstrapConfig {
	return c.vBootstrap
}

func (c *Config) Vault() *vault.DefaultConfigImpl {
	return c.vVault
}

func (c *Config) Application() *ApplicationConfig {
	return c.vApplication
}
