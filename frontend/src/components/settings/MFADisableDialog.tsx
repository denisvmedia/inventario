import { useState } from "react"
import { useTranslation } from "react-i18next"
import { ShieldOff } from "lucide-react"

import { PasswordInput } from "@/components/auth/PasswordInput"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useDisableMFA } from "@/features/auth/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { parseServerError } from "@/lib/server-error"

// MFADisableDialog asks for password + either a current TOTP code or
// an unused backup code, mirroring the issue's re-auth requirement.
// Removes the user's MFA row on success; SettingsPage's status query
// is invalidated by the hook so the badge flips back to Inactive.

interface Props {
  open: boolean
  onOpenChange: (next: boolean) => void
}

type Mode = "totp" | "backup"

// Outer component owns the open/close binding; the inner <DisableForm/>
// owns every piece of form state. Closing the dialog unmounts the form
// — that's the reset, no useEffect needed (and no React-hooks lint
// complaints about setState in an effect).
export function MFADisableDialog({ open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="mfa-disable-dialog" className="sm:max-w-md">
        {open ? <DisableForm onClose={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

function DisableForm({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const disableMFA = useDisableMFA()

  const [password, setPassword] = useState("")
  const [code, setCode] = useState("")
  const [mode, setMode] = useState<Mode>("totp")
  const [serverError, setServerError] = useState<string | null>(null)

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setServerError(null)
    try {
      await disableMFA.mutateAsync({
        password,
        totpCode: mode === "totp" ? code.trim() || undefined : undefined,
        backupCode: mode === "backup" ? code.trim() || undefined : undefined,
      })
      onClose()
      toast.success(t("settings:privacy.mfa.disable.toastSuccess"))
    } catch (err) {
      setServerError(parseServerError(err, t("settings:privacy.mfa.disable.error")))
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4" noValidate>
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2 text-destructive">
          <ShieldOff className="size-5" aria-hidden="true" />
          {t("settings:privacy.mfa.disable.title")}
        </DialogTitle>
        <DialogDescription>{t("settings:privacy.mfa.disable.description")}</DialogDescription>
      </DialogHeader>

      <div className="space-y-1.5">
        <Label htmlFor="mfa-disable-password">{t("settings:privacy.mfa.disable.password")}</Label>
        <PasswordInput
          id="mfa-disable-password"
          autoComplete="current-password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          disabled={disableMFA.isPending}
          data-testid="mfa-disable-password"
        />
      </div>

      <div className="space-y-1.5">
        <Label htmlFor="mfa-disable-code">
          {mode === "totp"
            ? t("settings:privacy.mfa.disable.totpLabel")
            : t("settings:privacy.mfa.disable.backupLabel")}
        </Label>
        <Input
          id="mfa-disable-code"
          inputMode={mode === "totp" ? "numeric" : "text"}
          autoComplete="one-time-code"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder={
            mode === "totp"
              ? t("settings:privacy.mfa.disable.totpPlaceholder")
              : t("settings:privacy.mfa.disable.backupPlaceholder")
          }
          disabled={disableMFA.isPending}
          data-testid="mfa-disable-code"
          data-mode={mode}
        />
        <button
          type="button"
          onClick={() => {
            setCode("")
            setMode(mode === "totp" ? "backup" : "totp")
          }}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
          data-testid="mfa-disable-toggle"
        >
          {mode === "totp"
            ? t("settings:privacy.mfa.disable.useBackupInstead")
            : t("settings:privacy.mfa.disable.useTotpInstead")}
        </button>
      </div>

      {serverError ? (
        <Alert variant="destructive" data-testid="mfa-disable-error">
          <AlertDescription>{serverError}</AlertDescription>
        </Alert>
      ) : null}

      <DialogFooter>
        <Button type="button" variant="outline" onClick={onClose}>
          {t("common:actions.cancel")}
        </Button>
        <Button
          type="submit"
          variant="destructive"
          disabled={!password || !code.trim() || disableMFA.isPending}
          data-testid="mfa-disable-confirm"
        >
          {disableMFA.isPending
            ? t("settings:privacy.mfa.disable.submitting")
            : t("settings:privacy.mfa.disable.submit")}
        </Button>
      </DialogFooter>
    </form>
  )
}
