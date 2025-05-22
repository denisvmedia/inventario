package config

import (
	"os"
	"path/filepath"
)

const (
	inventarDir = ".inventar"
)

type Config struct {
	datadir string
}

type Option func(*Config)

func WithDataDir(datadir string) Option {
	return func(c *Config) {
		c.datadir = datadir
	}
}

func New(opts ...Option) *Config {
	c := &Config{}

	for _, o := range opts {
		o(c)
	}

	if c.datadir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			panic("failed to get user home directory, please specify data directory")
		}

		c.datadir = filepath.Join(homeDir, inventarDir)
	}

	return c
}

func (c *Config) DataDir() string {
	return c.datadir
}
