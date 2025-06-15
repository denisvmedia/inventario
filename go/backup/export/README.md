# Export Package

The `export` package is a core Go package that handles XML export operations in the Inventario application. It provides comprehensive functionality to export inventory data to XML format with support for various export types, file attachments, and background processing.

## Overview

The export package is responsible for generating XML exports of inventory data, including locations, areas, commodities, and their associated files (images, invoices, manuals). It supports multiple export types and implements streaming approaches to handle large datasets efficiently without memory exhaustion.

## Key Features

### 1. Multiple Export Types
- **Full Database**: Complete export of all inventory data
- **Selected Items**: Export specific locations, areas, or commodities
- **Locations**: Export only location data
- **Areas**: Export only area data
- **Commodities**: Export only commodity data
- **Imported**: Special type for externally imported exports

### 2. File Data Handling
- Optional inclusion of binary file data (images, invoices, manuals)
- Base64 encoding of file content for XML embedding
- Streaming approach to prevent memory issues with large files
- Support for multiple file types with proper MIME type handling

### 3. Background Processing
- Worker-based architecture for asynchronous export generation
- Prevents API timeouts on large exports
- Semaphore-based concurrency control
- Automatic cleanup of deleted exports

### 4. Statistics Tracking
- Comprehensive statistics collection during export generation
- Counts for locations, areas, commodities, and files
- Binary data size tracking
- Performance metrics and error reporting

### 5. Streaming XML Generation
- Memory-efficient streaming approach for large datasets
- Direct file-to-XML streaming without loading into memory
- Chunked processing to handle massive inventories
- Real-time statistics collection during streaming

## Architecture

### Core Components

#### ExportService (`service.go`)
The main service responsible for export generation:
- **ProcessExport()**: Main method that orchestrates export generation
- **generateExport()**: Creates XML files using blob storage
- **streamXMLExport()**: Streams XML generation with statistics tracking
- **ParseXMLMetadata()**: Parses existing XML files to extract metadata
- Supports multiple export types with specialized streaming methods

#### ExportWorker (`worker.go`)
Background worker that manages export processing:
- **Start()**: Begins processing exports in the background
- **Stop()**: Gracefully stops the export worker
- **processPendingExports()**: Finds and processes pending export requests
- **cleanupDeletedExports()**: Removes files for soft-deleted exports
- Configurable poll intervals (default: 10 seconds for exports, 1 hour for cleanup)
- Semaphore-based concurrency limiting

#### Export Types (`types.go`)
Defines XML structure and data types:
- **XMLInventory**: Root element containing all export data
- **XMLLocation**, **XMLArea**, **XMLCommodity**: Entity representations
- **File**: File attachment structure with base64 data
- **ExportStats**: Statistics tracking during generation
- **ExportArgs**: Configuration options for export operations

### Export Types and Models

#### Export Status Flow
```
Pending → In Progress → Completed
                    ↘ Failed
```

#### Export Types (`models/export.go`)
- `ExportTypeFullDatabase`: Complete inventory export
- `ExportTypeSelectedItems`: User-selected items export
- `ExportTypeLocations`: Location-only export
- `ExportTypeAreas`: Area-only export
- `ExportTypeCommodities`: Commodity-only export
- `ExportTypeImported`: External import placeholder

## Data Flow

### Standard Export Process
1. **Request Creation**: User creates export request via API
2. **Record Storage**: Export record created with "pending" status
3. **Worker Detection**: ExportWorker polls and detects pending export
4. **Processing**: ExportService generates XML file with streaming approach
5. **Statistics Collection**: Real-time tracking of exported entities and file sizes
6. **Completion**: Export record updated with file path, statistics, and "completed" status
7. **Download**: Users can download generated XML files

### Import Process (External Files)
1. **File Upload**: User uploads external XML file
2. **Import Record**: System creates export record with type "imported"
3. **Metadata Extraction**: ParseXMLMetadata extracts statistics without data restoration
4. **Catalog Integration**: Import appears in export catalog for management

## Usage

### Creating an Export Service
```go
import "github.com/denisvmedia/inventario/backup/export"

// Create export service
service := export.NewExportService(registrySet, uploadLocation)
```

