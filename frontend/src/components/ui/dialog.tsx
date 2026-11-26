import * as React from "react"
import { XIcon } from "lucide-react"
import { Dialog as DialogPrimitive } from "radix-ui"
import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"

// "default" is the centered card at every width. "fullscreen-mobile" is for
// long forms: below `sm` (640px) the dialog fills the screen, the header
// (title and close button) sticks to the top, the footer (the actions) sticks
// to the bottom above the safe-area inset, and the content between them
// scrolls; it slides up on open and down on close. From `sm` up it is the
// same centered card as "default".
type DialogContentVariant = "default" | "fullscreen-mobile"

// DialogHeader and DialogFooter read the variant from here, so a dialog opts
// in with one prop on DialogContent and keeps its markup.
const DialogLayoutContext = React.createContext<{
  variant: DialogContentVariant
  showCloseButton: boolean
}>({ variant: "default", showCloseButton: false })

const closeButtonClassName =
  "opacity-70 ring-offset-background transition-opacity hover:opacity-100 focus:outline-hidden disabled:pointer-events-none data-[state=open]:bg-accent data-[state=open]:text-muted-foreground [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4"

// Below `sm` the content box is the scroll container. Its scroll padding
// keeps a field the browser scrolls to on focus clear of the sticky header
// (up to three lines of title and description) and footer (up to two rows of
// buttons). Every direct child keeps its height (`*:shrink-0`) so an inner
// list with its own max height does not collapse. A direct child that wraps
// the footer (usually the form) grows to fill the screen and drops its bottom
// padding, so the footer (`mt-auto`) sits on the bottom edge even when the
// content is short. Under reduced motion the slide becomes the default fade.
// From `sm` up the card is never taller than the screen (a phone held
// sideways) and scrolls as a whole.
const fullscreenMobileContentClassName = cn(
  "max-sm:top-0 max-sm:left-0 max-sm:flex max-sm:h-dvh max-sm:max-h-none max-sm:w-full max-sm:max-w-none max-sm:translate-none max-sm:flex-col",
  "max-sm:overflow-y-auto max-sm:overscroll-contain max-sm:scroll-pt-36 max-sm:scroll-pb-28 max-sm:rounded-none max-sm:border-0 max-sm:px-4 max-sm:py-0 max-sm:shadow-none",
  "sm:max-h-[calc(100dvh-2rem)] sm:overflow-y-auto",
  "max-sm:*:shrink-0",
  "max-sm:[&>*:has(>[data-slot=dialog-footer])]:flex max-sm:[&>*:has(>[data-slot=dialog-footer])]:flex-1 max-sm:[&>*:has(>[data-slot=dialog-footer])]:flex-col max-sm:[&>*:has(>[data-slot=dialog-footer])]:pb-0",
  "max-sm:duration-300 max-sm:data-[state=closed]:ease-in max-sm:data-[state=open]:ease-out",
  "max-sm:data-[state=closed]:zoom-out-100 max-sm:data-[state=open]:zoom-in-100",
  "max-sm:motion-safe:data-[state=closed]:fade-out-100 max-sm:motion-safe:data-[state=closed]:slide-out-to-bottom",
  "max-sm:motion-safe:data-[state=open]:fade-in-100 max-sm:motion-safe:data-[state=open]:slide-in-from-bottom"
)

// The header sticks to the top of the scrolling content and spans it edge to
// edge (`-mx-4` cancels the content's side padding). Its text is left-aligned
// at every width, next to the close button. `pr-14` keeps the title clear of
// that button.
const fullscreenMobileHeaderClassName =
  "text-left max-sm:sticky max-sm:top-0 max-sm:z-10 max-sm:-mx-4 max-sm:border-b max-sm:bg-background max-sm:px-4 max-sm:pt-[calc(0.75rem+env(safe-area-inset-top))] max-sm:pb-3 max-sm:pr-14"

// The footer sticks to the bottom, lays its buttons out in a row (wrapping
// when they do not fit) and pads itself above the home indicator.
const fullscreenMobileFooterClassName =
  "flex-row justify-end max-sm:sticky max-sm:bottom-0 max-sm:z-10 max-sm:-mx-4 max-sm:mt-auto max-sm:flex-wrap max-sm:border-t max-sm:bg-background max-sm:px-4 max-sm:pt-3 max-sm:pb-[calc(0.75rem+env(safe-area-inset-bottom))]"

function Dialog({ ...props }: React.ComponentProps<typeof DialogPrimitive.Root>) {
  return <DialogPrimitive.Root data-slot="dialog" {...props} />
}

function DialogTrigger({ ...props }: React.ComponentProps<typeof DialogPrimitive.Trigger>) {
  return <DialogPrimitive.Trigger data-slot="dialog-trigger" {...props} />
}

function DialogPortal({ ...props }: React.ComponentProps<typeof DialogPrimitive.Portal>) {
  return <DialogPrimitive.Portal data-slot="dialog-portal" {...props} />
}

