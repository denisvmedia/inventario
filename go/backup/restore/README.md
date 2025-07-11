# Restore Package

The `restore` package is a core Go package that handles XML restore operations in the Inventario application. It provides comprehensive functionality to restore inventory data from XML exports with support for multiple restore strategies, detailed logging, and background processing.

## Overview

The restore package is responsible for parsing XML export files and restoring inventory data back into the database. It supports multiple restore strategies to handle different scenarios, implements streaming approaches for large files, and provides detailed step-by-step logging of the restoration process.

## Key Features

### 1. Multiple Restore Strategies
- **Full Replace (Destructive)**: Wipes current database and restores everything from backup
- **Merge Add**: Only adds data missing in current database (matched by primary key)
- **Merge Update**: Creates if missing, updates if exists, leaves other records untouched

### 2. Streaming XML Processing
- Memory-efficient streaming approach for large XML files
- On-the-fly processing without loading entire files into memory
- Chunked decoding to handle massive inventories with embedded base64 data
- Real-time statistics collection during processing

### 3. Detailed Step Logging
- Comprehensive step-by-step logging of restoration process
- Emoji-based status indicators (📝, 🔄, ✅, ❌, ⏭️)
- Individual item processing logs showing what was checked, skipped, updated
- Real-time progress tracking with detailed action descriptions

### 4. File Data Restoration
- Restoration of binary file attachments (images, invoices, manuals)
- Base64 decoding and file recreation in blob storage
- Proper MIME type and extension handling
- Error resilience for corrupted or missing file data

### 5. Background Processing
- Worker-based architecture for asynchronous restoration
- Prevents API timeouts on large restore operations
- Semaphore-based concurrency control (only one restore at a time)
- Graceful error handling and status reporting

### 6. Dry Run Support
- Preview mode to see what would be restored without making changes
- Validation of XML structure and data integrity
- Conflict detection and resolution preview
- Safe testing of restore operations

## Architecture

The restore package follows a processor-based architecture where each restore operation is handled by a dedicated `RestoreOperationProcessor` instance. This design provides better isolation, detailed logging, and cleaner separation of concerns.

### Core Components

#### RestoreService (`service.go`)
The main service responsible for restore operations:
- **NewRestoreService()**: Creates a new restore service instance
- **ProcessRestoreOperation()**: Delegates restore processing to RestoreOperationProcessor
- Lightweight service that acts as a factory for restore processors

#### RestoreOperationProcessor (`service.go`)
Core processor that handles restore operations with detailed logging:
- **Process()**: Main orchestration method for complete restore operations
- **RestoreFromXML()**: Processes XML restore with streaming and detailed logging
- **processRestoreWithDetailedLogging()**: Handles detailed step-by-step logging
- **validateOptions()**: Validates restore configuration options
- **clearExistingData()**: Handles full replace strategy data clearing
- **markRestoreFailed()**: Error handling and status management
- Supports multiple restore strategies with specialized processing methods

#### RestoreWorker (`worker.go`)
Background worker that manages restore processing:
- **Start()**: Begins processing restores in the background
- **Stop()**: Gracefully stops the restore worker
- **processPendingRestores()**: Finds and processes pending restore requests
- **processRestore()**: Processes individual restore operations
- Configurable poll intervals (default: 10 seconds)
- Semaphore-based concurrency limiting (max 1 concurrent restore)

#### XML Types (`types.go`)
Defines XML structure and conversion methods:
- **XMLInventory**: Root element containing all restore data
- **XMLLocation**, **XMLArea**, **XMLCommodity**: Entity representations
- **XMLFile**: File attachment structure with base64 data
- **RestoreStats**: Statistics tracking during restoration
- **RestoreOptions**: Configuration options for restore operations

### Processor-Based Architecture

The refactored architecture centers around the `RestoreOperationProcessor` pattern:

#### Key Benefits
- **Operation Isolation**: Each restore operation gets its own processor instance
- **Detailed Logging**: Built-in step-by-step logging with emoji indicators
- **Error Handling**: Comprehensive error tracking and status management
- **Resource Management**: Proper cleanup and resource management per operation
- **Testability**: Easier unit testing with dedicated processor instances

#### Processor Lifecycle
1. **Creation**: `NewRestoreOperationProcessor()` creates processor for specific operation
2. **Validation**: Processor validates restore operation and export file existence
3. **Processing**: `Process()` method orchestrates the complete restore workflow
4. **Logging**: Detailed steps logged throughout the process with status updates
5. **Completion**: Final status and statistics recorded in database

