import { useEffect, useState } from "react"

// useDebouncedValue trails an input value, only updating the returned
// value once `delayMs` has elapsed without further changes. Use it to
// keep an as-you-type input responsive while throttling whatever it
// drives — e.g. a server-side search query. 250ms is the delay other
// "as-you-type" surfaces in this codebase use (see TagsListPage).
export function useDebouncedValue<T>(value: T, delayMs = 250): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const id = window.setTimeout(() => setDebounced(value), delayMs)
    return () => window.clearTimeout(id)
  }, [value, delayMs])

  return debounced
}