function DialogClose({ ...props }: React.ComponentProps<typeof DialogPrimitive.Close>) {
  return <DialogPrimitive.Close data-slot="dialog-close" {...props} />
}

function DialogOverlay({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Overlay>) {
  return (
    <DialogPrimitive.Overlay
      data-slot="dialog-overlay"
      className={cn(
        "fixed inset-0 z-50 bg-black/50 data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:animate-in data-[state=open]:fade-in-0",
        className
      )}
      {...props}
    />
  )
}

function DialogContent({
  className,
  children,
  showCloseButton = true,
  variant = "default",
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Content> & {
  showCloseButton?: boolean
  variant?: DialogContentVariant
}) {
  const { t } = useTranslation()
  const fullscreenMobile = variant === "fullscreen-mobile"
  const layout = React.useMemo(() => ({ variant, showCloseButton }), [variant, showCloseButton])
  return (
    <DialogPortal data-slot="dialog-portal">
      <DialogOverlay />
      <DialogPrimitive.Content
        data-slot="dialog-content"
        data-variant={variant}
        className={cn(
          "fixed top-[50%] left-[50%] z-50 grid w-full max-w-[calc(100%-2rem)] translate-x-[-50%] translate-y-[-50%] gap-4 rounded-lg border bg-background p-6 shadow-lg duration-200 outline-none data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=closed]:zoom-out-95 data-[state=open]:animate-in data-[state=open]:fade-in-0 data-[state=open]:zoom-in-95 sm:max-w-lg",
          fullscreenMobile && fullscreenMobileContentClassName,
          className
        )}
        {...props}
      >
        <DialogLayoutContext.Provider value={layout}>{children}</DialogLayoutContext.Provider>
        {showCloseButton && (
          // In the full-screen layout the header carries the close button
          // below `sm`, so this one only shows from `sm` up.
          <DialogPrimitive.Close
            data-slot="dialog-close"
            className={cn(
              "absolute top-4 right-4 rounded-xs focus:ring-2 focus:ring-ring focus:ring-offset-2",
              closeButtonClassName,
              fullscreenMobile && "max-sm:hidden"
            )}
          >
            <XIcon />
            <span className="sr-only">{t("common:actions.close")}</span>
          </DialogPrimitive.Close>
        )}
      </DialogPrimitive.Content>
    </DialogPortal>
  )
}

function DialogHeader({ className, children, ...props }: React.ComponentProps<"div">) {
  const { t } = useTranslation()
  const { variant, showCloseButton } = React.useContext(DialogLayoutContext)
  const fullscreenMobile = variant === "fullscreen-mobile"
  return (
    <div
      data-slot="dialog-header"
      className={cn(
        "flex flex-col gap-2 text-center sm:text-left",
        fullscreenMobile && fullscreenMobileHeaderClassName,
        className
      )}
      {...props}
    >
      {children}
      {fullscreenMobile && showCloseButton && (
        // Below `sm` the close button stays with the sticky header. It is the
        // first control in the dialog there, so it takes the focus on open
        // and the on-screen keyboard stays closed until a field is tapped.
        // It is 44px square, a full touch target. The ring shows for keyboard
        // focus only. From `sm` up it is hidden and the content's own close
        // button (last in the dialog) is used, as in "default".
        <DialogPrimitive.Close
          data-slot="dialog-close"
          className={cn(
            "absolute top-1 right-1 mt-[env(safe-area-inset-top)] inline-flex size-11 items-center justify-center rounded-md focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 sm:hidden",
            closeButtonClassName
          )}
        >
          <XIcon />
          <span className="sr-only">{t("common:actions.close")}</span>
        </DialogPrimitive.Close>
      )}
    </div>
  )
}

function DialogFooter({
  className,
  showCloseButton = false,
  children,
  ...props
}: React.ComponentProps<"div"> & {
  showCloseButton?: boolean
}) {
  const { t } = useTranslation()
  const { variant } = React.useContext(DialogLayoutContext)
  return (
    <div
      data-slot="dialog-footer"
      className={cn(
        "flex flex-col-reverse gap-2 sm:flex-row sm:justify-end",
        variant === "fullscreen-mobile" && fullscreenMobileFooterClassName,
        className
      )}
      {...props}
    >
      {children}
      {showCloseButton && (
        <DialogPrimitive.Close asChild>
          <Button variant="outline">{t("common:actions.close")}</Button>
        </DialogPrimitive.Close>
      )}
    </div>
  )
}

function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return (
    <DialogPrimitive.Title
      data-slot="dialog-title"
      className={cn("text-lg leading-none font-semibold", className)}
      {...props}
    />
  )
}

function DialogDescription({
  className,
  ...props
}: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return (
    <DialogPrimitive.Description
      data-slot="dialog-description"
      className={cn("text-sm text-muted-foreground", className)}
      {...props}
    />
  )
}

export {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogOverlay,
  DialogPortal,
  DialogTitle,
  DialogTrigger,
  type DialogContentVariant,
}
