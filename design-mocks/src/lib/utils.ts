import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

// `crypto.randomUUID` is undefined outside a secure context (HTTPS or
// `localhost` / `127.0.0.1`). Plain-HTTP dev hosts (e.g. tailscale
// hostnames) hit the unbounded form and crash on mount. The fallback
// yields enough entropy for in-memory list keys — these ids never
// leave the client.
export function makeId(): string {
  if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") {
    return crypto.randomUUID()
  }
  return `id-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}
