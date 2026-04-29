import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router-dom"
import { ArrowLeft, ArrowRight, CheckCircle2, Lock, Mail, User } from "lucide-react"

import { ComingSoonBanner } from "@/components/coming-soon"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/features/auth/AuthContext"
import { useChangePassword, useLogout, useUpdateProfile } from "@/features/auth/hooks"
import { useGroups } from "@/features/group/hooks"
import {
  changePasswordSchema,
  profileEditSchema,
  type ChangePasswordInput,
  type ProfileEditInput,
} from "@/features/auth/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { HttpError } from "@/lib/http"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// /profile/edit — two side-by-side forms in one page: profile fields
// (name + default group) + password change. Email is intentionally
// read-only because the backend ignores it on PUT /auth/me.
export function EditProfilePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const { data: groups } = useGroups()
  const toast = useAppToast()
  const updateMutation = useUpdateProfile()
  const changePasswordMutation = useChangePassword()
  const logoutMutation = useLogout()

  const [profileError, setProfileError] = useState<string | null>(null)
  const [passwordError, setPasswordError] = useState<string | null>(null)
  const [passwordSuccess, setPasswordSuccess] = useState(false)

  const profileForm = useForm<ProfileEditInput>({
    resolver: zodResolver(profileEditSchema),
    defaultValues: {
      name: user?.name ?? "",
      defaultGroupId: user?.default_group_id ?? "",
    },
  })

  const passwordForm = useForm<ChangePasswordInput>({
    resolver: zodResolver(changePasswordSchema),
    defaultValues: { currentPassword: "", newPassword: "", confirmPassword: "" },
  })

  // Sync default values once the auth probe finishes — useForm's defaults
  // are read once on mount, so a Profile page hard-refresh that races the
  // /auth/me probe needs an explicit reset.
  useEffect(() => {
    if (!user) return
    profileForm.reset({
      name: user.name ?? "",
      defaultGroupId: user.default_group_id ?? "",
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [user?.id])

  // Drop stale server errors when the user starts editing again.
  useEffect(() => {
    const sub = profileForm.watch(() => {
      if (profileError) setProfileError(null)
    })
    return () => sub.unsubscribe()
  }, [profileForm, profileError])
  useEffect(() => {
    const sub = passwordForm.watch(() => {
      if (passwordError) setPasswordError(null)
    })
    return () => sub.unsubscribe()
  }, [passwordForm, passwordError])

  async function onProfileSubmit(values: ProfileEditInput) {
    setProfileError(null)
    try {
      await updateMutation.mutateAsync({
        name: values.name.trim(),
        // Empty string in the select → null on the wire (clears the
        // preference); a uuid sets it. Sending undefined would leave the
        // server-side value untouched, which we don't want — the form is
        // the source of truth on save.
        default_group_id: values.defaultGroupId ? values.defaultGroupId : null,
      })
      toast.success(t("settings:profile.edit.successToast"))
      navigate("/profile")
    } catch (err) {
      setProfileError(parseServerError(err, t("settings:profile.edit.errorGeneric")))
    }
  }

  async function onPasswordSubmit(values: ChangePasswordInput) {
    setPasswordError(null)
    try {
      await changePasswordMutation.mutateAsync({
        current_password: values.currentPassword,
        new_password: values.newPassword,
      })
      setPasswordSuccess(true)
      // The server invalidated every session — sign out locally and bounce
      // to /login so the user re-authenticates with the new password.
      passwordForm.reset()
      window.setTimeout(async () => {
        await logoutMutation.mutateAsync()
        navigate("/login")
      }, 1500)
    } catch (err) {
      // 422 from the BE means "current password incorrect" — surface a
      // dedicated message for that path; everything else falls back to
      // the parsed server message.
      if (err instanceof HttpError && err.status === 422) {
        setPasswordError(t("settings:profile.password.incorrectCurrent"))
        return
      }
      setPasswordError(parseServerError(err, t("settings:profile.password.errorGeneric")))
    }
  }

  return (
    <>
      <RouteTitle title={t("settings:profile.edit.title")} />
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-8" data-testid="edit-profile-page">
        <div className="space-y-1">
          <Link
            to="/profile"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("settings:profile.title")}
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("settings:profile.edit.title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("settings:profile.edit.subtitle")}</p>
        </div>

        <ComingSoonBanner surface="profilePhoto" />

        <form
          className="space-y-4 rounded-xl border border-border bg-card p-5"
          onSubmit={profileForm.handleSubmit(onProfileSubmit)}
          noValidate
        >
          <div className="space-y-1.5">
            <Label htmlFor="profile-name">{t("auth:fields.name")}</Label>
            <div className="relative">
              <User
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="profile-name"
                autoComplete="name"
                placeholder={t("auth:fields.namePlaceholder")}
                className="pl-9"
                disabled={updateMutation.isPending}
                aria-invalid={!!profileForm.formState.errors.name}
                data-testid="profile-name-input"
                {...profileForm.register("name")}
              />
            </div>
            {profileForm.formState.errors.name ? (
              <p className="text-xs text-destructive" data-testid="profile-name-error">
                {t(profileForm.formState.errors.name.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="profile-email">{t("auth:fields.email")}</Label>
            <div className="relative">
              <Mail
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="profile-email"
                type="email"
                value={user?.email ?? ""}
                readOnly
                disabled
                className="pl-9 opacity-70"
              />
            </div>
            <p className="text-[11px] text-muted-foreground">
              {t("settings:account.email")} — {t("settings:profile.edit.subtitle")}
            </p>
          </div>

          {(groups?.length ?? 0) > 0 ? (
            <div className="space-y-1.5">
              <Label htmlFor="profile-default-group">{t("settings:profile.defaultGroup")}</Label>
              <select
                id="profile-default-group"
                disabled={updateMutation.isPending}
                data-testid="profile-default-group-select"
                className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-xs transition-colors outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:cursor-not-allowed disabled:opacity-50"
                {...profileForm.register("defaultGroupId")}
              >
                <option value="">{t("settings:profile.noGroupSelection")}</option>
                {groups?.map((g) => (
                  <option key={g.id} value={g.id}>
                    {g.name}
                  </option>
                ))}
              </select>
              <p className="text-[11px] text-muted-foreground">
                {t("settings:profile.defaultGroupHelp")}
              </p>
            </div>
          ) : null}

          {profileError ? (
            <Alert variant="destructive" data-testid="profile-server-error">
              <AlertDescription>{profileError}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex justify-end gap-2 pt-2">
            <Button asChild variant="ghost" type="button">
              <Link to="/profile">{t("settings:profile.edit.cancel")}</Link>
            </Button>
            <Button
              type="submit"
              className="gap-2"
              disabled={updateMutation.isPending}
              data-testid="profile-save"
            >
              {updateMutation.isPending
                ? t("settings:profile.edit.saving")
                : t("settings:profile.edit.save")}
              {!updateMutation.isPending ? <ArrowRight className="size-4" /> : null}
            </Button>
          </div>
        </form>

        <form
          className="space-y-4 rounded-xl border border-border bg-card p-5"
          onSubmit={passwordForm.handleSubmit(onPasswordSubmit)}
          noValidate
          data-testid="change-password-form"
        >
          <div className="flex items-start gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/60">
              <Lock className="size-4 text-muted-foreground" aria-hidden="true" />
            </div>
            <div className="space-y-0.5">
              <h2 className="text-base font-semibold">{t("settings:profile.password.title")}</h2>
              <p className="text-sm text-muted-foreground">
                {t("settings:profile.password.subtitle")}
              </p>
            </div>
          </div>

          {passwordSuccess ? (
            <Alert data-testid="password-change-success">
              <CheckCircle2 aria-hidden="true" />
              <AlertDescription>
                <strong>{t("settings:profile.password.successTitle")}</strong>
                {" — "}
                {t("settings:profile.password.successBody")}
              </AlertDescription>
            </Alert>
          ) : (
            <>
              <div className="space-y-1.5">
                <Label htmlFor="current-password">
                  {t("settings:profile.password.currentLabel")}
                </Label>
                <PasswordInput
                  id="current-password"
                  autoComplete="current-password"
                  hideLockIcon
                  disabled={changePasswordMutation.isPending}
                  aria-invalid={!!passwordForm.formState.errors.currentPassword}
                  data-testid="current-password"
                  {...passwordForm.register("currentPassword")}
                />
                {passwordForm.formState.errors.currentPassword ? (
                  <p className="text-xs text-destructive" data-testid="current-password-error">
                    {t(passwordForm.formState.errors.currentPassword.message ?? "")}
                  </p>
                ) : null}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="new-password">{t("settings:profile.password.newLabel")}</Label>
                <PasswordInput
                  id="new-password"
                  autoComplete="new-password"
                  hideLockIcon
                  disabled={changePasswordMutation.isPending}
                  aria-invalid={!!passwordForm.formState.errors.newPassword}
                  data-testid="new-password"
                  {...passwordForm.register("newPassword")}
                />
                {passwordForm.formState.errors.newPassword ? (
                  <p className="text-xs text-destructive" data-testid="new-password-error">
                    {t(passwordForm.formState.errors.newPassword.message ?? "")}
                  </p>
                ) : null}
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="confirm-password">
                  {t("settings:profile.password.confirmLabel")}
                </Label>
                <PasswordInput
                  id="confirm-password"
                  autoComplete="new-password"
                  hideLockIcon
                  disabled={changePasswordMutation.isPending}
                  aria-invalid={!!passwordForm.formState.errors.confirmPassword}
                  data-testid="confirm-password"
                  {...passwordForm.register("confirmPassword")}
                />
                {passwordForm.formState.errors.confirmPassword ? (
                  <p className="text-xs text-destructive" data-testid="confirm-password-error">
                    {t(passwordForm.formState.errors.confirmPassword.message ?? "")}
                  </p>
                ) : null}
              </div>

              {passwordError ? (
                <Alert variant="destructive" data-testid="password-server-error">
                  <AlertDescription>{passwordError}</AlertDescription>
                </Alert>
              ) : null}

              <div className="flex justify-end pt-2">
                <Button
                  type="submit"
                  variant="destructive"
                  className="gap-2"
                  disabled={changePasswordMutation.isPending}
                  data-testid="change-password-submit"
                >
                  {changePasswordMutation.isPending
                    ? t("settings:profile.password.submitting")
                    : t("settings:profile.password.submit")}
                </Button>
              </div>
            </>
          )}
        </form>
      </div>
    </>
  )
}
