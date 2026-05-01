// Centralised pdfjs-dist setup. The worker URL must be configured once
// at module load — repeating it per-component churns the worker
// connection. Mirrors the legacy `frontend/src/utils/pdfjs-init.ts`.
import * as pdfjsLib from "pdfjs-dist"
// `?url` (Vite asset import) gives us the bundler-fingerprinted path
// to the worker file so it's served from the same origin and survives
// chunk hashing.
import pdfWorkerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url"

pdfjsLib.GlobalWorkerOptions.workerSrc = pdfWorkerUrl

export { pdfjsLib }
