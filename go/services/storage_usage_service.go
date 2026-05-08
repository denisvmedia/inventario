package services

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/registry"
)

// DefaultGroupStorageQuotaBytes is the per-group storage budget applied
// to every group while no plan/tier model exists (#1388, #1389). Sized
// at 150 MiB per the issue thread; lifted to a per-group column when
// plans land.
const DefaultGroupStorageQuotaBytes int64 = 150 * 1024 * 1024

// StorageUsage is the wire-compatible payload for GET
// /g/{slug}/storage-usage. QuotaBytes is nullable so v1 can ship
// without enforcement: a non-nil value means the FE should render a
// progress bar and a percentage; nil means "show the absolute number
// only". Today we always return DefaultGroupStorageQuotaBytes — the
// field is nullable for the upcoming plans-aware path.
type StorageUsage struct {
	UsedBytes  int64                     `json:"used_bytes"`
	QuotaBytes *int64                    `json:"quota_bytes"`
	Breakdown  registry.StorageBreakdown `json:"breakdown"`
}

// StorageUsageService aggregates per-group blob byte totals for the
// settings storage card. Today it just reads the SUM(size_bytes)
// breakdown from the file registry (RLS scopes to the current group);
// when plans land, the quota lookup will move here too.
type StorageUsageService struct {
	files registry.FileRegistry
}

// NewStorageUsageService wires the service against an already
// group-scoped FileRegistry — typically the one from the request's
// RegistrySet, so RLS handles tenant + group isolation.
func NewStorageUsageService(files registry.FileRegistry) *StorageUsageService {
	return &StorageUsageService{files: files}
}

// GetUsage returns the storage usage payload for the registry's group.
// The default quota applies to every group until plans land — see the
// constant doc.
func (s *StorageUsageService) GetUsage(ctx context.Context) (StorageUsage, error) {
	breakdown, err := s.files.SumSizeBreakdown(ctx)
	if err != nil {
		return StorageUsage{}, errxtrace.Wrap("failed to compute storage breakdown", err)
	}

	quota := DefaultGroupStorageQuotaBytes
	return StorageUsage{
		UsedBytes:  breakdown.Total(),
		QuotaBytes: &quota,
		Breakdown:  breakdown,
	}, nil
}
