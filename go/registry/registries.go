package registry

import (
	"maps"
	"net/url"

	"go.5x5.cz/inventario/internal/urlerr"
)

type Config string

// Parse parses the DSN. Its error leaves the DSN out: url.Parse would quote
// it, password included. A postgres:// DSN with an "@" after the host is
// refused (urlerr.CheckDatabaseURL).
func (c Config) Parse() (*url.URL, error) {
	if err := urlerr.CheckDatabaseURL(string(c)); err != nil {
		return nil, err
	}
	return urlerr.Parse(string(c))
}

type SetFunc func(Config) (*FactorySet, error)

var registries = make(map[string]SetFunc)

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

// RegistryNames returns a slice of registered registry set names.
func RegistryNames() []string {
	names := make([]string, 0, len(registries))
	for name := range registries {
		names = append(names, name)
	}

	return names
}

func GetRegistry(conf string) (SetFunc, bool) {
	parsed, err := url.Parse(conf)
	if err != nil {
		return nil, false
	}

	r, ok := registries[parsed.Scheme]
	if !ok {
		return nil, false
	}

	return r, true
}
