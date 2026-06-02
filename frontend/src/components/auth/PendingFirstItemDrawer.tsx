import { useState } from "react"
import { useTranslation } from "react-i18next"
import { PackageCheck } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Drawer,
  DrawerClose,
  DrawerContent,
  DrawerDescription,
  DrawerFooter,
  DrawerHeader,
  DrawerTitle,
} from "@/components/ui/drawer"

// Reassurance drawer shown on the Login / Register pages when an anonymous
// visitor drafted their first item before signing up (#1988). A muted inline
// banner was too easy to skip past, so this is an up-front bottom Drawer
// (the design mock's vaul-based primitive — UIShowcaseView's Drawer demo):
// it greets the visitor with "your item is saved, just sign in / register",
// they tap "Got it", and the form is revealed underneath. Purely
// informational — the draft replay still happens at /welcome via
// FirstItemResolver.
//
// Self-opening on mount (the caller only renders it when a pending marker
// exists) and self-dismissing; it does not auto-reopen within a mount.
export function PendingFirstItemDrawer() {
  const { t } = useTranslation()
  const [open, setOpen] = useState(true)

  return (
    <Drawer open={open} onOpenChange={setOpen}>
      <DrawerContent data-testid="pending-first-item-drawer">
        {/* Constrain the bottom sheet's content to a readable column on
            desktop — the canonical shadcn drawer pattern. */}
        <div className="mx-auto w-full max-w-sm">
          <DrawerHeader>
            <div className="mx-auto mb-1 flex size-12 items-center justify-center rounded-xl bg-primary/10">
              <PackageCheck className="size-6 text-primary" aria-hidden="true" />
            </div>
            <DrawerTitle>{t("auth:firstItem.title")}</DrawerTitle>
            <DrawerDescription className="leading-relaxed">
              {t("auth:firstItem.body")}
            </DrawerDescription>
          </DrawerHeader>
          <DrawerFooter>
            <DrawerClose asChild>
              <Button className="w-full" data-testid="pending-first-item-drawer-ok">
                {t("auth:firstItem.ok")}
              </Button>
            </DrawerClose>
          </DrawerFooter>
        </div>
      </DrawerContent>
    </Drawer>
  )
}
