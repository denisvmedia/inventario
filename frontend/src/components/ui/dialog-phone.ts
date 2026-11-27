import * as React from "react"

// What the "fullscreen-mobile" dialog does on a phone, below `sm`, beyond its
// layout: it stays above the on-screen keyboard, the back gesture closes it,
// and toasts keep clear of its footer.

// Tailwind's `max-sm`. The full-screen layout applies below it.
const BELOW_SM = "(width < 40rem)"

/** True below `sm` (640px), where a "fullscreen-mobile" dialog fills the screen. */
export function useBelowSm(): boolean {
  // One query list per mount: DialogContent renders on every render of its
  // parent, open or not, and a MediaQueryList keeps `matches` current.
  const [query] = React.useState(() => window.matchMedia(BELOW_SM))
  const subscribe = React.useCallback(
    (onChange: () => void) => {
      query.addEventListener("change", onChange)
      return () => query.removeEventListener("change", onChange)
    },
    [query]
  )
  const getSnapshot = React.useCallback(() => query.matches, [query])
  return React.useSyncExternalStore(subscribe, getSnapshot, () => false)
}

/**
 * The part of the screen the on-screen keyboard leaves visible, in CSS
 * pixels from the top of the layout viewport: `top` and `height` of the
 * visible area, and `bottom`, the height the keyboard covers.
 */
export interface VisibleArea {
  top: number
  height: number
  bottom: number
}

function subscribeVisualViewport(onChange: () => void) {
  const viewport = window.visualViewport
  if (!viewport) return () => {}
  viewport.addEventListener("resize", onChange)
  viewport.addEventListener("scroll", onChange)
  return () => {
    viewport.removeEventListener("resize", onChange)
    viewport.removeEventListener("scroll", onChange)
  }
}

// A string, so useSyncExternalStore compares snapshots by value. Empty when
// nothing covers the screen, and when the page is zoomed: a pinch zoom also
// shrinks the visual viewport, and a dialog that followed it would undo the
// zoom.
function readVisibleArea(): string {
  const viewport = window.visualViewport
  if (!viewport || Math.abs(viewport.scale - 1) > 0.01) return ""
  const top = Math.max(0, Math.round(viewport.offsetTop))
  const height = Math.round(viewport.height)
  const bottom = Math.max(0, Math.round(window.innerHeight - viewport.offsetTop - viewport.height))
  if (top === 0 && bottom === 0) return ""
  return `${top} ${height} ${bottom}`
}

/**
 * The visible area while the on-screen keyboard is open, or null when the
 * whole layout viewport is visible. The layout viewport keeps its height
 * when the keyboard opens (iOS Safari, and Chrome on Android from 108), so
 * `100dvh` and fixed elements end up under the keyboard; the visual
 * viewport shrinks.
 */
export function useKeyboardVisibleArea(enabled: boolean): VisibleArea | null {
  const subscribe = React.useCallback(
    (onChange: () => void) => (enabled ? subscribeVisualViewport(onChange) : () => {}),
    [enabled]
  )
  const getSnapshot = React.useCallback(() => (enabled ? readVisibleArea() : ""), [enabled])
  const snapshot = React.useSyncExternalStore(subscribe, getSnapshot, () => "")
  return React.useMemo(() => {
    if (!snapshot) return null
    const [top, height, bottom] = snapshot.split(" ").map(Number)
    return { top, height, bottom }
  }, [snapshot])
}

// The history entries an open dialog pushed, kept in `history.state` next to
// the router's own fields. A list, so a dialog opened over another one can
// tell whether the step back was its own.
const DIALOG_STACK_KEY = "inventarioDialogs"

// Dialogs with an entry in this document's history. The prefix keeps an id
// left in `history.state` from before a reload from matching a new dialog.
const liveDialogs = new Set<string>()
const DOCUMENT_PREFIX = Math.random().toString(36).slice(2, 8)

const realPushState = History.prototype.pushState
const realReplaceState = History.prototype.replaceState

function asRecord(state: unknown): Record<string, unknown> {
  return state && typeof state === "object" ? { ...(state as Record<string, unknown>) } : {}
}

