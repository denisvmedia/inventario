import { createContext, useContext, useEffect, useState, type ReactNode } from "react"

type Theme = "dark" | "light" | "system"

interface ThemeProviderProps {
  children: ReactNode
  defaultTheme?: Theme
  storageKey?: string
}

interface ThemeProviderState {
  theme: Theme
  setTheme: (theme: Theme) => void
}

const ThemeProviderContext = createContext<ThemeProviderState | undefined>(undefined)

// Tiny custom theme hook intentionally replacing `next-themes`. The mock
// uses next-themes, but we don't want a dep that pretends to be Next.js in
// a Vite SPA. Behavior matches: persists choice in localStorage, resolves
// "system" against prefers-color-scheme, toggles `.dark` on <html>.
export function ThemeProvider({
  children,
  defaultTheme = "system",
  storageKey = "inventario-theme",
}: ThemeProviderProps) {
  const [theme, setThemeState] = useState<Theme>(() => {
    if (typeof window === "undefined") return defaultTheme
    const stored = window.localStorage.getItem(storageKey) as Theme | null
    return stored ?? defaultTheme
  })

  useEffect(() => {
    const root = window.document.documentElement

    const apply = (resolved: "dark" | "light") => {
      root.classList.remove("light", "dark")
      root.classList.add(resolved)
    }

    if (theme !== "system") {
      apply(theme)
      return
    }

    const mql = window.matchMedia("(prefers-color-scheme: dark)")
    apply(mql.matches ? "dark" : "light")

    const onChange = (event: MediaQueryListEvent) => apply(event.matches ? "dark" : "light")
    mql.addEventListener("change", onChange)
    return () => mql.removeEventListener("change", onChange)
  }, [theme])

  const setTheme = (next: Theme) => {
    window.localStorage.setItem(storageKey, next)
    setThemeState(next)
  }

  return (
    <ThemeProviderContext.Provider value={{ theme, setTheme }}>
      {children}
    </ThemeProviderContext.Provider>
  )
}

export function useTheme() {
  const ctx = useContext(ThemeProviderContext)
  if (!ctx) throw new Error("useTheme must be used within a ThemeProvider")
  return ctx
}
