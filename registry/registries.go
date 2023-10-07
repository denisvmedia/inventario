package registry

import (
	"maps"
	"net/url"
)

type Config string

func (c Config) Parse() (*url.URL, error) {
	return url.Parse(string(c))
}

type SetFunc func(Config) (*Set, error)

var registries map[string]SetFunc

// Register registers a new registry set func.
// It panics if the name is already registered.
// It is intended to be called from the init function in the registry set package.
// It is NOT safe for concurrent use.
func Register(name string, f SetFunc) {
	if _, ok := registries[name]; ok {
		panic("registry: duplicate registry name")
	}

	registries[name] = f
}

// Unregister unregisters a registry set func.
// It is intended to be called from tests.
// It is NOT safe for concurrent use.
func Unregister(name string) {
	delete(registries, name)
}

// Registries returns a map of registered registry set funcs.
// It can be used concurrently with itself, but not with Register or Unregister.
func Registries() map[string]SetFunc {
	return maps.Clone(registries)
}
