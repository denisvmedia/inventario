import { useState } from "react"
import { useTranslation } from "react-i18next"
import { ArrowRight, KeyRound, ShieldCheck } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useCompleteLoginMFA } from "@/features/auth/hooks"
import { parseServerError } from "@/lib/server-error"

// MFAChallenge owns the second-step UI of the post-MFA login flow.
// Rendered by LoginPage after /auth/login responds with mfa_required.
// On success, it calls `onSuccess` with the resolved CurrentUser so
// the parent can run the invite-accept + redirect chain that the
// regular login path uses.
//
// Two input modes: a 6-digit TOTP from the authenticator app (default)
// and a backup code (toggle). The toggle keeps the codebase free of a
// "if I had no authenticator, what now?" page since the recovery UX
// is one click away.
interface Props {
  mfaToken: string
  email: string
  onSuccess: () => void
  onCancel: () => void
}

type Mode = "totp" | "backup"

export function MFAChallenge({ mfaToken, email, onSuccess, onCancel }: Props) {
  const { t } = useTranslation()
  const completeLoginMutation = useCompleteLoginMFA()

  const [mode, setMode] = useState<Mode>("totp")
  const [code, setCode] = useState("")
  const [serverError, setServerError] = useState<string | null>(null)

  const isPending = completeLoginMutation.isPending
  const trimmed = code.trim()

  // Submit handler keeps the same flow regardless of mode: the
  // backend chooses which field to consume based on whether
  // totp_code or backup_code is non-empty.
  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!trimmed) return
    setServerError(null)
    try {
      await completeLoginMutation.mutateAsync({
        mfaToken,
        totpCode: mode === "totp" ? trimmed : undefined,
        backupCode: mode === "backup" ? trimmed : undefined,
      })
      onSuccess()
    } catch (err) {
      setServerError(parseServerError(err, t("auth:mfa.challenge.error")))
    }
  }

  return (
    <div className="space-y-6" data-testid="mfa-challenge">
      <header className="space-y-1.5">
        <div className="flex items-center gap-2">
          <ShieldCheck className="size-5 text-foreground" aria-hidden="true" />
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth:mfa.challenge.title")}</h1>
        </div>
        <p className="text-sm text-muted-foreground">
          {t("auth:mfa.challenge.subtitle", { email })}
        </p>
      </header>

      <form className="space-y-4" onSubmit={handleSubmit} noValidate>
        <div className="space-y-1.5">
          <Label htmlFor="mfa-code">
            {mode === "totp"
              ? t("auth:mfa.challenge.totpLabel")
              : t("auth:mfa.challenge.backupLabel")}
          </Label>
          <Input
            id="mfa-code"
            inputMode={mode === "totp" ? "numeric" : "text"}
            autoComplete="one-time-code"
            placeholder={
              mode === "totp"
                ? t("auth:mfa.challenge.totpPlaceholder")
                : t("auth:mfa.challenge.backupPlaceholder")
            }
            value={code}
            onChange={(e) => setCode(e.target.value)}
            disabled={isPending}
            data-testid="mfa-code-input"
            data-mode={mode}
          />
          <p className="text-xs text-muted-foreground">
            {mode === "totp"
              ? t("auth:mfa.challenge.totpHint")
              : t("auth:mfa.challenge.backupHint")}
          </p>
        </div>

        {serverError ? (
          <Alert variant="destructive" data-testid="mfa-server-error">
            <AlertDescription>{serverError}</AlertDescription>
          </Alert>
        ) : null}

        <Button
          type="submit"
          className="w-full gap-2"
          disabled={isPending || !trimmed}
          data-testid="mfa-submit"
        >
          {isPending ? t("auth:mfa.challenge.submitting") : t("auth:mfa.challenge.submit")}
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
          data-testid="mfa-toggle-mode"
        >
          <KeyRound className="size-3.5" aria-hidden="true" />
          {mode === "totp"
            ? t("auth:mfa.challenge.useBackupInstead")
            : t("auth:mfa.challenge.useTotpInstead")}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-xs text-muted-foreground hover:text-foreground transition-colors"
          data-testid="mfa-cancel"
        >
          {t("auth:mfa.challenge.cancel")}
        </button>
      </div>
    </div>
  )
}
