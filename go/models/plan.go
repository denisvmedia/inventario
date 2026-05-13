package models

// Plan is a subscription tier — what the tenant pays for, expressed as a
// bundle of caps + capability gates. Plans are defined in code (not the
// database) in v1: there are three of them, they ship together with the
// binary, and no UI ever edits a row. When billing / operator override
// lands in a follow-up, the source of truth migrates to a `plans` table.
//
// All caps are pointers so `nil` means "no cap" without sentinel values
// like -1 or math.MaxInt. The JSON layer keeps the pointer, so the FE
// can branch on `null` vs. a number.
type Plan struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	MaxItems           *int   `json:"max_items"`
	MaxLocations       *int   `json:"max_locations"`
	MaxStorageBytes    *int64 `json:"max_storage_bytes"`
	MaxGroups          *int   `json:"max_groups"`
	MaxMembersPerGroup *int   `json:"max_members_per_group"`
	MaxExportsPerMonth *int   `json:"max_exports_per_month"`
	AllowsRestore      bool   `json:"allows_restore"`
	AllowsAPIAccess    bool   `json:"allows_api_access"`
}

// PlanFree — entry tier. Caps mirror the design-mock chips (500 items /
// 20 locations / 1 GiB), with a small extras-per-month cap and no
// restore / API. Numbers can move freely; they are not load-bearing for
// the surrounding code — only the BE enforcement layer (#1389 follow-up)
// will cite them at request time.
//
// PlanPro — paid tier. Generous caps with a soft storage limit (50 GiB);
// everything else is uncapped. Restore + API access flipped on.
//
// PlanUnlimited — operator override / self-hoster default. No caps at
// all; the `tenants.plan_id` default points here so a fresh install
// behaves like the pre-#1389 binary.
//
// Cap pointers are populated in init(): Go has no inline `&500`-style
// syntax, and helper-wrappers trip the modernize.newexpr analyzer with
// no clean per-callsite suppression. An init() touches every field
// exactly once at startup and stays out of the lint surface.
var (
	PlanFree = Plan{
		ID:            "free",
		Name:          "Free",
		AllowsRestore: false, AllowsAPIAccess: false,
	}
	PlanPro = Plan{
		ID:            "pro",
		Name:          "Pro",
		AllowsRestore: true, AllowsAPIAccess: true,
	}
	PlanUnlimited = Plan{
		ID:            "unlimited",
		Name:          "Unlimited",
		AllowsRestore: true, AllowsAPIAccess: true,
	}
)

// The plan catalogue's pointer caps can't be expressed as literal
// addresses in Go (`&500` is not a thing); an init() is the cleanest
// way to populate them without per-callsite lint suppressions or ugly
// slice-index workarounds.
//
//nolint:gochecknoinits // see comment above
func init() {
	free500 := 500
	free20 := 20
	free1GiB := int64(1) << 30
	free1 := 1
	free3 := 3
	free5 := 5
	PlanFree.MaxItems = &free500
	PlanFree.MaxLocations = &free20
	PlanFree.MaxStorageBytes = &free1GiB
	PlanFree.MaxGroups = &free1
	PlanFree.MaxMembersPerGroup = &free3
	PlanFree.MaxExportsPerMonth = &free5

	pro50GiB := int64(50) << 30
	PlanPro.MaxStorageBytes = &pro50GiB
}

// Plans returns the catalog in display order. Order matches the natural
// upgrade path (free → pro → unlimited) — only the FE catalogue page
// (#1389 follow-up) reads this; nothing else should iterate.
func Plans() []Plan {
	return []Plan{PlanFree, PlanPro, PlanUnlimited}
}

// PlanByID resolves a plan ID to its definition. Unknown IDs (e.g. a
// row patched by an admin to a value not in this table) degrade to the
// `unlimited` plan rather than returning an error: every request path
// that reads a plan would otherwise need to handle a "plan vanished"
// case at runtime. Caller can compare `got.ID == id` to detect the
// fallback if they care.
func PlanByID(id string) Plan {
	switch id {
	case PlanFree.ID:
		return PlanFree
	case PlanPro.ID:
		return PlanPro
	case PlanUnlimited.ID:
		return PlanUnlimited
	default:
		return PlanUnlimited
	}
}

// PlanUsage is the current consumption of a plan-gated resource bundle
// for a single group. Fields use plain types (not pointers) because
// usage is always known — a 0 means zero, not "unknown". When the BE
// can't compute a number cheaply (cross-table sum), it returns 0 and
// the FE renders "—" if that ever becomes a problem in practice.
type PlanUsage struct {
	Items        int   `json:"items"`
	Locations    int   `json:"locations"`
	StorageBytes int64 `json:"storage_bytes"`
}

// GroupPlanResult bundles what the Plan & quota card needs in one shot:
// the active plan + the current group's usage. Returned from the
// /g/{groupSlug}/plan handler.
type GroupPlanResult struct {
	Plan  Plan      `json:"plan"`
	Usage PlanUsage `json:"usage"`
}
