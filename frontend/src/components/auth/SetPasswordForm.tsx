import { useEffect, useMemo, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { CheckCircle2, KeyRound } from "lucide-react"

import { PasswordInput } from "@/components/auth/PasswordInput"
import { PasswordStrengthMeter } from "@/components/auth/PasswordStrengthMeter"
import { FieldError } from "@/components/FieldError"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/features/auth/AuthContext"
import { useChangePassword, useLogout } from "@/features/auth/hooks"
import { setPasswordSchema, type SetPasswordInput } from "@/features/auth/schemas"
import { classifyServerError, type ClassifiedServerError } from "@/lib/server-error"

// SetPasswordForm — surface for OAuth-only users to set their first
// password (#1394). Same look as the password card on EditProfilePage,
// minus the "current password" field that an OAuth-only user can't fill.
// Posts to /auth/change-password with `current_password: ""`; the BE
// skips the current-password verification when the user has no hash
// on file. On success the BE revokes every session, so we sign out
// and bounce to /login the same way the regular change-password form
// does — the user re-authenticates with the new password.
export function SetPasswordForm() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const changePasswordMutation = useChangePassword()
  const logoutMutation = useLogout()

  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)
  const [success, setSuccess] = useState(false)

  const form = useForm<SetPasswordInput>({
    resolver: zodResolver(setPasswordSchema),
    defaultValues: { newPassword: "", confirmPassword: "" },
  })

  const newPasswordValue = form.watch("newPassword") ?? ""
  const strengthInputs = useMemo(
    () => [user?.name ?? "", user?.email ?? ""].map((v) => v.trim()).filter(Boolean),
    [user?.name, user?.email]
  )

  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  async function onSubmit(values: SetPasswordInput) {
    setServerError(null)
    try {
      await changePasswordMutation.mutateAsync({
        // Empty current_password tells the BE we're in the OAuth-only
        // first-set branch; the handler honours it only when the row's
        // PasswordHash is empty, otherwise it rejects with 400.
        current_password: "",
        new_password: values.newPassword,
      })
      setSuccess(true)
      form.reset()
      window.setTimeout(() => {
        void (async () => {
          try {
            await logoutMutation.mutateAsync()
          } catch (logoutErr) {
            console.warn("[SetPassword] Logout after first-set failed:", logoutErr)
          } finally {
            navigate("/login")
          }
        })()
      }, 1500)
    } catch (err) {
      setServerError(classifyServerError(err, t("auth:setPassword.errorGeneric")))
    }
  }

  return (
    <form
      className="set-password-form space-y-4 rounded-xl border border-border bg-card p-5"
      onSubmit={form.handleSubmit(onSubmit)}
      noValidate
      data-testid="set-password-form"
    >
      <div className="flex items-start gap-3">
        <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/60">
          <KeyRound className="size-4 text-muted-foreground" aria-hidden="true" />
        </div>
        <div className="space-y-0.5">
          <h2 className="text-base font-semibold">{t("auth:setPassword.title")}</h2>
          <p className="text-sm text-muted-foreground">{t("auth:setPassword.subtitle")}</p>
        </div>
      </div>

      {success ? (
        <Alert className="success-banner" data-testid="set-password-success">
          <CheckCircle2 aria-hidden="true" />
          <AlertDescription>
            <strong>{t("auth:setPassword.successTitle")}</strong>
            {" — "}
            {t("auth:setPassword.successBody")}
          </AlertDescription>
        </Alert>
      ) : (
        <>
          <div className="space-y-1.5">
            <Label htmlFor="set-new-password">{t("auth:fields.newPassword")}</Label>
            <PasswordInput
              id="set-new-password"
              autoComplete="new-password"
              hideLockIcon
              disabled={changePasswordMutation.isPending}
              aria-invalid={!!form.formState.errors.newPassword}
              aria-describedby={
                form.formState.errors.newPassword ? "set-new-password-error" : undefined
              }
              data-testid="set-new-password"
              {...form.register("newPassword")}
            />
            <PasswordStrengthMeter
              password={newPasswordValue}
              userInputs={strengthInputs}
              testId="set-password-strength"
            />
            <FieldError
              id="set-new-password-error"
              testId="set-new-password-error"
              message={form.formState.errors.newPassword?.message}
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="set-confirm-password">{t("auth:fields.confirmPassword")}</Label>
            <PasswordInput
              id="set-confirm-password"
              autoComplete="new-password"
              hideLockIcon
              disabled={changePasswordMutation.isPending}
              aria-invalid={!!form.formState.errors.confirmPassword}
              aria-describedby={
                form.formState.errors.confirmPassword ? "set-confirm-password-error" : undefined
              }
              data-testid="set-confirm-password"
              {...form.register("confirmPassword")}
            />
            <FieldError
              id="set-confirm-password-error"
              testId="set-confirm-password-error"
              message={form.formState.errors.confirmPassword?.message}
            />
          </div>

          <ServerErrorBanner
            error={serverError}
            className="error-banner"
            testId="set-password-server-error"
          />

          <div className="flex justify-end pt-2">
            <Button
              type="submit"
              className="gap-2"
              disabled={changePasswordMutation.isPending}
              data-testid="set-password-submit"
            >
              {changePasswordMutation.isPending
                ? t("auth:setPassword.submitting")
                : t("auth:setPassword.submit")}
            </Button>
          </div>
        </>
      )}
    </form>
  )
}