#### Key Processor Methods
- **`Process()`**: Main orchestration method that handles the complete restore workflow
- **`processRestoreWithDetailedLogging()`**: Coordinates XML processing with step logging
- **`restoreFromXMLWithStreamingLogging()`**: Core XML processing with streaming and logging
- **`processLocationWithLogging()`**: Processes individual locations with detailed logging
- **`processAreaWithLogging()`**: Processes individual areas with detailed logging
- **`processCommodityWithLogging()`**: Processes individual commodities with detailed logging
- **`createRestoreStep()`**: Creates new restore steps for progress tracking
- **`updateRestoreStep()`**: Updates existing restore steps with results
- **`predictAction()`**: Predicts what action will be taken based on strategy
- **`markRestoreFailed()`**: Handles error scenarios and status updates

### Restore Strategies

#### Full Replace Strategy
```go
RestoreStrategyFullReplace
```
- **Behavior**: Clears entire database before restoration
- **Use Case**: Complete database replacement from backup
- **Risk**: All current data is lost
- **Validation**: Requires explicit confirmation

#### Merge Add Strategy
```go
RestoreStrategyMergeAdd
```
- **Behavior**: Only adds missing data, skips existing items
- **Use Case**: Adding new data from backup without conflicts
- **Risk**: Low - existing data remains unchanged
- **Matching**: Based on XML IDs and unique identifiers

#### Merge Update Strategy
```go
RestoreStrategyMergeUpdate
```
- **Behavior**: Creates missing items, updates existing ones
- **Use Case**: Synchronizing with updated backup data
- **Risk**: Medium - existing data may be overwritten
- **Matching**: Based on XML IDs with update capability

## Data Flow

### Standard Restore Process
1. **Request Creation**: User creates restore operation via API
2. **Record Storage**: Restore operation created with "pending" status
3. **Worker Detection**: RestoreWorker polls and detects pending operation
4. **Processor Creation**: RestoreOperationProcessor created for the specific operation
5. **Initialization**: Processor begins processing with initial logging and validation
6. **XML Parsing**: Streaming XML parser processes file sections
7. **Data Processing**: Entities processed according to selected strategy with detailed logging
8. **File Restoration**: Binary files decoded and stored in blob storage
9. **Statistics Collection**: Real-time tracking of processed entities
10. **Completion**: Restore operation marked as completed with final statistics

### Detailed Logging Flow
1. **Step Creation**: RestoreOperationProcessor creates restore steps for each major phase
2. **Progress Updates**: Steps updated with in-progress, success, or error status via `createRestoreStep()` and `updateRestoreStep()`
3. **Item Logging**: Individual items logged with emoji indicators using `processLocationWithLogging()`, `processAreaWithLogging()`, `processCommodityWithLogging()`
4. **Action Prediction**: `predictAction()` method predicts what action will be taken for each item based on strategy
5. **Result Tracking**: Actual results compared with predictions and logged accordingly
6. **Error Handling**: Failed items logged with detailed error messages via `markRestoreFailed()`

## Usage

### Creating a Restore Service
```go
import "github.com/denisvmedia/inventario/backup/restore"

// Create restore service
service := restore.NewRestoreService(registrySet, uploadLocation)
```

### Starting the Restore Worker
```go
// Create worker with max 1 concurrent restore
worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation, 1)

// Start background processing
ctx := context.Background()
worker.Start(ctx)

// Stop when done
defer worker.Stop()
```

### Processing a Restore Operation
```go
// Process restore operation by ID
err := service.ProcessRestoreOperation(ctx, restoreOperationID, uploadLocation)
if err != nil {
    log.Printf("Restore failed: %v", err)
}
```

### Direct Processor Usage
```go
// Create processor for specific restore operation
processor := restore.NewRestoreOperationProcessor(restoreOperationID, registrySet, uploadLocation)

// Process the restore operation
err := processor.Process(ctx)
if err != nil {
    log.Printf("Restore failed: %v", err)
}
```

### Restore Options Configuration
```go
options := restore.RestoreOptions{
    Strategy:        restore.RestoreStrategyMergeUpdate,
    IncludeFileData: true,
    DryRun:          false,
}

// Using processor directly for XML restore
processor := restore.NewRestoreOperationProcessor("operation-id", registrySet, uploadLocation)
stats, err := processor.RestoreFromXML(ctx, xmlReader, options)
```

## XML Processing

### Streaming Approach
The restore package uses a streaming XML parser to handle large files efficiently:

