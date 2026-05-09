import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vite"

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    // VITE_ALLOWED_HOSTS lets a remote dev session (tailscale, ngrok, etc.)
    // open the dev URL by hostname without tripping vite's host-check.
    // Comma-separated list; empty/unset keeps the strict default.
    allowedHosts: process.env.VITE_ALLOWED_HOSTS
      ? process.env.VITE_ALLOWED_HOSTS.split(",")
          .map((h) => h.trim())
          .filter(Boolean)
      : undefined,
    proxy: {
      "/api": {
        // Override with VITE_API_TARGET when the local backend isn't on :3333
        // (e.g. a worktree's docker stack on :3335). Keeps the default
        // behaviour for the canonical "binary on :3333" workflow.
        target: process.env.VITE_API_TARGET || "http://localhost:3333",
        changeOrigin: true,
      },
    },
  },
})
