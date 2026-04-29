import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"

export type Density = "comfortable" | "cozy" | "compact"
export const DENSITIES: Density[] = ["comfortable", "cozy", "compact"]

interface DensityContextValue {
  density: Density
  setDensity: (next: Density) => void
}

const DensityContext = createContext<DensityContextValue | undefined>(undefined)

interface DensityProviderProps {
  children: ReactNode
  defaultDensity?: Density
  storageKey?: string
}

function isDensity(value: string | null): value is Density {
  return value !== null && (DENSITIES as string[]).includes(value)
}

// DensityProvider mirrors the chosen density into a `data-density` attribute
// on <html> so the value is reachable from CSS via [data-density="cozy"]
// without React having to thread it through every consumer. We persist to
// localStorage with the same idempotency contract as the theme provider:
// rehydrate on mount, listen for cross-tab changes via the `storage` event,
// and write through `setDensity` so the attribute and the storage stay in
// sync.
//
// The server-side density preference (#1414 Settings page) writes through
// PATCH /settings/{field} and then mirrors into localStorage for the boot
// path; that direction is added when the Settings page lands.
export function DensityProvider({
  children,
  defaultDensity = "comfortable",
  storageKey = "inventario-density",
}: DensityProviderProps) {
  const [density, setDensityState] = useState<Density>(() => {
    if (typeof window === "undefined") return defaultDensity
    const stored = window.localStorage.getItem(storageKey)
    return isDensity(stored) ? stored : defaultDensity
  })

  useEffect(() => {
    const root = window.document.documentElement
    root.setAttribute("data-density", density)
  }, [density])

  // Cross-tab sync — same shape as the theme provider's storage listener.
  useEffect(() => {
    const onStorage = (event: StorageEvent) => {
      if (event.storageArea !== window.localStorage) return
      if (event.key !== storageKey) return
      if (isDensity(event.newValue)) {
        setDensityState(event.newValue)
        return
      }
      setDensityState(defaultDensity)
    }
    window.addEventListener("storage", onStorage)
    return () => window.removeEventListener("storage", onStorage)
  }, [storageKey, defaultDensity])

  const setDensity = useCallback(
    (next: Density) => {
      window.localStorage.setItem(storageKey, next)
      setDensityState(next)
    },
    [storageKey]
  )

  const value = useMemo(() => ({ density, setDensity }), [density, setDensity])
  return <DensityContext.Provider value={value}>{children}</DensityContext.Provider>
}

export function useDensity(): DensityContextValue {
  const ctx = useContext(DensityContext)
  if (!ctx) throw new Error("useDensity must be used within a DensityProvider")
  return ctx
}
