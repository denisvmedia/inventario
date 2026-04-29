import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import { App } from "@/app/App"
import { Providers } from "@/app/providers"
import { initI18n } from "@/i18n"

const rootElement = document.getElementById("root")
if (!rootElement) {
  throw new Error("Root element #root not found in index.html")
}

// Boot i18n before the first render so useTranslation() returns real strings
// on the very first paint rather than the raw keys. The en bundle is in
// memory already (statically imported), so the wait is one microtask.
void initI18n().then(() => {
  createRoot(rootElement).render(
    <StrictMode>
      <Providers>
        <App />
      </Providers>
    </StrictMode>
  )
})