function dialogStack(state: unknown): string[] {
  const stack = asRecord(state)[DIALOG_STACK_KEY]
  return Array.isArray(stack)
    ? stack.filter((entry): entry is string => typeof entry === "string" && liveDialogs.has(entry))
    : []
}

function withStack(state: unknown, stack: string[]): Record<string, unknown> {
  const next = asRecord(state)
  if (stack.length > 0) next[DIALOG_STACK_KEY] = stack
  else delete next[DIALOG_STACK_KEY]
  return next
}

interface HistoryEntry {
  state: unknown
  url: string
}

// Set while the dialog steps back over its own entry. The step is the
// dialog's, not the user's, so the router never hears of it. The entry below
// takes the URL and state the dialog's entry had by then, and the entries the
// app pushed meanwhile are pushed after it.
let pendingStepBack: { below: HistoryEntry; pushed: HistoryEntry[]; since: number } | null = null

// A step back that has not landed by then is taken as lost, so the history
// does not stay in this mode.
const STEP_BACK_TIMEOUT_MS = 3000

function stepBackUnderWay() {
  if (pendingStepBack && performance.now() - pendingStepBack.since > STEP_BACK_TIMEOUT_MS) {
    pendingStepBack = null
  }
  return pendingStepBack
}

function onPopStateCapture(event: PopStateEvent) {
  const step = stepBackUnderWay()
  if (!step) return
  pendingStepBack = null
  event.stopImmediatePropagation()
  realReplaceState.call(window.history, step.below.state, "", step.below.url)
  for (const entry of step.pushed) realPushState.call(window.history, entry.state, "", entry.url)
}

// While the dialog steps back, a push must not reach the browser: a browser
// may drop the step back for it, or step back over it. The entry the dialog is
// leaving takes the new URL and state at once, so the router reads them, and
// the entry is pushed once the step back lands.
function pushEntry(data: unknown, unused: string, url?: string | URL | null) {
  const step = stepBackUnderWay()
  if (!step) {
    realPushState.call(window.history, data, unused, url)
    return
  }
  realReplaceState.call(window.history, data, unused, url)
  step.pushed.push({ state: data, url: window.location.href })
}

// While a dialog's entry is the current one, a navigation of the app applies
// as if the dialog had taken the entry off first. A new page takes the
// entry's place, so closing a dialog and opening a page in the same handler
// leaves no extra entry behind. A replaced URL or state keeps the entry, and
// the entry below gets it when the dialog steps back.
let historyPatched = false
function patchHistory() {
  if (historyPatched) return
  historyPatched = true
  const history = window.history
  history.pushState = function pushState(data, unused, url) {
    if (!stepBackUnderWay() && dialogStack(history.state).length > 0) {
      realReplaceState.call(history, data, unused, url)
    } else {
      pushEntry(data, unused, url)
    }
  }
  history.replaceState = function replaceState(data, unused, url) {
    const step = stepBackUnderWay()
    if (step) {
      // The step back is under way: the entry that lands takes this state too.
      realReplaceState.call(history, data, unused, url)
      const entry = { state: data, url: window.location.href }
      if (step.pushed.length > 0) step.pushed[step.pushed.length - 1] = entry
      else step.below = entry
      return
    }
    const stack = dialogStack(history.state)
    if (stack.length > 0 && dialogStack(data).length === 0) data = withStack(data, stack)
    realReplaceState.call(history, data, unused, url)
  }
  window.addEventListener("popstate", onPopStateCapture, true)
}

function pushDialogEntry(id: string) {
  patchHistory()
  liveDialogs.add(id)
  const state: unknown = window.history.state
  const stack = dialogStack(state).filter((entry) => entry !== id)
  // The same URL and the router's fields (`key`, `idx`), so the router sees
  // the same location when the entry is popped.
  pushEntry(withStack(state, [...stack, id]), "")
}

