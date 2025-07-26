# PostgreSQL-Centric Architecture

This document describes Inventario's PostgreSQL-first database architecture and how it provides enhanced features while maintaining backward compatibility with other database backends.

## Overview

Inventario now uses a **PostgreSQL-centric architecture** where PostgreSQL is the reference implementation with full feature support, while other databases provide graceful degradation with fallback implementations.

## Database Feature Matrix

| Feature | PostgreSQL | MySQL | BoltDB | Memory |
|---------|------------|-------|--------|--------|
| **Full-Text Search** | ✓ | ✓ | ✗ | ✗ |
| **JSONB Operators** | ✓ | ✗ | ✗ | ✗ |
| **Advanced Indexing** | ✓ | ✗ | ✗ | ✗ |
| **Triggers** | ✓ | ✓ | ✗ | ✗ |
| **Stored Procedures** | ✓ | ✓ | ✗ | ✗ |
| **Bulk Operations** | ✓ | ✓ | ✗ | ✗ |
| **Transactions** | ✓ | ✓ | ✓ | ✗ |
| **Array Operations** | ✓ | ✗ | ✗ | ✗ |

**Legend:**
- ✓ = Native support with optimal performance
- ✗ = Fallback implementation (reduced performance/features)

## PostgreSQL-Specific Features

### 1. Full-Text Search

PostgreSQL provides advanced full-text search with:
- **tsvector** columns for optimized text search
- **Ranking** based on relevance scores
- **Automatic triggers** to maintain search vectors
- **Multi-language support** with stemming

```sql
-- Example: Search commodities with ranking
SELECT c.*, ts_rank(search_vector, plainto_tsquery('laptop')) as rank
FROM commodities c
WHERE search_vector @@ plainto_tsquery('laptop')
ORDER BY rank DESC;
```

### 2. JSONB Operators

Advanced JSON querying with PostgreSQL's JSONB operators:
- `@>` - Contains operator
- `?` - Key exists operator  
- `?|` - Any key exists operator
- `?&` - All keys exist operator
- `||` - Concatenation operator

```sql
-- Example: Find commodities with specific tags
SELECT * FROM commodities 
WHERE tags @> '["electronics", "laptop"]';

-- Example: Find commodities with any of the specified tags
SELECT * FROM commodities 
WHERE tags ?| array['electronics', 'computer'];
```

### 3. Advanced Indexing

PostgreSQL supports specialized index types:
- **GIN indexes** for JSONB and array columns
- **GiST indexes** for full-text search vectors
- **Partial indexes** for conditional indexing
- **Composite indexes** for multi-column queries

```sql
-- Examples from our migration
CREATE INDEX commodities_tags_gin_idx ON commodities USING GIN (tags);
CREATE INDEX commodities_search_vector_idx ON commodities USING GIN (search_vector);
CREATE INDEX commodities_active_idx ON commodities (status, area_id) WHERE draft = false;
```

### 4. Similarity Search

PostgreSQL provides trigram similarity for finding similar items:

```sql
-- Find commodities similar to a reference item
SELECT c.*, similarity(c.name, ref.name) as sim
FROM commodities c, commodities ref
WHERE ref.id = $1 
AND c.id != $1
AND similarity(c.name, ref.name) > 0.3
ORDER BY sim DESC;
```

## API Enhancements

### Enhanced Search Endpoint

The new `/api/v1/search` endpoint provides:
- **Multi-entity search** (commodities, files, areas, locations)
- **Tag-based filtering** with AND/OR operators
- **Pagination** with limit/offset
- **Automatic fallback** for non-PostgreSQL databases

```bash
# Full-text search with PostgreSQL
curl "/api/v1/search?q=laptop&type=commodities&limit=10"

# Tag-based search
curl "/api/v1/search?tags=electronics,computer&operator=AND&type=commodities"

# Database capabilities
curl "/api/v1/search/capabilities"
```

### Capability Detection

The system automatically detects database capabilities:

