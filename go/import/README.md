# Import Package (`importpkg`)

The `importpkg` is a Go package that handles XML import operations in the Inventario application. It provides functionality to process previously exported XML files and create new export records from them in the system.

## Overview

The import package serves as a bridge between external XML export files and the system's export management functionality. It allows users to upload XML files that were previously downloaded from the system (or potentially from other compatible systems) and import them back as new export records for management and potential restoration.

## Key Features

### 1. External File Integration
- Processes XML export files that were previously downloaded from the system
- Creates new export records from uploaded XML files
- Supports importing external exports into the export catalog

### 2. Background Processing
- Implements a worker-based architecture for asynchronous processing
- Prevents API server blocking and timeouts on large files
- Uses polling mechanism to detect and process pending imports

### 3. Metadata Extraction
- Parses uploaded XML files to extract comprehensive statistics
- Extracts counts for locations, areas, commodities, and file attachments
- Calculates file sizes and binary data sizes
- Does not restore actual data to the database (only metadata extraction)

### 4. Concurrency Control
- Uses semaphore-based concurrency control
- Limits the number of simultaneous import operations
- Prevents resource exhaustion during heavy import loads

## Architecture

### Core Components

#### ImportService (`service.go`)
The main service responsible for processing XML imports:
- **ProcessImport()**: Main method that processes an XML file and updates export record with metadata
- **markImportFailed()**: Handles error cases and marks imports as failed
- Integrates with the export service for XML parsing capabilities

#### ImportWorker (`worker.go`)
Background worker that manages import processing:
- **Start()**: Begins processing imports in the background
- **Stop()**: Gracefully stops the import worker
- **processPendingImports()**: Finds and processes pending import requests
- **processImport()**: Processes individual import operations
- Uses configurable poll intervals (default: 10 seconds)
- Implements semaphore-based concurrency limiting

#### XML Types (`types.go`)
Defines the structure for parsing XML inventory exports:
- **XMLInventory**: Root element of XML exports
- **XMLLocations**, **XMLAreas**, **XMLCommodities**: Section containers
- **ImportStats**: Tracks statistics during import processing
- **ImportProgress**: Represents current progress of import operations

### Data Flow

1. **Upload**: User uploads an XML file through the API
2. **Record Creation**: System creates an export record with type "imported" and status "pending"
3. **Worker Detection**: ImportWorker polls and detects the pending import
4. **Processing**: ImportService processes the XML file and extracts metadata
5. **Update**: Export record is updated with extracted statistics and marked as completed
6. **Management**: Import is now available in the export catalog for management

## Usage

### Creating an Import Service
```go
import "github.com/denisvmedia/inventario/import"

// Create import service
service := importpkg.NewImportService(registrySet, uploadLocation)
```

### Starting the Import Worker
```go
// Create worker with max 3 concurrent imports
worker := importpkg.NewImportWorker(importService, registrySet, 3)

// Start background processing
ctx := context.Background()
worker.Start(ctx)

// Stop when done
defer worker.Stop()
```

### Custom Poll Interval
```go
// Create worker with custom 5-second poll interval
worker := importpkg.NewImportWorkerWithPollInterval(
    importService,
    registrySet,
    3,
    5*time.Second,
)
```

## Integration with Export System

The import package is tightly integrated with the export system:
- Uses export records with type `models.ExportTypeImported`
- Leverages export service's XML parsing capabilities
- Stores import results in the same export record structure
- Enables unified management of both exports and imports

## Error Handling

The package implements comprehensive error handling:
- Failed imports are marked with status `models.ExportStatusFailed`
- Error messages are stored in the export record
- Detailed logging for debugging and monitoring
- Graceful handling of file access and parsing errors

## Testing

The package includes comprehensive test coverage:
- **Unit Tests**: `service_test.go`, `worker_test.go`
- **Integration Tests**: `worker_integration_test.go`
- Uses frankban's quicktest framework
- Includes tests for concurrent operations and error scenarios

## Configuration

### Environment Variables
- Upload location is configurable via the service constructor
- Worker concurrency limits are configurable
- Poll intervals can be customized

### Database Requirements
- Requires export registry for storing import records
- Uses blob storage for file handling
- Integrates with existing registry system

## Performance Considerations

- **Memory Efficiency**: Uses streaming approaches to avoid loading entire XML files into memory
- **Concurrency**: Semaphore-based limiting prevents resource exhaustion
- **Background Processing**: Asynchronous processing prevents API timeouts
- **Polling Optimization**: Configurable poll intervals balance responsiveness and resource usage

## Future Enhancements

Potential areas for future development:
- Real-time progress reporting during import processing
- Support for additional file formats beyond XML
- Enhanced validation and conflict resolution
