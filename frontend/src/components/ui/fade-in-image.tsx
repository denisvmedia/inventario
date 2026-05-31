import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type ReactNode,
} from "react"

import { cn } from "@/lib/utils"

// Load state of the underlying <img>. `loading` until the browser
// reports the first load/error for the current `src`; `error` lets the
// caller distinguish a broken image from a decoded one.
type FadeStatus = "loading" | "loaded" | "error"

// useImageFadeIn tracks the load state of a single <img> and resets it
// whenever `src` changes, so a recycled element — gallery navigation, or
// a cover that swaps to its real thumbnail once generation finishes
// (#1961) — re-runs the fade instead of staying stuck on the previous
// image's "loaded" state.
//
// It also reconciles the browser cache: an image already in cache can
// fire its `load` event before React attaches the `onLoad` handler, so
// the post-paint effect promotes a `complete` element to `loaded`. That
// keeps a cache hit from being stranded at opacity-0 (load fired, nobody
// listened) while still painting one frame at opacity-0 first so the
// fade actually plays.
//
// The hook is the shared core behind <FadeInImage>; the fullscreen
// viewer consumes it directly because its <img> carries an inline
// transform-transition that a className-based fade can't compose with.
export function useImageFadeIn(src: string | undefined) {
  const ref = useRef<HTMLImageElement>(null)
  const [status, setStatus] = useState<FadeStatus>("loading")

  useEffect(() => {
    const img = ref.current
    // Reconcile a cache hit (the load event may have fired before this
    // handler was wired) and otherwise reset to `loading` for the new
    // src. Reading the external DOM property (<img>.complete) via the ref
    // is the sanctioned way to sync it into state.
    setStatus(img?.complete && img.naturalWidth > 0 ? "loaded" : "loading")
  }, [src])

  const onLoad = useCallback(() => setStatus("loaded"), [])
  const onError = useCallback(() => setStatus("error"), [])

  return { ref, status, onLoad, onError }
}

export interface FadeInImageProps extends ComponentProps<"img"> {
  // Extra classes for the default muted-shimmer placeholder. Ignored
  // when `placeholder` is supplied.
  placeholderClassName?: string
  // Overrides the default placeholder, rendered only while the image is
  // still loading. Pass `null` to render no placeholder at all — the
  // caller owns the backdrop then (e.g. a dark fullscreen surface where
  // a muted box reads wrong).
  placeholder?: ReactNode
}

// FadeInImage eases an image in once it decodes instead of letting it
// snap into place (issue #1961). A neutral shimmer fills the box until
// the image is ready, then the <img> fades from opacity-0 to
// opacity-100. The fade is pure CSS and is skipped under
// `prefers-reduced-motion` via the `motion-reduce` variant; the shimmer
// likewise only animates under `motion-safe`. `decoding="async"` keeps
// the decode off the paint path.
//
// Layout contract: the placeholder fills the nearest positioned ancestor
// (`absolute inset-0`), so the wrapping element must establish a
// positioning context (`relative`) and reserve space (aspect-ratio or a
// fixed size) to keep CLS at zero. Give that wrapper a muted background
// (`bg-muted`) so the brief gap between the shimmer unmounting and the
// fade completing reads as the same neutral tone rather than a flash.
// The placeholder is phrasing content (a <span>), so it stays valid
// inside the <button> wrappers these grids use.
export function FadeInImage({
  className,
  placeholderClassName,
  placeholder,
  src,
  alt,
  onLoad,
  onError,
  ...rest
}: FadeInImageProps) {
  const { ref, status, onLoad: markLoaded, onError: markError } = useImageFadeIn(src)
  const isLoading = status === "loading"

  // While loading: the default muted shimmer, a caller override, or
  // nothing (placeholder === null). Once settled the placeholder is
  // dropped and the muted wrapper background covers the fade.
  const placeholderNode = isLoading ? (
    placeholder === undefined ? (
      <span
        aria-hidden="true"
        data-slot="fade-in-image-placeholder"
        className={cn(
          "pointer-events-none absolute inset-0 bg-muted motion-safe:animate-pulse",
          placeholderClassName
        )}
      />
    ) : (
      placeholder
    )
  ) : null

  return (
    <>
      {placeholderNode}
      <img
        {...rest}
        ref={ref}
        src={src}
        alt={alt}
        decoding="async"
        onLoad={(e) => {
          markLoaded()
          onLoad?.(e)
        }}
        onError={(e) => {
          markError()
          onError?.(e)
        }}
        className={cn(
          "transition-opacity duration-200 ease-out motion-reduce:transition-none",
          isLoading ? "opacity-0" : "opacity-100",
          className
        )}
      />
    </>
  )
}
