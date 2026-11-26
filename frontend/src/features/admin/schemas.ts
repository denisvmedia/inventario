import { z } from "zod"

// Limits and patterns of the tenant model (go/models/tenant.go). The BE is
// the authority and still refuses what slips past; checking here saves a
// round trip on a typo.
export const TENANT_NAME_MAX = 100
export const TENANT_SLUG_MAX = 50
export const TENANT_DOMAIN_MAX = 255
const SLUG_PATTERN = /^[a-z0-9-]+$/
// A bare lowercase hostname: no scheme, no port, no trailing dot.
const DOMAIN_PATTERN = /^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$/

// The BE counts the name in characters (code points), not UTF-16 units.
function codePoints(value: string): number {
  return [...value].length
}

const name = z
  .string()
  .trim()
  .min(1, "admin:tenantForm.validation.nameRequired")
  .refine((v) => codePoints(v) <= TENANT_NAME_MAX, "admin:tenantForm.validation.nameTooLong")

const optionalSlug = z
  .string()
  .trim()
  .max(TENANT_SLUG_MAX, "admin:tenantForm.validation.slugTooLong")
  .refine((v) => v === "" || SLUG_PATTERN.test(v), "admin:tenantForm.validation.slugFormat")

const domain = z
  .string()
  .trim()
  .max(TENANT_DOMAIN_MAX, "admin:tenantForm.validation.domainTooLong")
  .refine((v) => v === "" || DOMAIN_PATTERN.test(v), "admin:tenantForm.validation.domainFormat")

// Create: the slug may be left empty, and the BE generates it from the
// name. The domain is optional.
export const createTenantSchema = z.object({ name, slug: optionalSlug, domain })
export type CreateTenantInput = z.infer<typeof createTenantSchema>

// Edit: a tenant always has a slug, so it cannot be cleared. An empty
// domain removes the custom domain.
export const editTenantSchema = z.object({
  name,
  slug: optionalSlug.refine((v) => v !== "", "admin:tenantForm.validation.slugRequired"),
  domain,
})
export type EditTenantInput = z.infer<typeof editTenantSchema>
