/**
 * Domain pattern composites built on top of `@design/ui/*` primitives.
 *
 * Patterns are added in Phase 2 (#1327) of the design system migration
 * (Epic #1324). They are the toolbox views consume in Phases 3+; in
 * Phase 2 nothing under `views/` imports from here yet.
 */

export { default as FormSection, formSectionVariants } from "./FormSection.vue"
export type { FormSectionVariants } from "./FormSection.vue"

export { default as FormGrid, formGridVariants } from "./FormGrid.vue"
export type { FormGridVariants } from "./FormGrid.vue"

export { default as FormFooter, formFooterVariants } from "./FormFooter.vue"
export type { FormFooterVariants } from "./FormFooter.vue"

export { default as InlineListEditor } from "./InlineListEditor.vue"

export { default as LocationCard } from "./LocationCard.vue"

export { default as CommodityCard } from "./CommodityCard.vue"

export { default as FilePreview } from "./FilePreview.vue"

export { default as MediaGallery, mediaGalleryVariants } from "./MediaGallery.vue"
export type { MediaGalleryVariants } from "./MediaGallery.vue"

export { default as FileViewerDialog } from "./FileViewerDialog.vue"

export { default as FileUploader } from "./FileUploader.vue"

export { default as FileGallery } from "./FileGallery.vue"
export type { FileGallerySignedUrlData, FileGallerySignedUrls } from "./fileGalleryTypes"

export { default as StatCard, statCardVariants } from "./StatCard.vue"
export type { StatCardVariants } from "./StatCard.vue"

export { default as ValueByGroupingCard } from "./ValueByGroupingCard.vue"
export type { ValueByGroupingItem } from "./ValueByGroupingCard.vue"

export { default as CommandPalette } from "./CommandPalette.vue"

export { default as SearchInput } from "./SearchInput.vue"

export { default as FilterBar, filterBarVariants } from "./FilterBar.vue"
export type { FilterBarVariants } from "./FilterBar.vue"

export {
  default as CommodityStatusPill,
  commodityStatusPillVariants,
  COMMODITY_STATUSES,
  COMMODITY_STATUS_LABELS,
} from "./CommodityStatusPill.vue"
export type {
  CommodityStatus,
  CommodityStatusPillVariants,
} from "./CommodityStatusPill.vue"

export {
  default as ExportStatusPill,
  exportStatusPillVariants,
  EXPORT_STATUSES,
  EXPORT_STATUS_LABELS,
} from "./ExportStatusPill.vue"
export type {
  ExportStatus,
  ExportStatusPillVariants,
} from "./ExportStatusPill.vue"

/**
 * Migration alias — `StatusBadge` is the legacy name some callers
 * may use during the Phase 3/4 strangler-fig window. It points at
 * `CommodityStatusPill`; remove once no view imports it under this
 * name (tracked under Phase 6 #1331).
 */
export { default as StatusBadge } from "./CommodityStatusPill.vue"

// AreaCard added in Phase 4 (#1329) for the location detail view.
export { default as AreaCard } from "./AreaCard.vue"
