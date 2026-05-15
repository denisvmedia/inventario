import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { QRCodeSVG } from "qrcode.react"
import { ClipboardCopy, ShieldCheck } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
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
import { useStartMFASetup, useVerifyMFASetup } from "@/features/auth/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { parseServerError } from "@/lib/server-error"

// MFASetupDialog drives the three-step enrollment surface:
//
//   1. "scan"   — show QR + manual secret, ask for a 6-digit code
//   2. "codes"  — show backup codes, mandatory "I've saved these" tick
//   3. (dialog closes; SettingsPage reads the new status from /auth/mfa/status)
//
// The outer component owns nothing but the open/close state — every
// piece of form state lives in <SetupForm/> and is therefore reset
// whenever the dialog closes (the form unmounts). This sidesteps the
// useEffect-resets-state pattern that the React-hooks lint flags.

interface Props {
  open: boolean
  onOpenChange: (next: boolean) => void
}

export function MFASetupDialog({ open, onOpenChange }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="mfa-setup-dialog" className="sm:max-w-md">
        {open ? <SetupForm onClose={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  )
}

type Step = "scan" | "codes"

function SetupForm({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const startSetup = useStartMFASetup()
  const verifySetup = useVerifyMFASetup()

  const [step, setStep] = useState<Step>("scan")
  const [secret, setSecret] = useState("")
  const [qrURL, setQrURL] = useState("")
  const [code, setCode] = useState("")
  const [backupCodes, setBackupCodes] = useState<string[]>([])
  const [acknowledged, setAcknowledged] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)

  // Mount-only side effect: kick off the setup request and capture the
  // returned secret/URL. The form unmounts when the dialog closes, so
  // there's no need for a cleanup/re-run path — re-opening the dialog
  // mounts a fresh <SetupForm/> with new state.
  useEffect(() => {
    let cancelled = false
    startSetup
      .mutateAsync()
      .then((res) => {
        if (cancelled) return
        setSecret(res.secret)
        setQrURL(res.qrCodeURL)
      })
      .catch((err) => {
        if (cancelled) return
        setServerError(parseServerError(err, t("settings:privacy.mfa.setup.errorStart")))
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- mount-only
  }, [])

  const isLoadingSecret = !secret && startSetup.isPending
  const trimmed = code.trim()

  async function handleVerify(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!trimmed) return
    setServerError(null)
    try {
      const codes = await verifySetup.mutateAsync(trimmed)
      setBackupCodes(codes)
      setStep("codes")
    } catch (err) {
      setServerError(parseServerError(err, t("settings:privacy.mfa.setup.errorVerify")))
    }
  }

  function handleFinish() {
    onClose()
    toast.success(t("settings:privacy.mfa.setup.toastSuccess"))
  }

  async function copyBackupCodes() {
    try {
      await navigator.clipboard.writeText(backupCodes.join("\n"))
      toast.success(t("settings:privacy.mfa.setup.copied"))
    } catch {
      toast.error(t("settings:privacy.mfa.setup.copyFailed"))
    }
  }

  if (step === "scan") {
    return (
      <form onSubmit={handleVerify} className="space-y-4" noValidate>
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <ShieldCheck className="size-5" aria-hidden="true" />
            {t("settings:privacy.mfa.setup.scanTitle")}
          </DialogTitle>
          <DialogDescription>{t("settings:privacy.mfa.setup.scanDescription")}</DialogDescription>
        </DialogHeader>

        {isLoadingSecret ? (
          <div
            className="flex justify-center py-8 text-sm text-muted-foreground"
            data-testid="mfa-setup-loading"
          >
            {t("settings:privacy.mfa.setup.generating")}
          </div>
        ) : null}

        {qrURL ? (
          <div className="flex justify-center">
            <div className="rounded-lg bg-white p-3" data-testid="mfa-qr">
              <QRCodeSVG value={qrURL} size={176} marginSize={1} />
            </div>
          </div>
        ) : null}

        {secret ? (
          <div className="space-y-1.5">
            <Label htmlFor="mfa-setup-secret">
              {t("settings:privacy.mfa.setup.manualSecretLabel")}
            </Label>
            <div className="flex items-center gap-2">
              <Input
                id="mfa-setup-secret"
                readOnly
                value={secret}
                className="font-mono text-xs"
                data-testid="mfa-setup-secret"
                onFocus={(e) => e.currentTarget.select()}
              />
              <Button
                type="button"
                variant="outline"
                size="icon"
                aria-label={t("settings:privacy.mfa.setup.copySecret")}
                onClick={() => {
                  void navigator.clipboard.writeText(secret).catch(() => {
                    // best-effort: the input is selectable
                  })
                }}
                data-testid="mfa-setup-secret-copy"
              >
                <ClipboardCopy className="size-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              {t("settings:privacy.mfa.setup.manualSecretHint")}
            </p>
          </div>
        ) : null}

        <div className="space-y-1.5">
          <Label htmlFor="mfa-setup-code">{t("settings:privacy.mfa.setup.codeLabel")}</Label>
          <Input
            id="mfa-setup-code"
            inputMode="numeric"
            autoComplete="one-time-code"
            placeholder={t("settings:privacy.mfa.setup.codePlaceholder")}
            value={code}
            onChange={(e) => setCode(e.target.value)}
            disabled={!secret || verifySetup.isPending}
            data-testid="mfa-setup-code"
          />
        </div>

        {serverError ? (
          <Alert variant="destructive" data-testid="mfa-setup-error">
            <AlertDescription>{serverError}</AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={!secret || !trimmed || verifySetup.isPending}
            data-testid="mfa-setup-verify"
          >
            {verifySetup.isPending
              ? t("settings:privacy.mfa.setup.verifying")
              : t("settings:privacy.mfa.setup.verify")}
          </Button>
        </DialogFooter>
      </form>
    )
  }

  return (
    <div className="space-y-4">
      <DialogHeader>
        <DialogTitle>{t("settings:privacy.mfa.setup.codesTitle")}</DialogTitle>
        <DialogDescription>{t("settings:privacy.mfa.setup.codesDescription")}</DialogDescription>
      </DialogHeader>

      <div
        className="grid grid-cols-2 gap-2 rounded-lg border bg-muted/30 p-4 font-mono text-sm"
        data-testid="mfa-backup-codes"
      >
        {backupCodes.map((bc, idx) => (
          // key={idx} not key={bc}: the list is render-once-and-done so a
          // stable key buys us nothing, and using the secret value as the
          // key would surface it in dev-tools / profiler key columns.
          <span key={idx} className="select-all text-center">
            {bc}
          </span>
        ))}
      </div>

      <div className="flex items-center justify-between gap-3">
        <Button
          type="button"
          variant="outline"
          onClick={() => void copyBackupCodes()}
          data-testid="mfa-copy-backup"
          className="gap-1.5"
        >
          <ClipboardCopy className="size-4" />
          {t("settings:privacy.mfa.setup.copyAll")}
        </Button>
        <label className="flex items-center gap-2 text-sm">
          <Checkbox
            checked={acknowledged}
            onCheckedChange={(next) => setAcknowledged(next === true)}
            data-testid="mfa-ack-saved"
          />
          {t("settings:privacy.mfa.setup.ackSaved")}
        </label>
      </div>

      <DialogFooter>
        <Button disabled={!acknowledged} onClick={handleFinish} data-testid="mfa-finish">
          {t("settings:privacy.mfa.setup.finish")}
        </Button>
      </DialogFooter>
    </div>
  )
}
