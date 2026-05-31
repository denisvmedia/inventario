// react-hook-form adapter for the BE's field-level validation errors.
//
// `extractFieldErrors` (in `lib/server-error.ts`) turns a 422 envelope
// into a flat `{ <server-field-path>: <message> }` map. This module maps
// those server paths onto the form's own fields via `setError`, so the
// failing input gets highlighted and its inline error copy renders next
// to it instead of the user only seeing a generic banner.
//
// Server field paths are the BE model's snake_case attribute names
// (`address`, `short_name`, …) plus compound array paths (`urls.0`).
// When a form's react-hook-form field names differ from the BE names
// (e.g. camelCase `defaultGroupId` ↔ `default_group_id`), pass a `map`
// of `{ <server-root-segment>: <form-field-name> }`.
import type { FieldValues, Path, UseFormSetError } from "react-hook-form"

import { extractFieldErrors } from "./server-error"

export interface ServerFieldErrorResult {
  // Form field paths that were successfully set on the form.
  mapped: string[]
  // Server field paths (+ messages) that did NOT match a known form
  // field — the caller should still surface these (banner / toast) so
  // they aren't silently dropped.
  unmapped: Record<string, string>
}

export interface ApplyServerFieldErrorsOptions {
  // The form's own field names (typically the zod schema's keys). Only
  // server errors whose root segment maps to one of these are written to
  // the form; the rest go to `unmapped`.
  fields: readonly string[]
  // Optional `{ <server-root-segment>: <form-field-name> }` overrides for
  // forms whose field names differ from the BE attribute names.
  map?: Record<string, string>
}

// applyServerFieldErrors writes the BE's per-field validation messages
// onto a react-hook-form instance and reports what it could and could
// not place. Returns null when `err` isn't a field-level validation
// envelope at all (network / conflict / 5xx / non-validation 422), so
// the caller can fall straight through to its generic error surface.
export function applyServerFieldErrors<TFieldValues extends FieldValues>(
  err: unknown,
  setError: UseFormSetError<TFieldValues>,
  options: ApplyServerFieldErrorsOptions
): ServerFieldErrorResult | null {
  const raw = extractFieldErrors(err)
  if (!raw) return null

  const known = new Set(options.fields)
  const mapped: string[] = []
  const unmapped: Record<string, string> = {}

  for (const [serverPath, message] of Object.entries(raw)) {
    const root = serverPath.split(".")[0]
    const formRoot = options.map?.[root] ?? root
    if (!known.has(formRoot)) {
      unmapped[serverPath] = message
      continue
    }
    // Swap only the root segment when remapped, preserving any compound
    // suffix (`urls.0` stays `urls.0`; `default_group_id` → `defaultGroupId`).
    const formPath = formRoot + serverPath.slice(root.length)
    setError(formPath as Path<TFieldValues>, { type: "server", message })
    mapped.push(formPath)
  }

  return { mapped, unmapped }
}

// shouldShowGenericError decides whether a caller should ALSO render its
// generic banner/toast after calling `applyServerFieldErrors`. True when
// the error wasn't a field-validation envelope, when nothing mapped to a
// form field, or when some field errors were left unmapped — in all
// three cases the field highlights alone wouldn't tell the full story.
export function shouldShowGenericError(result: ServerFieldErrorResult | null): boolean {
  return !result || result.mapped.length === 0 || Object.keys(result.unmapped).length > 0
}