```go
decoder := xml.NewDecoder(reader)
for {
    tok, err := decoder.Token()
    if err == io.EOF {
        break
    }
    // Process tokens as they arrive
}
```

### Entity Processing Order
1. **Locations**: Processed first as they are referenced by areas
2. **Areas**: Processed second as they are referenced by commodities
3. **Commodities**: Processed last with full dependency resolution

### File Data Handling
- Base64 encoded data is decoded on-the-fly
- Files are streamed directly to blob storage
- MIME types and extensions are preserved
- Corrupted files are logged but don't stop the restore

## Error Handling

### Comprehensive Error Management
- **Graceful Degradation**: Individual item failures don't stop entire restore
- **Detailed Logging**: All errors logged with context and item information
- **Status Tracking**: Failed restores marked with detailed error messages
- **Rollback Support**: Dry run mode allows safe testing

### Error Recovery
- **Partial Success**: Statistics reflect successfully processed items
- **File Fallbacks**: Missing or corrupted files logged but restore continues
- **Validation Errors**: Data validation failures logged with specific field information
- **Dependency Resolution**: Missing dependencies handled gracefully

## Performance Considerations

### Memory Efficiency
- **Streaming Processing**: XML processed without loading entire file into memory
- **Chunked File Handling**: Large binary files processed in chunks
- **Real-time Statistics**: Statistics collected without memory accumulation
- **Garbage Collection**: Proper resource cleanup and memory management

### Concurrency Control
- **Single Restore Limit**: Only one restore operation allowed at a time
- **Semaphore Protection**: Prevents resource conflicts and data corruption
- **Background Processing**: Asynchronous processing prevents API timeouts
- **Worker Isolation**: Restore operations isolated from other system processes

## Testing

### Test Coverage
- **Unit Tests**: `service_test.go`, `worker_test.go`, `types_test.go`
- **Strategy Tests**: `merge_add_strategy_test.go` and other strategy-specific tests
- **Integration Tests**: End-to-end restore operations with real XML files
- **Error Tests**: Error handling and recovery scenarios

### Test Framework
- Uses frankban's quicktest framework
- Table-driven tests for multiple restore strategies
- RestoreOperationProcessor-based testing for realistic scenarios
- Memory registries for isolated testing
- Temporary file handling for integration tests

## Configuration

### Environment Variables
- **Upload Location**: Configurable blob storage location for file restoration
- **Worker Concurrency**: Maximum concurrent restore operations (recommended: 1)
- **Poll Intervals**: Restore processing frequency
- **Timeout Settings**: Maximum restore operation duration

### Database Integration
- **Restore Operation Registry**: Stores restore metadata and status
- **Restore Step Registry**: Tracks individual restore steps
- **Entity Registries**: Access to locations, areas, commodities for restoration
- **File Registries**: Access to images, invoices, manuals for file restoration

## Integration Points

### API Server Integration
- **REST Endpoints**: `/api/v1/exports/{id}/restores` for restore operations
- **Status Endpoints**: Real-time restore status and progress checking
- **Step Endpoints**: Detailed step-by-step progress monitoring
- **File Endpoints**: File restoration status and error reporting

### Frontend Integration
- **Restore Interface**: User interface for restore configuration
- **Progress Tracking**: Real-time progress updates with step details
- **Strategy Selection**: User-friendly restore strategy selection
- **Dry Run Preview**: Preview mode for safe restore testing

### Storage Integration
- **Blob Storage**: File restoration to cloud-agnostic storage
- **Database Storage**: Metadata and status persistence
- **File Management**: Automatic file organization and cleanup

## Security Considerations

### Data Protection
- **Validation**: Comprehensive input validation for XML data
- **Sanitization**: Proper sanitization of file paths and names
- **Access Control**: Restore operations require appropriate permissions
- **Audit Logging**: Complete audit trail of all restore operations

### File Security
- **Path Validation**: Prevention of directory traversal attacks
- **File Type Validation**: MIME type and extension validation
- **Size Limits**: Protection against oversized file uploads
- **Virus Scanning**: Integration points for file scanning (future enhancement)

## Future Enhancements

Potential areas for future development:
- **Selective Restoration**: Restore specific entities or date ranges
- **Conflict Resolution UI**: Interactive conflict resolution interface
- **Backup Validation**: Pre-restore backup integrity checking
- **Progress Streaming**: Real-time progress updates (e.g. via WebSocket)
- **Rollback Capability**: Automatic rollback on restore failures
- **Format Support**: Revise the format to use tar.gz, which will contain XML file with DB records and files that the XML refers to
