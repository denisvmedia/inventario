"use client"

import type { CSSProperties } from "react"
import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { Toaster as Sonner, type ToasterProps } from "sonner"

import { useTheme } from "@/components/theme-provider"
import { TOAST_BOTTOM_INSET_PROPERTY } from "@/components/ui/dialog-phone"

// Below `sm` a full-screen dialog sets this property to the height of its
// footer and of the on-screen keyboard, so toasts sit above its buttons. It
// is unset otherwise. 24px and 16px are Sonner's own offsets on a wide and a
// narrow (under 600px) screen.
const dialogInset = `var(${TOAST_BOTTOM_INSET_PROPERTY}, 0px)`
const offset = { bottom: `calc(24px + ${dialogInset})` }
const mobileOffset = { bottom: `calc(16px + ${dialogInset})` }

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      offset={offset}
      mobileOffset={mobileOffset}
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
          "--border-radius": "var(--radius)",
        } as CSSProperties
      }
      {...props}
    />
  )
}

export { Toaster }
