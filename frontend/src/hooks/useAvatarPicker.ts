import { useCallback, useEffect, useRef, useState, type ChangeEvent } from "react"
import { useTranslation } from "react-i18next"

import { AVATAR_CONTENT_TYPES, AVATAR_MAX_BYTES } from "@/features/auth/api"
import { useUploadAvatar } from "@/features/auth/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { HttpError } from "@/lib/http"

// preload resolves once the browser has the image, or has failed to get it.
// Swapping the local preview for the stored photo only then means the tile
// goes straight from one picture to the other, with no flash of initials while
// the new URL loads.
function preload(src: string): Promise<void> {
  return new Promise((resolve) => {
    const img = new Image()
    img.onload = () => resolve()
    img.onerror = () => resolve()
    img.src = src
  })
}

// useAvatarPicker drives the profile photo upload (#1382): a hidden file input
// the page renders with `inputProps`, an `open()` the visible controls call,
// and the local preview to show while the upload runs.
//
// The chosen file is shown at once from an object URL, then replaced by the
// stored photo when the server answers. A file of the wrong type or over 2 MB
// is refused here, before it is sent; the server checks both again.
export function useAvatarPicker() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const upload = useUploadAvatar()
  const inputRef = useRef<HTMLInputElement>(null)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)

  // Release each object URL once it is replaced or the page goes away.
  useEffect(() => {
    if (!previewUrl) return
    return () => URL.revokeObjectURL(previewUrl)
  }, [previewUrl])

  const open = useCallback(() => inputRef.current?.click(), [])

  async function handleFile(file: File) {
    if (!(AVATAR_CONTENT_TYPES as readonly string[]).includes(file.type)) {
      toast.error(t("settings:profile.photo.errors.type"))
      return
    }
    if (file.size > AVATAR_MAX_BYTES) {
      toast.error(t("settings:profile.photo.errors.size"))
      return
    }

    setPreviewUrl(URL.createObjectURL(file))
    try {
      const result = await upload.mutateAsync(file)
      if (result.avatar_url) await preload(result.avatar_url)
      toast.success(t("settings:profile.photo.uploaded"))
    } catch (err) {
      // A 400 is the server refusing the image itself (animated, or not
      // decodable); anything else is the upload failing.
      toast.error(
        err instanceof HttpError && err.status === 400
          ? t("settings:profile.photo.errors.rejected")
          : t("settings:profile.photo.errors.upload")
      )
    } finally {
      setPreviewUrl(null)
    }
  }

  function onChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    // Cleared so that choosing the same file again still fires a change.
    event.target.value = ""
    if (file) void handleFile(file)
  }

  return {
    open,
    previewUrl,
    isUploading: upload.isPending,
    inputProps: {
      ref: inputRef,
      type: "file" as const,
      accept: AVATAR_CONTENT_TYPES.join(","),
      onChange,
      className: "sr-only",
      tabIndex: -1,
      "aria-hidden": true,
    },
  }
}
