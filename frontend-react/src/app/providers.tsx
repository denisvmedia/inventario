import { useState, type ReactNode } from "react"
import { QueryClientProvider } from "@tanstack/react-query"
import { ReactQueryDevtools } from "@tanstack/react-query-devtools"

import { ThemeProvider } from "@/components/theme-provider"
import { createQueryClient } from "@/lib/query-client"

interface ProvidersProps {
  children: ReactNode
}

// One place that wires every cross-cutting context the app needs. Future
// providers (router, auth, group, i18n, sonner toaster) plug in here so
// individual pages don't have to know the order.
export function Providers({ children }: ProvidersProps) {
  // Lazy init: keep one QueryClient alive across re-renders without re-creating
  // it (which would drop the cache).
  const [queryClient] = useState(createQueryClient)
  return (
    <ThemeProvider defaultTheme="system" storageKey="inventario-theme">
      <QueryClientProvider client={queryClient}>
        {children}
        {import.meta.env.DEV && <ReactQueryDevtools initialIsOpen={false} />}
      </QueryClientProvider>
    </ThemeProvider>
  )
}
