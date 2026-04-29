import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import { App } from "@/app/App"
import { Providers } from "@/app/providers"

const rootElement = document.getElementById("root")
if (!rootElement) {
  throw new Error("Root element #root not found in index.html")
}

createRoot(rootElement).render(
  <StrictMode>
    <Providers>
      <App />
    </Providers>
  </StrictMode>
)
