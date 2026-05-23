import { useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { ArrowRight, KeyRound, ShieldCheck } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useBackofficeCompleteMFA } from "@/features/backoffice/auth/hooks"
import { parseServerError } from "@/lib/server-error"

// BackofficeMFAChallenge mirrors components/auth/MFAChallenge.tsx but
// targets the back-office plane's /backoffice/auth/login/mfa endpoint via
// `useBackofficeCompleteMFA`. The two are intentionally separate
// components rather than one parameterised — the tenant variant is a
// stable surface used by many code paths and threading a "which plane?"
// prop through it would muddy that. The component itself is small.
interface Props {
  mfaToken: string
  email: string
  onSuccess: () => void
  onCancel: () => void
}

type Mode = "totp" | "backup"

export function BackofficeMFAChallenge({ mfaToken, email, onSuccess, onCancel }: Props) {
  const { t } = useTranslation("backoffice")
  const completeMutation = useBackofficeCompleteMFA()

  const [mode, setMode] = useState<Mode>("totp")
  const [code, setCode] = useState("")
  const [serverError, setServerError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement | null>(null)

  // Auto-focus the code input on mount — same SR/keyboard rationale as
  // the tenant MFAChallenge. Ref-based focus avoids jsx-a11y/no-autofocus.
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const isPending = completeMutation.isPending
  const trimmed = code.trim()

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!trimmed) return
    setServerError(null)
    try {
      await completeMutation.mutateAsync({
        mfaToken,
        totpCode: mode === "totp" ? trimmed : undefined,
        backupCode: mode === "backup" ? trimmed : undefined,
      })
      onSuccess()
    } catch (err) {
      setServerError(parseServerError(err, t("mfa.challenge.error")))
    }
  }

  return (
    <div className="space-y-6" data-testid="backoffice-mfa-challenge">
      <header className="space-y-1.5">
        <div className="flex items-center gap-2">
          <ShieldCheck className="size-5 text-foreground" aria-hidden="true" />
          <h1 className="text-2xl font-semibold tracking-tight">{t("mfa.challenge.title")}</h1>
        </div>
        <p className="text-sm text-muted-foreground">{t("mfa.challenge.subtitle", { email })}</p>
      </header>

      <form className="space-y-4" onSubmit={handleSubmit} noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="backoffice-mfa-code">
            {mode === "totp" ? t("mfa.challenge.totpLabel") : t("mfa.challenge.backupLabel")}
          </Label>
          <Input
            id="backoffice-mfa-code"
            ref={inputRef}
            inputMode={mode === "totp" ? "numeric" : "text"}
            autoComplete="one-time-code"
            placeholder={
              mode === "totp"
                ? t("mfa.challenge.totpPlaceholder")
                : t("mfa.challenge.backupPlaceholder")
            }
            value={code}
            onChange={(e) => setCode(e.target.value)}
            disabled={isPending}
            data-testid="backoffice-mfa-code-input"
            data-mode={mode}
          />
          <p className="text-xs text-muted-foreground">
            {mode === "totp" ? t("mfa.challenge.totpHint") : t("mfa.challenge.backupHint")}
          </p>
        </div>

        {serverError ? (
          <Alert variant="destructive" data-testid="backoffice-mfa-server-error">
            <AlertDescription>{serverError}</AlertDescription>
          </Alert>
        ) : null}

        <Button
          type="submit"
          className="w-full gap-2"
          disabled={isPending || !trimmed}
          data-testid="backoffice-mfa-submit"
        >
          {isPending ? t("mfa.challenge.submitting") : t("mfa.challenge.submit")}
          {!isPending ? <ArrowRight className="size-4" /> : null}
        </Button>
      </form>

      <div className="flex flex-col gap-2 text-center">
        <button
          type="button"
          onClick={() => {
            setCode("")
            setServerError(null)
            setMode((prev) => (prev === "totp" ? "backup" : "totp"))
          }}
          className="inline-flex items-center justify-center gap-1.5 text-sm font-medium text-foreground hover:underline underline-offset-4"
          data-testid="backoffice-mfa-toggle-mode"
        >
          <KeyRound className="size-3.5" aria-hidden="true" />
          {mode === "totp"
            ? t("mfa.challenge.useBackupInstead")
            : t("mfa.challenge.useTotpInstead")}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
          data-testid="backoffice-mfa-cancel"
        >
          {t("mfa.challenge.cancel")}
        </button>
      </div>
    </div>
  )
}
