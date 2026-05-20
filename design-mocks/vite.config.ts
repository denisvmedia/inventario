import path from "path"
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
    // Allow remote preview of the design mock over Tailscale MagicDNS
    // (*.ts.net) only — keeps Vite's host-check protection on for every
    // other host. Plain IP / localhost access is permitted by Vite by default.
    allowedHosts: [".ts.net"],
  },
})