function removeDialogEntry(id: string) {
  const stack = dialogStack(window.history.state)
  liveDialogs.delete(id)
  // One step back at a time. During another one the entry stays; with the
  // dialog gone its marker no longer counts.
  if (stack[stack.length - 1] !== id || stepBackUnderWay()) return
  pendingStepBack = {
    below: {
      state: withStack(window.history.state, stack.slice(0, -1)),
      url: window.location.href,
    },
    pushed: [],
    since: performance.now(),
  }
  window.history.back()
}

// The entry comes off one task after the dialog closes, and a dialog that
// opens again before then (React's StrictMode runs effects twice) keeps it.
const pendingRemovals = new Map<string, number>()

function scheduleRemoval(id: string) {
  pendingRemovals.set(
    id,
    window.setTimeout(() => {
      pendingRemovals.delete(id)
      removeDialogEntry(id)
    }, 0)
  )
}

function cancelRemoval(id: string) {
  const timer = pendingRemovals.get(id)
  if (timer === undefined) return false
  window.clearTimeout(timer)
  pendingRemovals.delete(id)
  return true
}

/**
 * While `enabled`, a step back in the browser history (the Android back
 * gesture, the iOS edge swipe, the Back button) calls `close` instead of
 * leaving the page: the dialog pushes a history entry with the same URL and
 * closes itself when that entry is popped. When the dialog stays open (the
 * close asked about unsaved changes and the user kept editing), the entry is
 * pushed again, so the next step back asks again. Closing the dialog another
 * way takes its entry off; a page the app opens on close takes its place.
 */
export function useCloseOnHistoryBack(enabled: boolean, close: () => void) {
  const reactId = React.useId()
  const id = `${DOCUMENT_PREFIX}${reactId}`
  const closeRef = React.useRef(close)
  React.useEffect(() => {
    closeRef.current = close
  })
  // Bumped on each step back over this dialog's entry; when the dialog is
  // still open after it, the effect runs again and pushes a new entry.
  const [backs, setBacks] = React.useState(0)

  React.useEffect(() => {
    if (!enabled) return
    const kept = cancelRemoval(id) && dialogStack(window.history.state).at(-1) === id
    if (!kept) pushDialogEntry(id)
    function onPopState() {
      if (dialogStack(window.history.state).includes(id)) return
      setBacks((n) => n + 1)
      closeRef.current()
    }
    window.addEventListener("popstate", onPopState)
    return () => {
      window.removeEventListener("popstate", onPopState)
      scheduleRemoval(id)
    }
  }, [enabled, id, backs])
}

// The CSS property the toaster adds to its bottom offset, set on <html>.
export const TOAST_BOTTOM_INSET_PROPERTY = "--fullscreen-dialog-toast-inset"

// One inset per open dialog, in the order they opened; the last one wins.
const toastInsets = new Map<string, number>()

function applyToastInset() {
  const root = document.documentElement
  const insets = [...toastInsets.values()]
  if (insets.length === 0) root.style.removeProperty(TOAST_BOTTOM_INSET_PROPERTY)
  else root.style.setProperty(TOAST_BOTTOM_INSET_PROPERTY, `${insets[insets.length - 1]}px`)
}

/**
 * While `enabled`, toasts keep `footer`'s height plus `keyboard` pixels clear
 * at the bottom of the screen, so they do not cover the dialog's footer
 * buttons.
 */
export function useToastBottomInset(
  enabled: boolean,
  footer: HTMLElement | null,
  keyboard: number
) {
  const id = React.useId()
  const [footerHeight, setFooterHeight] = React.useState(0)

  React.useEffect(() => {
    if (!enabled || !footer) return
    // ResizeObserver reports the first size when it starts observing.
    const observer = new ResizeObserver(() => setFooterHeight(footer.offsetHeight))
    observer.observe(footer)
    return () => observer.disconnect()
  }, [enabled, footer])

  const inset = footer ? footerHeight + keyboard : keyboard
  React.useEffect(() => {
    if (!enabled) return
    toastInsets.set(id, inset)
    applyToastInset()
    return () => {
      toastInsets.delete(id)
      applyToastInset()
    }
  }, [enabled, id, inset])
}
