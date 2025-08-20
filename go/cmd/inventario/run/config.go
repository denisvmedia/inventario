package run

import (
	"github.com/denisvmedia/inventario/internal/defaults"
)

type Config struct {
	Addr                 string `yaml:"addr" env:"ADDR" env-default:":3333"`
	UploadLocation       string `yaml:"upload_location" env:"UPLOAD_LOCATION" env-default:""`
	MaxConcurrentExports int    `yaml:"max_concurrent_exports" env:"MAX_CONCURRENT_EXPORTS" env-default:"0"`
	MaxConcurrentImports int    `yaml:"max_concurrent_imports" env:"MAX_CONCURRENT_IMPORTS" env-default:"0"`
}

func (c *Config) setDefaults() {
	if c.Addr == "" {
		c.Addr = defaults.GetServerAddr()
	}
	if c.UploadLocation == "" {
		c.UploadLocation = defaults.GetUploadLocation()
	}
	if c.MaxConcurrentExports == 0 {
		c.MaxConcurrentExports = defaults.GetMaxConcurrentExports()
	}
	if c.MaxConcurrentImports == 0 {
		c.MaxConcurrentImports = defaults.GetMaxConcurrentImports()
	}
}
