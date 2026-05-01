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

function mount(): void {
  createRoot(rootElement!).render(
    <StrictMode>
      <Providers>
        <App />
      </Providers>
    </StrictMode>
  )
}

// Boot i18n before the first render so useTranslation() returns real strings
// on the very first paint rather than the raw keys. The en bundle is in
// memory already (statically imported), so the wait is one microtask.
//
// If init rejects (network down for a lazy chunk, malformed JSON, etc.) we
// still mount the app rather than leaving the user staring at a blank page —
// i18next will fall back to rendering raw keys, which is ugly but at least
// puts the rest of the UI on screen so the user can navigate to /login or
// retry.
initI18n()
  .catch((err) => {
    console.error("[i18n] init failed; rendering with default i18next state", err)
  })
  .finally(mount)