```json
{
  "data": {
    "id": "capabilities",
    "type": "capabilities", 
    "attributes": {
      "FullTextSearch": true,
      "JSONBOperators": true,
      "AdvancedIndexing": true,
      "Triggers": true,
      "StoredProcedures": true,
      "BulkOperations": true,
      "Transactions": true,
      "ArrayOperations": true
    }
  }
}
```

## Graceful Degradation

For non-PostgreSQL databases, the system provides fallback implementations:

### Full-Text Search Fallback
```go
// PostgreSQL: Uses tsvector with ranking
SELECT c.*, ts_rank(search_vector, plainto_tsquery($1)) as rank
FROM commodities c WHERE search_vector @@ plainto_tsquery($1)

// Fallback: Simple LIKE queries
SELECT * FROM commodities 
WHERE name ILIKE '%query%' OR comments ILIKE '%query%'
```

### JSONB Query Fallback
```go
// PostgreSQL: Native JSONB operators
WHERE tags @> '["electronics"]'

// Fallback: Load all records and filter in Go
commodities := loadAll()
filtered := filterByTags(commodities, tags)
```

## Configuration

### Database Connection Strings

```yaml
# PostgreSQL (recommended - full features)
db-dsn: "postgres://user:password@localhost/inventario"

# MySQL (most features, no JSONB)
db-dsn: "mysql://user:password@localhost/inventario"

# BoltDB (basic features only)
db-dsn: "boltdb://./data/inventario.db"

# Memory (testing only)
db-dsn: "memory://"
```

### Feature Flags

```yaml
# Enable PostgreSQL advanced features
enable-advanced-features: true

# Fallback behavior for unsupported features
# Options: "error", "warn", "silent"
unsupported-feature-handling: "warn"
```

## Migration Strategy

### For Existing Deployments

1. **PostgreSQL users**: Run migrations to get new features
   ```bash
   inventario migrate --db-dsn="postgres://user:pass@localhost/db"
   ```

2. **Other database users**: Continue using existing functionality
   - No migration required
   - Automatic fallback to compatible implementations
   - Consider migrating to PostgreSQL for better performance

### Performance Recommendations

- **Small deployments** (< 1000 items): Any database works fine
- **Medium deployments** (1000-10000 items): PostgreSQL recommended
- **Large deployments** (> 10000 items): PostgreSQL strongly recommended

## CLI Tools

### Feature Matrix Command

View supported features by database type:

```bash
inventario features
```

Output:
```
Inventario Database Feature Matrix
==================================

Feature             postgres    mysql       boltdb      memory      
=======             ========    ========    ========    ========    
FullTextSearch      ✓           ✓           ✗           ✗           
JSONBOperators      ✓           ✗           ✗           ✗           
AdvancedIndexing    ✓           ✗           ✗           ✗           
Triggers            ✓           ✓           ✗           ✗           
StoredProcedures    ✓           ✓           ✗           ✗           
BulkOperations      ✓           ✓           ✗           ✗           
Transactions        ✓           ✓           ✓           ✗           
ArrayOperations     ✓           ✗           ✗           ✗           
```

## Development Guidelines

### Adding New Features

1. **Implement PostgreSQL version first** with full capabilities
2. **Add fallback implementation** for other databases
3. **Update capability matrix** if needed
4. **Add tests** for both enhanced and fallback versions
5. **Document** the feature differences

### Testing Strategy

- **Unit tests** for capability detection
- **Integration tests** with PostgreSQL for enhanced features
- **Fallback tests** with memory/BoltDB databases
- **E2E tests** covering both enhanced and basic functionality

## Migration Path

### From Other Databases to PostgreSQL

1. **Export data** from current database
2. **Set up PostgreSQL** instance
3. **Run migrations** to create schema with advanced features
4. **Import data** using restore functionality
5. **Update configuration** to use PostgreSQL DSN
6. **Restart application** to use enhanced features

The system is designed to make this migration seamless while providing immediate benefits from PostgreSQL's advanced capabilities.
