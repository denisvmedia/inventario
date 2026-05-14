// Shared UA-parsing helper for the Privacy & Security surfaces — used by
// SessionsPage and LoginHistoryPage. The parser is intentionally
// minimal: we don't pull in ua-parser-js or similar because the UI
// only needs a "Chrome on macOS" / "Safari on iOS" label, and a real
// parser ages faster than the regex table here (new device shapes show
// up in UA strings every quarter).
//
// The returned shape is i18n-free on purpose — callers pair the
// `browser` / `os` strings with translated "Unknown browser" / "Unknown
// OS" fallbacks at render time. That lets the parser stay testable
// without a TFunction, and avoids the string-sentinel problem
// (comparing against a localized label) flagged in #1674 review.

import { Laptop, Monitor, Smartphone, TabletSmartphone } from "lucide-react"

// IconComponent is the structural type lucide-react exports for every
// icon — keeping it loose so we don't pin to a specific re-export.
type IconComponent = typeof Laptop

export interface ParsedUA {
  /** Device-shape icon for the card; Monitor when the UA is empty. */
  deviceIcon: IconComponent
  /** Detected browser family (Chrome / Edge / Safari / …) or null. */
  browser: string | null
  /** Detected OS family (Windows / macOS / iOS / …) or null. */
  os: string | null
  /** True when neither browser nor OS could be derived. */
  isUnknown: boolean
}

const BROWSERS: Array<[RegExp, string]> = [
  // Order matters — Edge / Opera identify themselves *and* contain the
  // Chrome token, so they have to win their respective tests first.
  [/Edg\/\d+/, "Edge"],
  [/OPR\/\d+/, "Opera"],
  [/Chrome\/\d+/, "Chrome"],
  [/Safari\/\d+/, "Safari"],
  [/Firefox\/\d+/, "Firefox"],
]

const OPERATING_SYSTEMS: Array<[RegExp, string]> = [
  // Order matters: iPhone / iPad UA strings carry the substring
  // "like Mac OS X" for legacy WebKit compatibility, so `iPhone OS` /
  // `iPad` need to match *before* the Mac OS X line — otherwise an
  // iPhone is reported as macOS.
  [/Windows NT/, "Windows"],
  [/iPhone OS|iPad/, "iOS"],
  [/Mac OS X/, "macOS"],
  [/Android/, "Android"],
  [/Linux/, "Linux"],
]

/**
 * parseUserAgent runs in the browser per #1378 option 2 — keeps the DB
 * free of UA strings that age poorly. The returned shape is i18n-free;
 * callers are responsible for localizing "Unknown" fallbacks.
 */
export function parseUserAgent(ua: string): ParsedUA {
  if (!ua) return { deviceIcon: Monitor, browser: null, os: null, isUnknown: true }
  const isMobile = /iPhone|Android.*Mobile|Mobile/i.test(ua)
  const isTablet = /iPad|Android(?!.*Mobile)/i.test(ua)
  const browser = matchFirst(ua, BROWSERS)
  const os = matchFirst(ua, OPERATING_SYSTEMS)
  let deviceIcon: IconComponent = Laptop
  if (isMobile) deviceIcon = Smartphone
  else if (isTablet) deviceIcon = TabletSmartphone
  return { deviceIcon, browser, os, isUnknown: !browser && !os }
}

function matchFirst(ua: string, table: Array<[RegExp, string]>): string | null {
  for (const [re, label] of table) {
    if (re.test(ua)) return label
  }
  return null
}
