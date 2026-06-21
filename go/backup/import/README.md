# Import Package (`importpkg`)

The `importpkg` package handles backup import operations in the Inventario application. It processes a previously-exported backup file and creates a new export record from it (for management and later restoration). By default it expects a **signed `.inb` archive** (issue #534); the obsolete XML format is supported only behind the `legacy_xml_backup` build tag.

## Overview

The import package is the bridge between an uploaded backup file and the system's export management. A user uploads a backup that was previously downloaded from the system (or a compatible one), and it is imported back as a new export record. Import only extracts **metadata** (statistics) from the archive — it does not restore data into the database (that is the `restore` package's job).

## Backup Format

Files in this package split by build tag:

- `jsonimport.go` (under `//go:build !legacy_xml_backup`) — the default `.inb` import path: verifies the archive signature against the server's own key, then reads `manifest.json` for statistics.
- `service_legacy_xml.go` (under `//go:build legacy_xml_backup`) — the deprecated XML import path (streams XML metadata via `backup/export/parser`).
- `service_shared.go`, `worker.go`, `helpers_test.go` — build-agnostic glue (service struct, worker, file-entity creation).

The default `.inb` import verifies the Ed25519 signature **before** inflating, spools the payload to a temp file while digesting (so verification never buffers the whole payload), and only reads the `manifest.json` member — file bytes are never inflated during import. A bad/missing signature or a non-`.inb` upload (e.g. legacy XML) fails hard; there is no bypass.

## Key Features

### 1. External File Integration
- Processes backup files previously downloaded from the system
- Creates a new export record (type "imported") from the uploaded file

### 2. Background Processing
- Worker-based architecture for asynchronous processing
- Polling mechanism to detect and process pending imports
- Soft-pause aware (#1308)

### 3. Metadata Extraction
- Reads `manifest.json` statistics: counts for locations, areas, commodities, images, invoices, manuals, total files, and total file size
- Does **not** restore data to the database (metadata only)

### 4. Concurrency Control
- Semaphore-based limiting of simultaneous import operations

## Architecture

### Core Components

#### ImportService (`service_shared.go`, `jsonimport.go`)
The main service responsible for processing imports:
- **ProcessImport(ctx, exportID, sourceFilePath)** (`service_shared.go`): processes an uploaded backup file and stamps the export record with extracted statistics. Format-agnostic; delegates to the per-build `parseImportMetadata`.
- **parseImportMetadata()** (`jsonimport.go` / `service_legacy_xml.go`): the per-build parser — verifies + reads the `.inb` manifest by default, or parses XML under the legacy build.
- **markImportFailed()**: marks an import as failed with an error message.

#### ImportWorker (`worker.go`)
Background worker that manages import processing:
- **Start() / Stop() / IsRunning()**: lifecycle control
- **processPendingImports()**: finds and processes pending imported exports; respects soft-pause
- **processImport()**: processes a single import (`exportID`, `sourceFilePath`)
- Configurable poll interval (default: 10 seconds)
- Semaphore-based concurrency limiting

## Data Flow

1. **Upload**: User uploads a backup file through the API
2. **Record Creation**: System creates an export record with type "imported" and status "pending"
3. **Worker Detection**: `ImportWorker` polls and detects the pending import
4. **Processing**: `ImportService` verifies the archive, reads its manifest, and extracts statistics
5. **Update**: export record updated with statistics and marked completed
6. **Management**: the import appears in the export catalog and can drive a restore

## Usage

### Creating an Import Service
```go
import (
    importpkg "github.com/denisvmedia/inventario/backup/import"
    "github.com/denisvmedia/inventario/internal/backupsign"
)

// signer is *backupsign.Signer (used by the .inb path to verify the archive
// signature; ignored by the legacy XML path, so the signature is identical
// across both builds).
service := importpkg.NewImportService(factorySet, uploadLocation, signer)
```

### Starting the Import Worker
```go
// Create worker with max 3 concurrent imports.
worker := importpkg.NewImportWorker(importService, factorySet, 3)

// Optional configuration via functional options:
//   importpkg.WithPollInterval(5 * time.Second)
//   importpkg.WithPauseController(pauseController) // #1308 soft-pause

// Start background processing
ctx := context.Background()
worker.Start(ctx)

// Stop when done
defer worker.Stop()
```

### Processing an Import
```go
// Process an import by export id and the uploaded file's blob key.
err := service.ProcessImport(ctx, exportID, sourceFilePath)
if err != nil {
    log.Printf("Import failed: %v", err)
}
```

## Integration with Export System

The import package is tightly integrated with the export system:
- Uses export records with type `models.ExportTypeImported`
- Stores import results in the same export record structure
- Enables unified management of both exports and imports

## Error Handling
- Failed imports are marked with status `models.ExportStatusFailed`
- Error messages are stored in the export record
- A bad/missing signature or non-`.inb` upload fails the import
- `ErrManifestTooLarge` caps the inflated `manifest.json` size (4 MiB) to prevent an out-of-memory DoS

## Testing
- **Unit Tests**: `service_test.go`, `worker_test.go`
- **Integration Tests**: `worker_integration_test.go`
- Uses frankban's quicktest framework

## Configuration

### Inputs
- **Upload Location**: blob storage location for the uploaded backup file
- **Signer**: `*backupsign.Signer` used to verify the `.inb` archive
- **Worker Concurrency**: maximum concurrent import operations
- **Poll Interval**: import queue polling frequency (`WithPollInterval`)

### Database Requirements
- Requires the export registry for storing import records
- Uses blob storage for file handling

## Performance Considerations
- **Memory Efficiency**: the payload is verified via a temp-file digest; only `manifest.json` is inflated — file bytes are never decompressed during import
- **Concurrency**: semaphore-based limiting prevents resource exhaustion
- **Background Processing**: asynchronous processing prevents API timeouts
