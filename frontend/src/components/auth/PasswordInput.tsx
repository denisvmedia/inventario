import { forwardRef, useState, type ComponentProps } from "react"
import { Eye, EyeOff, Lock } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

type PasswordInputProps = Omit<ComponentProps<"input">, "type"> & {
  // When true, hides the leading lock icon. Keeps line-up with sibling inputs
  // that don't have a leading affordance.
  hideLockIcon?: boolean
}

// Password field with eye toggle — used by every auth form. The toggle does
// not change the field's React `value`, so RHF/zod stay oblivious. Accessible
// label on the toggle button switches between "Show"/"Hide" via i18n.
export const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(
  function PasswordInput({ className, hideLockIcon, ...props }, ref) {
    const { t } = useTranslation()
    const [show, setShow] = useState(false)
    return (
      <div className="relative">
        {!hideLockIcon ? (
          <Lock
            aria-hidden="true"
            className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
          />
        ) : null}
        <Input
          ref={ref}
          type={show ? "text" : "password"}
          className={cn(hideLockIcon ? "pr-9" : "pl-9 pr-9", className)}
          {...props}
        />
        <button
          type="button"
          onClick={() => setShow((v) => !v)}
          className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
          aria-label={show ? t("auth:passwordHide") : t("auth:passwordShow")}
        >
          {show ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
        </button>
      </div>
    )
  }
)
