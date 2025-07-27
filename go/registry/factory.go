package registry

import (
	"fmt"
	"log"
	"net/url"
)

// RegistryFactory creates registries with capability detection
type RegistryFactory struct {
	registries map[string]SetFunc
}

// NewRegistryFactory creates a new registry factory
func NewRegistryFactory() *RegistryFactory {
	return &RegistryFactory{
		registries: make(map[string]SetFunc),
	}
}

// RegisterBackend registers a database backend
func (f *RegistryFactory) RegisterBackend(name string, setFunc SetFunc) {
	f.registries[name] = setFunc
}

// CreateRegistry creates a registry with capability detection and fallback support
func (f *RegistryFactory) CreateRegistry(dsn string) (any, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DSN: %w", err)
	}

	scheme := parsed.Scheme
	createFunc, exists := f.registries[scheme]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", scheme)
	}

	// Create the base registry
	baseRegistry, err := createFunc(Config(dsn))
	if err != nil {
		return nil, fmt.Errorf("failed to create registry: %w", err)
	}

	// Check if this is PostgreSQL and return enhanced registry
	if scheme == "postgres" {
		log.Printf("Created PostgreSQL registry with enhanced features")
		return baseRegistry, nil // PostgreSQL registry is already enhanced
	}

	// For other databases, wrap with fallback registry
	capabilities, exists := GetCapabilities(scheme)
	if !exists {
		log.Printf("Unknown database type %s, using minimal capabilities", scheme)
		capabilities = DatabaseCapabilities{} // Minimal capabilities
	}

	log.Printf("Created %s registry with fallback support (capabilities: %+v)", scheme, capabilities)
	fallbackRegistry := NewFallbackRegistry(baseRegistry, scheme)

	return fallbackRegistry, nil
}

// CreateEnhancedRegistry creates an enhanced registry interface
func (f *RegistryFactory) CreateEnhancedRegistry(dsn string) (EnhancedRegistry, error) {
	registry, err := f.CreateRegistry(dsn)
	if err != nil {
		return nil, err
	}

	// Try to cast to enhanced registry
	if enhanced, ok := registry.(EnhancedRegistry); ok {
		return enhanced, nil
	}

	// If it's a fallback registry, it should implement EnhancedRegistry
	if fallback, ok := registry.(*FallbackRegistry); ok {
		return fallback, nil
	}

	return nil, fmt.Errorf("registry does not implement EnhancedRegistry interface")
}

// GetSupportedBackends returns a list of supported database backends
func (f *RegistryFactory) GetSupportedBackends() []string {
	backends := make([]string, 0, len(f.registries))
	for name := range f.registries {
		backends = append(backends, name)
	}
	return backends
}

// GetBackendCapabilities returns the capabilities of a specific backend
func (f *RegistryFactory) GetBackendCapabilities(backend string) (DatabaseCapabilities, bool) {
	return GetCapabilities(backend)
}

// DefaultFactory is the default registry factory instance
var DefaultFactory = NewRegistryFactory()

// CreateRegistryWithFactory creates a registry using the default factory
func CreateRegistryWithFactory(dsn string) (any, error) {
	return DefaultFactory.CreateRegistry(dsn)
}

// CreateEnhancedRegistryWithFactory creates an enhanced registry using the default factory
func CreateEnhancedRegistryWithFactory(dsn string) (EnhancedRegistry, error) {
	return DefaultFactory.CreateEnhancedRegistry(dsn)
}

// RegisterBackendWithFactory registers a backend with the default factory
func RegisterBackendWithFactory(name string, setFunc SetFunc) {
	DefaultFactory.RegisterBackend(name, setFunc)
}

// FeatureSupport provides information about feature support across backends
type FeatureSupport struct {
	Feature     string
	Description string
	Backends    map[string]bool
}

// GetFeatureMatrix returns a matrix of features supported by each backend
func GetFeatureMatrix() []FeatureSupport {
	features := []FeatureSupport{
		{
			Feature:     "FullTextSearch",
			Description: "Advanced full-text search with ranking",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "JSONBOperators",
			Description: "JSONB query operators (@>, ?, etc.)",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "AdvancedIndexing",
			Description: "GIN, GiST, and partial indexes",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "Triggers",
			Description: "Database triggers",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "StoredProcedures",
			Description: "Stored procedures and functions",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "BulkOperations",
			Description: "Efficient bulk insert/update operations",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "Transactions",
			Description: "ACID transaction support",
			Backends:    make(map[string]bool),
		},
		{
			Feature:     "ArrayOperations",
			Description: "Array data types and operations",
			Backends:    make(map[string]bool),
		},
	}

	// Populate the matrix
	for backend, capabilities := range CapabilityMatrix {
		for i := range features {
			switch features[i].Feature {
			case "FullTextSearch":
				features[i].Backends[backend] = capabilities.FullTextSearch
			case "JSONBOperators":
				features[i].Backends[backend] = capabilities.JSONBOperators
			case "AdvancedIndexing":
				features[i].Backends[backend] = capabilities.AdvancedIndexing
			case "Triggers":
				features[i].Backends[backend] = capabilities.Triggers
			case "StoredProcedures":
				features[i].Backends[backend] = capabilities.StoredProcedures
			case "BulkOperations":
				features[i].Backends[backend] = capabilities.BulkOperations
			case "Transactions":
				features[i].Backends[backend] = capabilities.Transactions
			case "ArrayOperations":
				features[i].Backends[backend] = capabilities.ArrayOperations
			}
		}
	}

	return features
}

// PrintFeatureMatrix prints a human-readable feature matrix
func PrintFeatureMatrix() {
	features := GetFeatureMatrix()
	backends := []string{"postgres", "mysql", "boltdb", "memory"}

	fmt.Printf("%-20s", "Feature") //nolint:forbidigo // CLI output is OK
	for _, backend := range backends {
		fmt.Printf("%-12s", backend) //nolint:forbidigo // CLI output is OK
	}
	fmt.Println() //nolint:forbidigo // CLI output is OK

	fmt.Printf("%-20s", "=======") //nolint:forbidigo // CLI output is OK
	for range backends {
		fmt.Printf("%-12s", "========") //nolint:forbidigo // CLI output is OK
	}
	fmt.Println() //nolint:forbidigo // CLI output is OK

	for _, feature := range features {
		fmt.Printf("%-20s", feature.Feature) //nolint:forbidigo // CLI output is OK
		for _, backend := range backends {
			if feature.Backends[backend] {
				fmt.Printf("%-12s", "✓") //nolint:forbidigo // CLI output is OK
			} else {
				fmt.Printf("%-12s", "✗") //nolint:forbidigo // CLI output is OK
			}
		}
		fmt.Println() //nolint:forbidigo // CLI output is OK
	}
}
