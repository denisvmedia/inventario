package registry

import "errors"

// DatabaseCapabilities defines what features a database backend supports
type DatabaseCapabilities struct {
	FullTextSearch    bool // PostgreSQL tsvector, MySQL FULLTEXT
	JSONBOperators    bool // PostgreSQL JSONB operators (@>, ?, etc.)
	AdvancedIndexing  bool // GIN, GiST, partial indexes
	Triggers          bool // Database triggers
	StoredProcedures  bool // Stored procedures/functions
	BulkOperations    bool // Efficient bulk insert/update
	Transactions      bool // Transaction support
	ArrayOperations   bool // Array data types and operations
}

// CapabilityMatrix defines the capabilities of each database backend
var CapabilityMatrix = map[string]DatabaseCapabilities{
	"postgres": {
		FullTextSearch:   true,
		JSONBOperators:   true,
		AdvancedIndexing: true,
		Triggers:         true,
		StoredProcedures: true,
		BulkOperations:   true,
		Transactions:     true,
		ArrayOperations:  true,
	},
	"mysql": {
		FullTextSearch:   true,
		JSONBOperators:   false, // JSON but not JSONB
		AdvancedIndexing: false,
		Triggers:         true,
		StoredProcedures: true,
		BulkOperations:   true,
		Transactions:     true,
		ArrayOperations:  false,
	},
	"boltdb": {
		FullTextSearch:   false,
		JSONBOperators:   false,
		AdvancedIndexing: false,
		Triggers:         false,
		StoredProcedures: false,
		BulkOperations:   false,
		Transactions:     true,
		ArrayOperations:  false,
	},
	"memory": {
		FullTextSearch:   false,
		JSONBOperators:   false,
		AdvancedIndexing: false,
		Triggers:         false,
		StoredProcedures: false,
		BulkOperations:   false,
		Transactions:     false,
		ArrayOperations:  false,
	},
}

// Common errors for unsupported features
var (
	ErrFeatureNotSupported = errors.New("feature not supported by this database backend")
)

// GetCapabilities returns the capabilities for a given database type
func GetCapabilities(dbType string) (DatabaseCapabilities, bool) {
	caps, exists := CapabilityMatrix[dbType]
	return caps, exists
}

// SupportsFeature checks if a database type supports a specific feature
func SupportsFeature(dbType string, feature func(DatabaseCapabilities) bool) bool {
	caps, exists := GetCapabilities(dbType)
	if !exists {
		return false
	}
	return feature(caps)
}