### Starting the Export Worker
```go
// Create worker with max 3 concurrent exports
worker := export.NewExportWorker(exportService, registrySet, 3)

// Start background processing
ctx := context.Background()
worker.Start(ctx)

// Stop when done
defer worker.Stop()
```

### Processing an Export
```go
// Process export by ID
err := service.ProcessExport(ctx, exportID)
if err != nil {
    log.Printf("Export failed: %v", err)
}
```

### Custom Export Arguments
```go
args := export.ExportArgs{
    IncludeFileData: true, // Include binary file data
}
```

## XML Structure

### Root Element
```xml
<inventory xmlns="http://inventario.example.com/schema"
           exportDate="2024-01-15T10:30:00Z"
           exportType="full_database">
```

### Entity Sections
- `<locations>`: Location data with nested areas
- `<areas>`: Standalone area data
- `<commodities>`: Commodity data with file attachments

### File Attachments
```xml
<file id="123" path="image_001" originalPath="uploads/image_001.jpg"
      extension=".jpg" mimeType="image/jpeg">
  <data>base64encodedcontent...</data>
</file>
```

## Performance Considerations

### Memory Efficiency
- **Streaming Approach**: XML generation streams directly to storage
- **Chunked Processing**: Large datasets processed in manageable chunks
- **File Streaming**: Binary files streamed without loading into memory
- **Statistics Tracking**: Real-time collection without memory accumulation

### Concurrency Control
- **Semaphore Limiting**: Prevents resource exhaustion during concurrent exports
- **Background Processing**: Asynchronous generation prevents API timeouts
- **Worker Isolation**: Export and cleanup operations run independently

### Storage Optimization
- **Blob Storage**: Efficient file storage with cloud provider support
- **Cleanup Automation**: Automatic removal of deleted export files
- **Compression**: XML files benefit from HTTP compression during download

## Error Handling

### Comprehensive Error Management
- **Graceful Degradation**: Individual file failures don't stop entire export
- **Status Tracking**: Failed exports marked with detailed error messages
- **Retry Logic**: Worker automatically retries failed operations
- **Logging**: Detailed logging for debugging and monitoring

### Error Recovery
- **Partial Success**: Statistics reflect successfully processed items
- **File Fallbacks**: Missing files logged but don't fail entire export
- **Status Updates**: Real-time status updates during processing

## Testing

### Test Coverage
- **Unit Tests**: `service_test.go`, `worker_test.go`, `types_test.go`
- **Integration Tests**: End-to-end export generation and parsing
- **Performance Tests**: Large dataset handling and memory usage
- **Concurrency Tests**: Multi-worker scenarios and race conditions

### Test Framework
- Uses frankban's quicktest framework
- Table-driven tests for multiple scenarios
- Mock registries for isolated testing
- Temporary file handling for integration tests

## Configuration

### Environment Variables
- **Upload Location**: Configurable blob storage location
- **Worker Concurrency**: Maximum concurrent export operations
- **Poll Intervals**: Export processing and cleanup frequencies
- **File Size Limits**: Maximum export file sizes

### Database Integration
- **Export Registry**: Stores export metadata and status
- **Entity Registries**: Access to locations, areas, commodities
- **File Registries**: Access to images, invoices, manuals
- **Settings Registry**: Global configuration access

## Integration Points

### API Server Integration
- **REST Endpoints**: `/api/v1/exports` for CRUD operations
- **Download Endpoints**: Streaming file downloads
- **Import Endpoints**: External file upload and processing
- **Status Endpoints**: Real-time export status checking

### Frontend Integration
- **Export Service**: TypeScript service for API communication
- **Progress Tracking**: Real-time status updates
- **Download Management**: File download handling
- **Import Interface**: External file upload interface

### Storage Integration
- **Blob Storage**: Cloud-agnostic file storage
- **Database Storage**: Metadata and status persistence
- **File Management**: Automatic cleanup and organization

## Future Enhancements

Potential areas for future development:
- **Progress Reporting**: Real-time progress updates during generation
- **Compression Options**: Optional ZIP compression for large exports
- **Format Support**: Revise the format to use tar.gz, which will contain XML file with DB records and files that the XML refers to
- **Scheduling**: Automated export generation on schedules
- **Validation**: XML schema validation for generated exports
- **Encryption**: Optional encryption for sensitive data exports
