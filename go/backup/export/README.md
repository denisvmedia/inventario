# Export Package

The `export` package is a core Go package that handles backup export generation in the Inventario application. By default it produces a **signed, streaming `.inb` archive** (issue #534): a tar containing a signed payload plus the gzip-compressed payload itself. The obsolete XML format is retained only behind the `legacy_xml_backup` build tag.

## Overview

The export package generates backup archives of inventory data, including locations, areas, commodities, and their associated commodity files (images, invoices, manuals). The default build streams the archive directly to blob storage without ever holding the whole payload (or any single file) in memory.

## Backup Format

### Default: signed `.inb` (streaming JSON)

Files in this package split by build tag:

- `jsonexport.go`, `inb_types.go`, `inb_builder.go` (under `//go:build !legacy_xml_backup`) — the default `.inb` exporter.
- `service_legacy_xml.go`, `worker_legacy_xml.go` (under `//go:build legacy_xml_backup`) — the deprecated XML exporter.
- `service_shared.go`, `worker.go`, `userinput.go`, `errors.go` — build-agnostic glue (service struct, worker, file-entity creation).

An `.inb` artifact is an **outer tar** (`internal/inb`) holding:

1. `payload.tar.gz.sig` — an Ed25519 signature (`internal/backupsign`) over the streaming SHA-256 digest of the payload.
2. `payload.tar.gz` — a gzip(tar) payload whose members are:
   - `manifest.json` — written first; format, signing-key info, per-location index, and aggregate statistics (see `inb_types.go`).
   - one `location-<slug>-<uuid>.json` member per location (location → areas → commodities, each commodity bundling its image/invoice/manual file references).
   - `unassigned-commodities.json` — area-less commodities, written only when at least one is in scope (issue #1986).
   - `files/<loc>/<commodity>/<bucket>/<name>` — the raw bytes of each referenced commodity file, streamed directly from blob storage.

The exporter stamps the resulting `FileEntity` with `Ext=".inb"`, `MIMEType="application/x-inventario-backup"`, and `LinkedEntityMeta="inb-2.0"` (`exportFileMeta` in `jsonexport.go`).

### Legacy: XML (build-tag-gated)

The pre-#534 XML format (root `<inventory>` with base64-embedded file data) is **deprecated** and compiled only with `go build -tags legacy_xml_backup`. Do not write new code against it.

## Key Features

### 1. Multiple Export Types
- **Full Database**: Complete export of all inventory data
- **Selected Items**: Export specific locations, areas, or commodities
- **Locations / Areas / Commodities**: Scoped exports
- **Imported**: Placeholder type for externally imported backups (skipped by the worker)

### 2. File Data Handling
- Commodity files (images, invoices, manuals) are streamed straight from blob storage into the archive — never base64-buffered in memory.
- Each file member carries its original basename and metadata so a restore reproduces the original filename.

### 3. Background Processing
- Worker-based architecture for asynchronous export generation
- Prevents API timeouts on large exports
- Semaphore-based concurrency control
- Soft-pause aware (#1308)

### 4. Statistics Tracking
- Counts for locations, areas, commodities, images, invoices, manuals, and total files
- Binary data size tracking, surfaced both on the `Export` record and in `manifest.json`

### 5. Memory-Safe Streaming
- The inner `payload.tar.gz` is spooled to a **local temp file** while a streaming SHA-256 digest is computed via `io.MultiWriter`.
- The signature is taken over that digest (never over a buffered payload), then the finished container is streamed from the temp file into blob storage.

## Architecture

### Core Components

#### ExportService (`service_shared.go`, `jsonexport.go`)
The main service responsible for export generation:
- **ProcessExport()** (`service_shared.go`): orchestrates an export — loads the record, injects user/group context, calls the per-build `generateExport`, creates the backing `FileEntity`, and records statistics. Format-agnostic.
- **generateExport()** (`jsonexport.go`): produces the signed `.inb` archive and returns its blob key plus statistics.
- **writePayload() / writeContainerToBlob()** (`jsonexport.go`): build and sign the streaming payload, then stream the container to blob storage.

#### ExportWorker (`worker.go`)
Background worker that manages export processing:
- **Start() / Stop() / IsRunning()**: lifecycle control
- **processPendingExports()**: finds and processes pending export requests; skips imported exports and respects soft-pause
- Configurable poll interval (default: 10 seconds)
- Semaphore-based concurrency limiting

#### `.inb` Schemas (`inb_types.go`)
Defines the JSON documents written into the inner tar (kept in sync with `backup/restore/types/types_inb.go`):
- **INBManifest**, **INBManifestLoc**, **INBManifestStats**, **INBSignatureInfo**
- **INBLocationDoc**, **INBLocation**, **INBArea**, **INBCommodity**, **INBFileRef**
- **INBUnassignedDoc** (area-less commodities, #1986)
- Format constants: `INBFormatVersion = "2.0"`, `INBManifestName`, `INBUnassignedName`, `INBFilesPrefix`

#### Export Types (`models/export.go`)
- `ExportTypeFullDatabase`, `ExportTypeSelectedItems`, `ExportTypeLocations`, `ExportTypeAreas`, `ExportTypeCommodities`, `ExportTypeImported`

#### Export Status Flow
```
Pending → In Progress → Completed
                    ↘ Failed
```

## Data Flow

### Standard Export Process
1. **Request Creation**: User creates an export request via API
2. **Record Storage**: Export record created with "pending" status
3. **Worker Detection**: `ExportWorker` polls and detects the pending export
4. **Processing**: `ExportService` generates a signed `.inb` archive and streams it to blob storage
5. **Statistics Collection**: real-time tracking of exported entities and file sizes
6. **File Entity**: a `FileEntity` is created for the artifact and linked to the export
7. **Completion**: export record updated with file id, statistics, size, and "completed" status

## Usage

### Creating an Export Service
```go
import (
    "github.com/denisvmedia/inventario/backup/export"
    "github.com/denisvmedia/inventario/internal/backupsign"
)

// signer is *backupsign.Signer (consumed by the .inb exporter; ignored by the
// legacy XML exporter, so the signature is the same across both builds).
service := export.NewExportService(factorySet, uploadLocation, signer)
```

### Starting the Export Worker
```go
// Create worker with max 3 concurrent exports.
worker := export.NewExportWorker(exportService, factorySet, 3)

// Optional configuration via functional options:
//   export.WithPollInterval(5 * time.Second)
//   export.WithPauseController(pauseController) // #1308 soft-pause

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

### Export Arguments
```go
args := export.ExportArgs{
    IncludeFileData: true, // Include commodity file bytes in the archive
}
```

## Performance Considerations

### Memory Efficiency
- **Temp-file spooling**: the payload is digested and signed via a local temp file, never buffered on the heap.
- **File streaming**: commodity file bytes are copied straight from blob storage into the tar.
- **Manifest-first layout**: statistics are fully known before any payload bytes are written, so the import metadata path reads them without inflating file bodies.

### Concurrency Control
- **Semaphore Limiting**: bounds concurrent exports
- **Background Processing**: asynchronous generation prevents API timeouts

## Error Handling
- **Status Tracking**: failed exports are marked with a detailed error message
- **Atomic completion**: the export record is only marked completed after the artifact and its `FileEntity` are written

## Testing

### Test Coverage
- **Unit Tests**: `service_test.go`, `streaming_test.go`, `worker_test.go`, `worker_pause_test.go`
- Uses frankban's quicktest framework
- Memory registries for isolated testing
- Temporary file handling for integration tests

## Configuration

### Inputs
- **Upload Location**: blob storage location for the generated artifact
- **Signer**: `*backupsign.Signer` used to sign the `.inb` archive
- **Worker Concurrency**: maximum concurrent export operations
- **Poll Interval**: export queue polling frequency (`WithPollInterval`)

### Database Integration
- **Export Registry**: export metadata and status
- **Entity / File Registries**: access to locations, areas, commodities, and their files
- **User / Group Registries**: resolved into context for worker-driven exports

## Integration Points

### API Server Integration
- **REST Endpoints**: `/api/v1/exports` for CRUD operations and downloads
- **Public Key**: `/api/v1/backup/public-key` exposes the server's backup verification key

### Storage Integration
- **Blob Storage**: cloud-agnostic file storage, tenant-namespaced (#1793)
- **Database Storage**: metadata and status persistence
