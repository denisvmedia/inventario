import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import { App } from "@/app/App"
import { ThemeProvider } from "@/components/theme-provider"

const rootElement = document.getElementById("root")
if (!rootElement) {
  throw new Error("Root element #root not found in index.html")
}

createRoot(rootElement).render(
  <StrictMode>
    <ThemeProvider defaultTheme="system" storageKey="inventario-theme">
      <App />
    </ThemeProvider>
  </StrictMode>
)
