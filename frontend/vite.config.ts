import path from "node:path"
import tailwindcss from "@tailwindcss/vite"
import react from "@vitejs/plugin-react"
import { defineConfig, type Plugin, type ProxyOptions } from "vite"

// `http-proxy` (the lib behind both `server.proxy` and `preview.proxy`) does
// not attach error listeners on the upstream / downstream sockets it spawns.
// Any abnormal close — typical on mobile networks (tailscale handoff, OS
// pausing the tab during a file picker, flaky wifi) — bubbles ECONNRESET up
// as an unhandled `error` event and crashes the Node process. Vite has no
// built-in handler; the canonical workaround is the `configure` hook, which
// runs once per proxy with the underlying `httpProxy` instance, where we
// can hang the listeners ourselves. Reference:
// https://github.com/vitejs/vite/issues/9018
function withResilientProxy(target: string): ProxyOptions {
  return {
    target,
    changeOrigin: true,
    configure: (proxy) => {
      proxy.on("error", (err, _req, res) => {
        // Log with a distinctive prefix so it's easy to grep in long
        // dev-server logs.
        console.warn(`[proxy] upstream ${target} error:`, err.message)
        // The stock behaviour leaves the request hanging on the client
        // side; emit a 502 so fetch() rejects rather than dangling.
        if (res && "writeHead" in res && !res.headersSent) {
          try {
            res.writeHead(502, { "content-type": "text/plain" })
            res.end(`Bad Gateway: ${err.message}`)
          } catch {
            // res may have been torn down already — nothing more to do.
          }
        }
      })
      proxy.on("proxyReq", (proxyReq, req) => {
        // Cancel the upstream request when the client aborts (closed
        // tab, navigated away, mobile network blip). Otherwise the
        // upstream socket lingers and emits the unhandled error a few
        // hundred ms later when its peer goes away.
        req.on("aborted", () => proxyReq.destroy())
      })
    },
  }
}

// Strip the `location.reload()` calls baked into Vite's HMR client
// (`/@vite/client`) so a transient WebSocket drop on a backgrounded mobile
// tab doesn't yank the user back to a fresh page. The client's stock
// behaviour assumes "WS dropped = dev server restarted" and reloads to
// resync; on Android Chrome with the OS file-picker overlay open, the WS
// times out a few seconds in, the picker comes back, and the page reloads
// — destroying the form the user was halfway through.
//
// The plugin only fires when `VITE_DEFANG_RELOAD=true` is set, so dev
// behaviour is unchanged unless we explicitly opt in for mobile-debug.
// Production builds (`vite build`) never run this.
function defangViteClientReload(): Plugin {
  const enabled = process.env.VITE_DEFANG_RELOAD === "true"
  return {
    name: "inventario-defang-vite-reload",
    enforce: "pre",
    apply: "serve",
    transform(code, id) {
      if (!enabled) return
      // The transform runs on resolved module ids; Vite's client lives
      // under `vite/dist/client/client.mjs` (path varies per install).
      if (!id.includes("vite/dist/client/")) return
      // Comment out the reload calls. Two patterns appear in the
      // bundled client today: bare `location.reload()` and
      // `window.location.reload()`. Replace each with a console.warn
      // so the trail stays visible during diagnosis.
      const replaced = code
        .replace(
          /\blocation\.reload\(\)/g,
          'console.warn("[defang] vite client tried to location.reload()")'
        )
        .replace(
          /\bwindow\.location\.reload\(\)/g,
          'console.warn("[defang] vite client tried to window.location.reload()")'
        )
      if (replaced === code) return
      return { code: replaced, map: null }
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), defangViteClientReload()],
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
    // VITE_DISABLE_HMR=true turns the HMR WebSocket off entirely. Use
    // it for mobile dev sessions over tailscale where the picker /
    // OS pause briefly disconnects the WS — Vite's client can react
    // to that by triggering a full `location.reload()`, which on
    // Android looks like "the form just reloaded after I picked a
    // PDF". Production builds (and `vite preview`) don't have this
    // path. See devdocs/frontend/screenshots.md for the binary-based
    // mobile-test alternative.
    hmr: process.env.VITE_DISABLE_HMR === "true" ? false : undefined,
    proxy: {
      // Override with VITE_API_TARGET when the local backend isn't on :3333
      // (e.g. a worktree's docker stack on :3335). Keeps the default
      // behaviour for the canonical "binary on :3333" workflow.
      "/api": withResilientProxy(process.env.VITE_API_TARGET || "http://localhost:3333"),
    },
  },
  // `vite preview` ignores `server.proxy` — it has its own `preview.proxy`
  // namespace. We mirror the same /api → BE forwarding so the production
  // build is testable end-to-end against the same backend the dev server
  // talks to. Useful for chasing reload bugs that only show up against a
  // real network stack (mobile + tailscale + Vite HMR client interactions).
  preview: {
    allowedHosts: process.env.VITE_ALLOWED_HOSTS
      ? process.env.VITE_ALLOWED_HOSTS.split(",")
          .map((h) => h.trim())
          .filter(Boolean)
      : undefined,
    proxy: {
      "/api": withResilientProxy(process.env.VITE_API_TARGET || "http://localhost:3333"),
    },
  },
})
