# Restore Package

The `restore` package handles backup restore operations in the Inventario application. It restores inventory data from a backup archive with support for multiple restore strategies, detailed step-by-step logging, and background processing. By default it restores from a **signed `.inb` archive** (issue #534); the obsolete XML format is supported only behind the `legacy_xml_backup` build tag.

## Overview

The restore package reads a backup archive and writes its inventory data back into the database. For the default `.inb` format it **verifies the Ed25519 signature against the server's own key before inflating**, then walks the inner tar applying each entity through strategy-aware model handlers. It supports multiple restore strategies, streams large file members without buffering, and logs the process step by step.

## Backup Format

The decode body splits by build tag; everything around it is format-agnostic.

- `processor/processor.go` (under `//go:build !legacy_xml_backup`) — the default `.inb` restorer: `decodeAndRestore` verifies the signature, then `applyInbPayload` walks the inner tar.
- `processor/processor_legacy_xml.go` (under `//go:build legacy_xml_backup`) — the deprecated XML restorer; this is where the build-tag-gated `RestoreFromXML` entry point lives (used only by the legacy XML test suite).
- `processor/processor_shared.go` — build-agnostic processor glue: `Process`, option validation, strategy handlers, step logging.
- `service.go` — the thin `RestoreService` that constructs a processor.
- `worker.go`, `status_querier.go` — background worker and a query-only status helper.

An `.inb` archive (see the `export` package) is an outer tar of `payload.tar.gz.sig` + `payload.tar.gz`. On restore the payload is spooled to a temp file while a streaming SHA-256 digest is computed, the signature is verified **before** inflate, the manifest's format version is gated, and only then is the inner tar walked: per-location JSON members recreate entities, `files/_drafts.json` recreates the open drafts of new items, `files/_index.json` registers the non-commodity files, and `files/...` members stream their bytes into a re-minted tenant blob key. A bad/missing signature, a non-`.inb` upload, or any framing violation fails the restore hard.

### Format-version gate

Before `prepareRestore` runs — i.e. before a `full_replace` wipes anything — `checkInbFormatVersion` reads the manifest (always the first member) and rejects any archive whose **MAJOR** version is above `maxSupportedInbMajor` with `ErrUnsupportedFormatVersion`. An absent version and any known-major MINOR are accepted, which is what lets a 2.0 archive restore unchanged on a 2.1 reader. Running the gate before the wipe is deliberate: a version rejection must never leave the operator with an emptied database.

### Linked-entity resolution

Every file reference carries its linked entity's **immutable UUID**, never a DB key. At byte time the walker maps it to the destination DB id through `IDMapping` (`Commodities` / `Locations` / `Areas`) or, for a draft, through the drafts the walker restored; a standalone file resolves to no link at all (#2235), and so does a link to a draft the walker did not recreate. A reference whose entity never landed is dropped with a counted error rather than persisted with a dangling `linked_entity_id`. A link to a draft is the exception: when the archive does not carry the draft, or the walker kept the group's draft with that client key, the link is dropped without an error, and a file with no other link is restored as a standalone file. A draft that fails to restore counts one error of its own, and its links are dropped the same way. This is why the exporter emits `files/_index.json` after every location member — the mapping only fills as the location documents are applied.

## Key Features

### 1. Multiple Restore Strategies
- **Full Replace (Destructive)**: Wipes current data and restores everything from backup
- **Merge Add**: Only adds data missing in the current database (matched by immutable UUID)
- **Merge Update**: Creates if missing, updates if exists, leaves other records untouched

### 2. Memory-Safe Streaming
- The verified payload is inflated and walked member-by-member; file bytes are streamed straight into blob storage, never buffered.
- Per-member and total caps bound a hostile archive (`maxInbMembers`, `maxInbTotalUncompress` = 8 GiB, `maxJSONDocBytes` = 32 MiB for every JSON document member, `maxInbManifestBytes` = 4 MiB for the manifest pre-scan).

### 3. Detailed Step Logging
- Step-by-step logging of the restoration process with status updates per phase and per entity
- Real-time progress tracking persisted as restore steps

### 4. File Data Restoration
- File members are streamed into a re-minted tenant blob key, `t/<tenant>/files/<file-uuid><ext>` (never the archive path or source key), so an archive always lands in the importing tenant's namespace
- Commodity attachments, location-, area- and draft-linked files and standalone files all round-trip, each recreated with its original `linked_entity_type` / `linked_entity_id` / `linked_entity_meta` (#2235)
- A declared file reference whose member never arrives fails the restore (`ErrMissingFileMembers`) rather than silently dropping data
- Cover-photo cross-references (#1451) are patched after all files are restored

### 5. Background Processing
- Worker-based architecture for asynchronous restoration
- Semaphore-based concurrency control (default: one restore at a time)
- Soft-pause aware (#1308)

### 6. Dry Run Support
- Preview mode that drains file bytes (for the byte count) but writes nothing

## Architecture

The restore package is processor-based: each restore operation is handled by a dedicated `RestoreOperationProcessor` instance (in the `processor` subpackage) for isolation, detailed logging, and cleaner separation of concerns.

### Core Components

#### RestoreService (`service.go`)
A thin service that constructs and delegates to a processor:
- **NewRestoreService()**: creates the service
- **ProcessRestoreOperation()**: builds a `RestoreOperationProcessor` and calls its `Process()`

#### RestoreOperationProcessor (`processor/`)
Core processor that handles a restore with detailed logging:
- **Process()** (`processor_shared.go`): main orchestration — status/step bookkeeping, blob open, then the per-build `decodeAndRestore`
- **decodeAndRestore()** (`processor.go` / `processor_legacy_xml.go`): the per-build decode/verify/apply body
- **applyInbPayload()** (`processor.go`): walks the verified inner tar
- **RestoreFromXML()** (`processor_legacy_xml.go`, **build-tag-gated**): exported entry point used only by the legacy XML test suite
- **validateOptions()**, **prepareRestore()**, **markRestoreFailed()**, and the strategy handlers (`applyStrategyForLocationModel`, `…AreaModel`, `…CommodityModel`, `…FileModel`) in `processor_shared.go`

#### RestoreWorker (`worker.go`)
Background worker that manages restore processing:
- **Start() / Stop() / IsRunning()**: lifecycle control
- **processPendingRestores()**: finds and processes pending restore operations; respects soft-pause
- **processRestore()**: processes a single restore operation
- **HasRunningRestores()**: delegates to `RegistryStatusQuerier`
- Configurable poll interval (default: 10 seconds) and max-concurrency (default: 1)

#### `.inb` Schemas (`types/types_inb.go`)
Decoded counterparts to the export-side `.inb` documents (kept in sync with `backup/export/inb_types.go`): `INBLocationDoc`, `INBLocation`, `INBArea`, `INBCommodity`, `INBFileRef`, `INBUnassignedDoc`, `INBDraftsDoc`, `INBDraft`, `INBIntakeDocumentsDoc`, `INBIntakeDocument`, `INBFilesDoc`, `INBEntityFileRef`, `INBFileLink`, plus the member-name constants `INBManifestMember`, `INBUnassignedMember`, `INBDraftsMember`, `INBIntakeDocumentsMember`, `INBFilesMember`.

#### Restore Types (`types/types.go`)
- **RestoreOptions**: `Strategy`, `IncludeFileData`, `DryRun`
- **RestoreStrategy** + constants: `RestoreStrategyFullReplace` (`"full_replace"`), `RestoreStrategyMergeAdd` (`"merge_add"`), `RestoreStrategyMergeUpdate` (`"merge_update"`)
- **RestoreStats**, **ExistingEntities**, **IDMapping**

### Restore Strategies

#### Full Replace Strategy
```go
types.RestoreStrategyFullReplace // "full_replace"
```
- **Behavior**: Clears existing data before restoration
- **Use Case**: Complete replacement from backup
- **Risk**: All current data is lost

`clearExistingData` runs four passes, in order (see `processor_shared.go`):

1. `DeleteLocationRecursive` per location — cascades areas, their commodities, and the files of that subtree.
2. `DeleteCommodityRecursive` per **surviving** commodity. Commodities are enumerated directly because `commodity.area_id` is nullable since #1986: an area-less commodity is unreachable through the location → area recursion, so before #2236 its row survived the wipe while the file sweep still deleted its attachments — a zombie item with no files, whose preserved UUID then collided with the archive's own copy on re-create.
3. The group's drafts, in every state, with their file links. The archive's open drafts take their place: `files/_drafts.json` recreates them and `files/_index.json` links their files to them again. The group's intake sessions and its record of delivered documents go too; `files/_intake_documents.json` brings back the archive's record.
4. A type-agnostic sweep of the remaining file rows + blobs, catching standalone files and any orphan.

Files with `linked_entity_type = "export"` are skipped by the sweep: that is the archive being restored, plus the backup history — not user inventory.

#### Merge Add Strategy
```go
types.RestoreStrategyMergeAdd // "merge_add"
```
- **Behavior**: Only adds missing data, skips existing items
- **Risk**: Low — existing data remains unchanged
- **Matching**: Based on immutable UUIDs

#### Merge Update Strategy
```go
types.RestoreStrategyMergeUpdate // "merge_update"
```
- **Behavior**: Creates missing items, updates existing ones
- **Risk**: Medium — existing data may be overwritten
- **Matching**: Based on immutable UUIDs with update capability

### Drafts

`files/_drafts.json` (format 3.5) holds the group's open drafts of new items. The walker recreates each one with new ids and the archived revision and timestamps (through `registry.WithRestoredDraft`). Its `area_id` and `seller_id` values carry UUIDs and are mapped to the restored area and seller; one that maps to nothing is left out. Its `type` is mapped onto the group's item types as an item's is: the same slug, the type with the archived name, or a type the restore creates; a kept draft creates no type. Its tags are created in the group if missing, as for an item. In a merge, a draft whose client key the group already has is kept as it is, and the restore log gets a skipped step for it that says whether the group's draft is open, already an item, or being discarded. A draft that fails to restore is a counted error. A link to a draft the walker did not recreate (kept, failed, or not in the archive) is left out without an error, and a file that only such drafts use is restored as a standalone file.

### Delivered documents

`files/_intake_documents.json` (format 3.6) holds the group's record of the documents other systems delivered through the intake endpoint (#2809). The walker recreates each one with new ids and the archived payload hash and timestamps. Its draft and item are mapped to the ones the restore made; a document whose draft was made but which the restore brings back neither as a draft nor as an item gets an empty draft id, so a repeat of the delivery after the restore still makes no draft. In a merge, a document whose identity the group already has is kept as it is. A restore from an archive without the member, as from an older backup, recreates no record, and a full replace then leaves the group without one.

## Data Flow

1. **Request Creation**: User creates a restore operation via API
2. **Record Storage**: Restore operation created with "pending" status
3. **Worker Detection**: `RestoreWorker` polls and detects the pending operation
4. **Processor Creation**: a `RestoreOperationProcessor` is created for the operation
5. **Verify**: for `.inb`, the signature is verified before inflate
6. **Apply**: the inner tar is walked, recreating entities per the selected strategy with step logging
7. **File Restoration**: file members stream into re-minted tenant blob keys
8. **Completion**: restore operation marked completed with final statistics

## Usage

### Creating a Restore Service
```go
import (
    "github.com/denisvmedia/inventario/backup/restore"
    "github.com/denisvmedia/inventario/internal/backupsign"
    "github.com/denisvmedia/inventario/services"
)

// entityService is *services.EntityService; signer is *backupsign.Signer
// (used by the .inb restorer to verify the archive; ignored by the legacy XML
// restorer, so the signature is identical across both builds).
service := restore.NewRestoreService(factorySet, entityService, uploadLocation, signer)
```

### Starting the Restore Worker
```go
// registrySet is *registry.Set; the worker defaults to one concurrent restore.
worker := restore.NewRestoreWorker(restoreService, registrySet, uploadLocation)

// Optional configuration via functional options:
//   restore.WithPollInterval(5 * time.Second)
//   restore.WithMaxConcurrent(1)
//   restore.WithPauseController(pauseController) // #1308 soft-pause

// Start background processing
ctx := context.Background()
worker.Start(ctx)

// Stop when done
defer worker.Stop()
```

### Processing a Restore Operation
```go
// Process restore operation by ID.
err := service.ProcessRestoreOperation(ctx, restoreOperationID, uploadLocation)
if err != nil {
    log.Printf("Restore failed: %v", err)
}
```

### Direct Processor Usage
```go
import "github.com/denisvmedia/inventario/backup/restore/processor"

p := processor.NewRestoreOperationProcessor(
    restoreOperationID, factorySet, entityService, uploadLocation, signer,
)
if err := p.Process(ctx); err != nil {
    log.Printf("Restore failed: %v", err)
}
```

### Restore Options Configuration
```go
options := types.RestoreOptions{
    Strategy:        types.RestoreStrategyMergeUpdate,
    IncludeFileData: true,
    DryRun:          false,
}
```

## Error Handling
- **Graceful Degradation**: most per-item failures are tolerated and counted in `RestoreStats.Errors`
- **Hard Failures**: signature verification failure, framing violations, missing file members (`ErrMissingFileMembers`), oversized JSON members (`ErrJSONDocTooLarge` — including an implausibly large manifest, which must never let an archive skip the version gate), an unsupported format MAJOR (`ErrUnsupportedFormatVersion`), and malformed entity fields (`ErrMalformedEntity`) abort the whole restore
- **Status Tracking**: failed restores marked with a detailed error message

## Performance Considerations

### Memory Efficiency
- **Verify-before-inflate**: the payload is digested via a temp file; nothing is decompressed until the signature is trusted
- **Streaming file members**: file bytes are copied straight into blob storage
- **Bounded walk**: per-member and total caps prevent decompression-bomb DoS

### Concurrency Control
- **Single Restore Default**: one restore operation at a time (`WithMaxConcurrent` to override)
- **Background Processing**: asynchronous processing prevents API timeouts

## Testing

### Test Coverage
- **Unit / Integration Tests**: `service_test.go`, `worker_test.go`, `streaming_test.go`, `restore_files_test.go`, `restore_tags_test.go`, `recursive_delete_integration_test.go`, `status_querier_test.go`
- Uses frankban's quicktest framework
- Memory registries for isolated testing

## Security Considerations

### Data Protection
- **Signature Verification**: `.inb` archives are verified against the server's own key before any inflate
- **Path Sanitization**: inner-tar member names are sanitized (`blobkeys.SanitizeArchivePath`); unsafe names are rejected
- **Key Re-minting**: file bytes are always written to a fresh tenant-namespaced blob key, never the archive path or source key (#1793)
- **Ownership Validation**: commodity ownership is validated against the importing user before persisting

## Integration Points

### API Server Integration
- **REST Endpoints**: `/api/v1/exports/{id}/restores` for restore operations
- **Status / Step Endpoints**: real-time restore status and step progress

### Storage Integration
- **Blob Storage**: file restoration to tenant-namespaced, cloud-agnostic storage
- **Database Storage**: metadata and status persistence
