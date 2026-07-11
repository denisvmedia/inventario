# Export Package

The `export` package is a core Go package that handles backup export generation in the Inventario application. By default it produces a **signed, streaming `.inb` archive** (issue #534): a tar containing a signed payload plus the gzip-compressed payload itself. The obsolete XML format is retained only behind the `legacy_xml_backup` build tag.

## Overview

The export package generates backup archives of inventory data: locations, areas, commodities, and **every** group file — commodity attachments (images, invoices, manuals), files attached to a location or an area, and standalone (unlinked) files (issue #2235). The default build streams the archive directly to blob storage without ever holding the whole payload (or any single file) in memory.

Export artifacts themselves (`linked_entity_type = "export"`) are deliberately excluded: a backup must not embed previous backups.

## Backup Format

### Default: signed `.inb` (streaming JSON)

Files in this package split by build tag:

- `jsonexport.go`, `inb_types.go`, `inb_builder.go` (under `//go:build !legacy_xml_backup`) — the default `.inb` exporter.
- `service_legacy_xml.go`, `worker_legacy_xml.go` (under `//go:build legacy_xml_backup`) — the deprecated XML exporter.
- `service_shared.go`, `worker.go`, `userinput.go`, `errors.go` — build-agnostic glue (service struct, worker, file-entity creation).

An `.inb` artifact is an **outer tar** (`internal/inb`) holding:

1. `payload.tar.gz.sig` — an Ed25519 signature (`internal/backupsign`) over the streaming SHA-256 digest of the payload.
2. `payload.tar.gz` — a gzip(tar) payload whose members are, **in this order**:
   - `manifest.json` — written first; format, signing-key info, per-location index, and aggregate statistics (see `inb_types.go`).
   - one `location-<slug>-<uuid>.json` member per location (location → areas → commodities, each commodity bundling its image/invoice/manual file references), each immediately followed by that location's commodity file bytes at `files/<loc-slug>/<commodity-uuid>/<bucket>/<file-uuid>/<name>`.
   - `unassigned-commodities.json` — area-less commodities, written only when at least one is in scope (issue #1986), followed by their file bytes.
   - `files/_index.json` — the **non-commodity files** document (issue #2235): every location-linked, area-linked and standalone file, each carrying its own `linkedEntityType` / `linkedEntityId` (the linked entity's immutable **UUID**) / `linkedEntityMeta` plus its `type` and `category`. Written only when at least one such file is in scope. Its bytes follow at `files/_entity/<type>/<entity-uuid>/<bucket>/<file-uuid>/<name>` and `files/_standalone/<file-uuid>/<name>`.

Ordering is load-bearing in two ways: a JSON document always precedes the file bytes it references (restore registers each reference, then matches the members against it), and `files/_index.json` is written **last** — restore resolves each entity link through the location/area ID mapping, which only fills as the location documents are applied.

The exporter stamps the resulting `FileEntity` with `Ext=".inb"`, `MIMEType="application/x-inventario-backup"`, and `LinkedEntityMeta="inb-2.0"` (`exportFileMeta` in `jsonexport.go`). That stamp is a `FileEntity` meta value, not the format version.

### Format versioning

`INBFormatVersion` (manifest `version`) is currently `"2.1"`. The **MAJOR** component is the compatibility contract the restore side enforces: a reader accepts any archive whose major version it knows and rejects a higher one with `ErrUnsupportedFormatVersion` *before* touching any data. MINOR bumps are additive-only — a new **optional** member plus an optional manifest pointer, dispatched by member name — so a 2.0 archive still restores unchanged on a 2.1 reader.

Both optional members (`unassignedFile`, `filesFile`) are omitted entirely when empty, so an archive that uses neither is byte-stable against the 2.0 layout.

### Scope rules for files

| Export type | Commodity attachments | Location/area files | Standalone files |
| --- | --- | --- | --- |
| whole-class (`full_database`, `locations`, `areas`, `commodities`) | of every emitted commodity | all | all |
| `selected_items` | of every emitted commodity | only of the locations/areas **explicitly selected** | none |

Standalone files have no parent entity that could imply them into a selection, so they ride along with whole-class exports only — matching the legacy XML exporter.

`selected_items` scopes entity files on the **explicit** selection, not on what the archive happens to emit: a selected commodity forces its parent location/area document to be emitted so the item has a home, but that implied parent's files stay out. Scoping on "emitted" instead would silently bundle a location's lease/floor plans into an archive where the user only picked one item — and would diverge from the legacy XML exporter, whose `selectedFileScope` is built from the selected item IDs alone (`service_legacy_xml.go`).

A file whose blob is missing or unsized (orphan row, manual blob delete) is **dropped** from the document and the statistics in every branch. The tar header needs an exact size up front, and a restore hard-fails on a declared reference whose member never arrives.

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
- `imageCount` / `invoiceCount` / `manualCount` are legacy **commodity-scoped** bucket counters. Location-, area-linked and standalone files feed only the unified `fileCount` and `totalFileSize` (#2235) — a location image does not inflate `imageCount`.
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
- **INBFilesDoc**, **INBEntityFileRef** (non-commodity files, #2235)
- Format constants: `INBFormatVersion = "2.1"`, `INBManifestName`, `INBUnassignedName`, `INBFilesPrefix`, `INBFilesName`, `INBEntityFilesPrefix`, `INBStandaloneFilesPrefix`

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
