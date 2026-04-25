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
