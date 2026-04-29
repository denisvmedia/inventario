import { useState, type ReactNode } from "react"
import { QueryClientProvider } from "@tanstack/react-query"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"
import { BrowserRouter } from "react-router-dom"

import { ThemeProvider } from "@/components/theme-provider"
import { AuthProvider } from "@/features/auth/AuthContext"
import { createQueryClient } from "@/lib/query-client"

interface ProvidersProps {
  children: ReactNode
}

// One place that wires every cross-cutting context the app needs.
// Provider order matters: BrowserRouter wraps AuthProvider so AuthProvider
// can use react-router's useNavigate to install the http client's
// router-aware redirect. AuthProvider in turn wraps the routes so every
// protected page has a non-empty <AuthContext>.
export function Providers({ children }: ProvidersProps) {
  // Lazy init: keep one QueryClient alive across re-renders without re-creating
  // it (which would drop the cache).
  const [queryClient] = useState(createQueryClient)
  return (
    <ThemeProvider defaultTheme="system" storageKey="inventario-theme">
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <AuthProvider>{children}</AuthProvider>
        </BrowserRouter>
        {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
      </QueryClientProvider>
    </ThemeProvider>
  )
}
