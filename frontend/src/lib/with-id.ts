/**
 * The OpenAPI generator marks `id` optional on every model, because the same
 * schema describes a create payload and a persisted row. Everything the API
 * returns carries one, but the type cannot say so, and `value={x.id ?? ""}`
 * turns that into a runtime problem: Radix throws on a `SelectItem` whose
 * value is the empty string, because it reserves it for clearing the
 * selection.
 *
 * withId narrows instead of papering over. A row without an id is dropped,
 * and what survives has `id: string`, so the call site has nothing left to
 * fall back to.
 */
export function withId<T extends { id?: string }>(
  items: readonly T[] | null | undefined
): (T & { id: string })[] {
  return (items ?? []).filter((item): item is T & { id: string } => Boolean(item.id))
}
