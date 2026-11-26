import { useCallback, useEffect, useRef } from "react"
import { useLocation } from "react-router-dom"

/**
 * Guards a navigation that follows async work. Call the returned function
 * when the work starts; the check it returns reports whether the user is
 * still on that location, so a late answer does not take the user away from
 * where they went in the meantime. The check is false once the component
 * that started the work is gone.
 */
export function useLocationGuard(): () => () => boolean {
  const location = useLocation()
  const current = useRef<string | null>(location.key)
  useEffect(() => {
    current.current = location.key
    return () => {
      current.current = null
    }
  }, [location.key])
  return useCallback(() => {
    const started = location.key
    return () => current.current === started
  }, [location.key])
}
