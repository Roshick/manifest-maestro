package config

import (
	"github.com/Roshick/manifest-maestro/pkg/go-autumn-vault/pkg/vault"
)

const (
	ExitCodeSuccess       = 0
	ExitCodeDirtyShutdown = 10
	ExitCodeCreateFailed  = 20
	ExitCodeRunFailed     = 30
)

func New() *Impl {
	return &Impl{
		vBootstrap:   NewBootstrapConfig(),
		vVault:       vault.NewDefaultConfig(),
		vApplication: NewApplicationConfig(),
	}
}

type Impl struct {
	vBootstrap   *BootstrapConfig
	vVault       *vault.DefaultConfigImpl
	vApplication *ApplicationConfig
}

func (c *Impl) Bootstrap() *BootstrapConfig {
	return c.vBootstrap
}

func (c *Impl) Vault() *vault.DefaultConfigImpl {
	return c.vVault
}

func (c *Impl) Application() *ApplicationConfig {
	return c.vApplication
}
