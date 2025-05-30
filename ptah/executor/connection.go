package executor

import (
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"

	"github.com/denisvmedia/inventario/ptah/schema/parser/parsertypes"
)

// ConnectToDatabase creates a database connection from a URL
func ConnectToDatabase(dbURL string) (*DatabaseConnection, error) {
	// Handle MySQL URLs specially since they have a different format
	var parsedURL *url.URL
	var err error

	if (strings.HasPrefix(dbURL, "mysql://") || strings.HasPrefix(dbURL, "mariadb://")) && strings.Contains(dbURL, "@tcp(") {
		// For MySQL/MariaDB URLs, create a fake parseable URL for scheme detection
		fakeURL := strings.Replace(dbURL, "@tcp(", "@", 1)
		fakeURL = strings.Replace(fakeURL, ")", "", 1)
		parsedURL, err = url.Parse(fakeURL)
	} else {
		parsedURL, err = url.Parse(dbURL)
	}

	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	// Check for empty or invalid scheme
	if parsedURL.Scheme == "" {
		return nil, fmt.Errorf("invalid database URL: missing scheme")
	}

	// Determine the dialect
	dialect := strings.ToLower(parsedURL.Scheme)
	switch dialect {
	case "postgres", "postgresql":
		dialect = "postgres"
	case "mysql":
		dialect = "mysql"
	case "mariadb":
		dialect = "mysql" // MariaDB uses MySQL driver and protocol
	default:
		return nil, fmt.Errorf("unsupported database dialect: %s", dialect)
	}

	// Connect to the database
	// For MySQL/MariaDB, we need to convert the URL format
	connectionString := dbURL
	if dialect == "mysql" {
		connectionString = convertMySQLURL(dbURL)
	}

	db, err := sql.Open(dialect, connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Get database info
	info, err := getDatabaseInfo(db, dialect, parsedURL)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to get database info: %w", err)
	}

	// Create appropriate schema reader and writer
	var reader SchemaReader
	var writer SchemaWriter
	switch dialect {
	case "postgres":
		reader = NewPostgreSQLReader(db, info.Schema)
		writer = NewPostgreSQLWriter(db, info.Schema)
	case "mysql":
		reader = NewMySQLReader(db, info.Schema)
		writer = NewMySQLWriter(db, info.Schema)
	default:
		db.Close()
		return nil, fmt.Errorf("no schema reader available for dialect: %s", dialect)
	}

	return &DatabaseConnection{
		db:     db,
		info:   info,
		reader: reader,
		writer: writer,
	}, nil
}

// DatabaseConnection represents a database connection with metadata
type DatabaseConnection struct {
	db     *sql.DB
	info   parsertypes.DatabaseInfo
	reader SchemaReader
	writer SchemaWriter
}

func (dc *DatabaseConnection) Reader() SchemaReader {
	return dc.reader
}

func (dc *DatabaseConnection) Writer() SchemaWriter {
	return dc.writer
}

func (dc *DatabaseConnection) Info() parsertypes.DatabaseInfo {
	return dc.info
}

// QueryRow executes a query that returns a single row
func (dc *DatabaseConnection) QueryRow(query string, args ...interface{}) *sql.Row {
	return dc.db.QueryRow(query, args...)
}

// Query executes a query that returns multiple rows
func (dc *DatabaseConnection) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return dc.db.Query(query, args...)
}

// Close closes the database connection
func (dc *DatabaseConnection) Close() error {
	if dc.db != nil {
		return dc.db.Close()
	}
	return nil
}

// FormatDatabaseURL formats a database URL for display (hiding password)
func FormatDatabaseURL(dbURL string) string {
	// Handle MySQL/MariaDB URLs specially since they have a different format
	if (strings.HasPrefix(dbURL, "mysql://") || strings.HasPrefix(dbURL, "mariadb://")) && strings.Contains(dbURL, "@tcp(") {
		// For MySQL/MariaDB URLs like mysql://user:pass@tcp(host:port)/db?params
		// Just replace the password part
		re := regexp.MustCompile(`://([^:]+):([^@]+)@`)
		return re.ReplaceAllString(dbURL, "://$1:***@")
	}

	parsedURL, err := url.Parse(dbURL)
	if err != nil {
		return dbURL
	}

	// Hide password
	if parsedURL.User != nil {
		if _, hasPassword := parsedURL.User.Password(); hasPassword {
			// Create a new URL string manually to avoid URL encoding of ***
			username := parsedURL.User.Username()
			host := parsedURL.Host
			scheme := parsedURL.Scheme
			path := parsedURL.Path

			result := scheme + "://" + username + ":***@" + host + path
			if parsedURL.RawQuery != "" {
				result += "?" + parsedURL.RawQuery
			}
			return result
		}
	}

	return parsedURL.String()
}

// getDatabaseInfo retrieves database metadata
func getDatabaseInfo(db *sql.DB, dialect string, parsedURL *url.URL) (parsertypes.DatabaseInfo, error) {
	info := parsertypes.DatabaseInfo{
		Dialect: dialect,
	}

	switch dialect {
	case "postgres":
		// Get PostgreSQL version
		var version string
		err := db.QueryRow("SELECT version()").Scan(&version)
		if err != nil {
			return info, fmt.Errorf("failed to get PostgreSQL version: %w", err)
		}
		info.Version = version

		// Get schema name (default to 'public' if not specified in URL)
		schema := "public"
		if parsedURL.Path != "" && len(parsedURL.Path) > 1 {
			// Extract database name from path, schema is typically 'public'
			// For PostgreSQL, schema is usually specified via search_path or defaults to 'public'
			schema = "public"
		}
		info.Schema = schema

	case "mysql", "mariadb":
		// Get MySQL/MariaDB version
		var version string
		err := db.QueryRow("SELECT VERSION()").Scan(&version)
		if err != nil {
			return info, fmt.Errorf("failed to get MySQL/MariaDB version: %w", err)
		}
		info.Version = version

		// Get database name from URL path
		if parsedURL.Path != "" && len(parsedURL.Path) > 1 {
			info.Schema = parsedURL.Path[1:] // Remove leading '/'
		} else {
			// Get current database
			var dbName string
			err := db.QueryRow("SELECT DATABASE()").Scan(&dbName)
			if err != nil {
				return info, fmt.Errorf("failed to get current database name: %w", err)
			}
			info.Schema = dbName
		}
	}

	return info, nil
}

// convertMySQLURL converts a MySQL/MariaDB URL from standard format to Go driver format
func convertMySQLURL(dbURL string) string {
	// If the URL is already in the correct format (contains @tcp), return as-is
	if strings.Contains(dbURL, "@tcp(") {
		// Remove the mysql:// or mariadb:// prefix if present
		if strings.HasPrefix(dbURL, "mysql://") {
			return strings.TrimPrefix(dbURL, "mysql://")
		}
		if strings.HasPrefix(dbURL, "mariadb://") {
			return strings.TrimPrefix(dbURL, "mariadb://")
		}
		return dbURL
	}

	// Parse the URL
	parsedURL, err := url.Parse(dbURL)
	if err != nil {
		return dbURL // Return as-is if parsing fails
	}

	// Extract components
	user := parsedURL.User.Username()
	password, _ := parsedURL.User.Password()
	host := parsedURL.Host
	dbName := strings.TrimPrefix(parsedURL.Path, "/")
	query := parsedURL.RawQuery

	// Build MySQL connection string: user:password@tcp(host)/database?params
	connectionString := fmt.Sprintf("%s:%s@tcp(%s)/%s", user, password, host, dbName)
	if query != "" {
		connectionString += "?" + query
	}

	return connectionString
}
