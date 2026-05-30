import { useEffect, useMemo, useState } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router-dom"
import { ArrowLeft, ArrowRight, CheckCircle2, Lock, Mail, User } from "lucide-react"

import { ComingSoonBanner } from "@/components/coming-soon"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { PasswordStrengthMeter } from "@/components/auth/PasswordStrengthMeter"
import { SetPasswordForm } from "@/components/auth/SetPasswordForm"
import { FieldError } from "@/components/FieldError"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useAuth } from "@/features/auth/AuthContext"
import {
  useChangePassword,
  useHasPassword,
  useLogout,
  useUpdateProfile,
} from "@/features/auth/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroups } from "@/features/group/hooks"
import { withGroupQuery } from "@/lib/group-aware-url"
import {
  changePasswordSchema,
  profileEditSchema,
  type ChangePasswordInput,
  type ProfileEditInput,
} from "@/features/auth/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { HttpError } from "@/lib/http"
import { classifyServerError, type ClassifiedServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// /profile/edit — two side-by-side forms in one page: profile fields
// (name + default group) + password change. Email is intentionally
// read-only because the backend ignores it on PUT /auth/me.
export function EditProfilePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const { data: groups } = useGroups()
  const { currentGroup } = useCurrentGroup()
  const toast = useAppToast()
  const updateMutation = useUpdateProfile()
  const changePasswordMutation = useChangePassword()
  const logoutMutation = useLogout()
  // #1394: OAuth-only users (provisioned with empty password_hash) see
  // the Set-Password form instead of the regular Change Password card.
  // Defaults to true while the BE wire field is absent, so existing users
  // never see the wrong shape.
  const hasPassword = useHasPassword()

  const [profileError, setProfileError] = useState<ClassifiedServerError | null>(null)
  const [profileSaved, setProfileSaved] = useState(false)
  const [passwordError, setPasswordError] = useState<ClassifiedServerError | null>(null)
  const [passwordSuccess, setPasswordSuccess] = useState(false)
  // Password change form is collapsed by default — most page visits are for
  // the name/group fields above. The toggle expands it; e2e specs use the
  // legacy `.password-toggle` / `.password-form` classes for the same.
  const [passwordOpen, setPasswordOpen] = useState(false)

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

  const newPasswordValue = passwordForm.watch("newPassword") ?? ""
  // zxcvbn flags passwords derived from the user's name/email as weak.
  const strengthInputs = useMemo(
    () => [user?.name ?? "", user?.email ?? ""].map((v) => v.trim()).filter(Boolean),
    [user?.name, user?.email]
  )

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

  // Drop stale server errors when the user starts editing again. The
  // success banner is intentionally NOT auto-dismissed on edit: the e2e
  // suite (and most users) rely on it staying visible after a save until
  // the next save attempt explicitly resets it.
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
    setProfileSaved(false)
    try {
      // Only send `default_group_id` when it actually changed from the
      // user's saved value. The BE rejects with 400 if the id points at
      // a group the user is no longer a member of (e.g. stale picks left
      // by upstream tests); rounding the no-op case to `undefined` keeps
      // a "save the name" action from accidentally tripping that rule.
      // Under the #1592 invariant we never send null when the user has
      // memberships — the selector below has no "no default" option.
      const currentDefault = user?.default_group_id ?? ""
      const nextDefault = values.defaultGroupId ?? ""
      const defaultChanged = nextDefault !== currentDefault && nextDefault !== ""
      await updateMutation.mutateAsync({
        name: values.name.trim(),
        ...(defaultChanged ? { default_group_id: nextDefault } : {}),
      })
      toast.success(t("settings:profile.edit.successToast"))
      // Inline success banner so the user gets unambiguous in-page
      // confirmation; previously we navigated to /profile on save, but the
      // e2e suite (and most users) expect to stay on the edit page after a
      // save and see a "Profile updated" banner.
      setProfileSaved(true)
    } catch (err) {
      setProfileError(classifyServerError(err, t("settings:profile.edit.errorGeneric")))
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
      // to /login so the user re-authenticates with the new password. The
      // logout call is wrapped in a try/finally so a failed POST /auth/logout
      // (network blip, server error) never strands the user on a "succeeded"
      // page; navigation to /login fires regardless. Catching the rejection
      // also keeps it from surfacing as an unhandled promise rejection.
      passwordForm.reset()
      window.setTimeout(() => {
        void (async () => {
          try {
            await logoutMutation.mutateAsync()
          } catch (logoutErr) {
            console.warn("[EditProfile] Logout after password change failed:", logoutErr)
          } finally {
            navigate("/login")
          }
        })()
      }, 1500)
    } catch (err) {
      // 422 from the BE means "current password incorrect" — surface a
      // dedicated message for that path; everything else falls back to
      // the classified server error. The 422 is a user-fix case, so it's
      // a `validation` kind (no Retry affordance).
      if (err instanceof HttpError && err.status === 422) {
        setPasswordError({
          kind: "validation",
          message: t("settings:profile.password.incorrectCurrent"),
        })
        return
      }
      setPasswordError(classifyServerError(err, t("settings:profile.password.errorGeneric")))
    }
  }

  return (
    <>
      <RouteTitle title={t("settings:profile.edit.title")} />
      <Page width="narrow" className="gap-8" data-testid="edit-profile-page">
        <PageHeader
          size="detail"
          title={t("settings:profile.edit.title")}
          subtitle={t("settings:profile.edit.subtitle")}
          backLink={
            <Link
              to={withGroupQuery("/profile", currentGroup?.slug)}
              className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              {t("settings:profile.title")}
            </Link>
          }
        />

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
                aria-describedby={
                  profileForm.formState.errors.name ? "profile-name-error" : undefined
                }
                data-testid="profile-name-input"
                {...profileForm.register("name")}
              />
            </div>
            <FieldError
              id="profile-name-error"
              testId="profile-name-error"
              message={profileForm.formState.errors.name?.message}
            />
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
              {t("settings:profile.edit.emailReadOnlyHelp")}
            </p>
          </div>

          {(groups?.length ?? 0) > 0 ? (
            <div className="space-y-1.5">
              <Label htmlFor="profile-default-group">{t("settings:profile.defaultGroup")}</Label>
              <Controller
                control={profileForm.control}
                name="defaultGroupId"
                render={({ field }) => (
                  // Radix Select is the blessed form dropdown (mirrors the
                  // mock's SettingsView). Empty value → placeholder; the
                  // submit handler still treats "" as "no change".
                  <Select
                    value={field.value || undefined}
                    onValueChange={field.onChange}
                    disabled={updateMutation.isPending}
                  >
                    <SelectTrigger
                      id="profile-default-group"
                      ref={field.ref}
                      className="w-full"
                      data-testid="profile-default-group-select"
                    >
                      <SelectValue placeholder={t("settings:profile.defaultGroup")} />
                    </SelectTrigger>
                    <SelectContent>
                      {groups?.map((g) => (
                        <SelectItem key={g.id} value={g.id ?? ""}>
                          {g.name}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                )}
              />
              <p className="text-[11px] text-muted-foreground">
                {t("settings:profile.defaultGroupHelp")}
              </p>
            </div>
          ) : null}

          <ServerErrorBanner
            error={profileError}
            className="error-banner"
            testId="profile-server-error"
          />

          {profileSaved ? (
            <Alert className="success-banner" data-testid="profile-save-success">
              <CheckCircle2 aria-hidden="true" />
              <AlertDescription>{t("settings:profile.edit.savedBanner")}</AlertDescription>
            </Alert>
          ) : null}

          <div className="flex justify-end gap-2 pt-2">
            <Button asChild variant="ghost" type="button">
              <Link to={withGroupQuery("/profile", currentGroup?.slug)}>
                {t("settings:profile.edit.cancel")}
              </Link>
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

        {hasPassword ? (
          <button
            type="button"
            onClick={() => setPasswordOpen((prev) => !prev)}
            aria-expanded={passwordOpen}
            data-testid="password-toggle"
            className="password-toggle inline-flex w-fit items-center gap-1.5 text-sm font-medium text-foreground hover:text-primary"
          >
            <Lock className="size-4" aria-hidden="true" />
            {passwordOpen
              ? t("settings:profile.password.hide")
              : t("settings:profile.password.show")}
          </button>
        ) : (
          // OAuth-only users see the Set-Password form directly — no
          // toggle, because there's no "current password" path that
          // would compete with it. #1394.
          <SetPasswordForm />
        )}

        {hasPassword && passwordOpen ? (
          <form
            className="password-form space-y-4 rounded-xl border border-border bg-card p-5"
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
              <Alert className="success-banner" data-testid="password-change-success">
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
                    aria-describedby={
                      passwordForm.formState.errors.currentPassword
                        ? "current-password-error"
                        : undefined
                    }
                    data-testid="current-password"
                    {...passwordForm.register("currentPassword")}
                  />
                  <FieldError
                    id="current-password-error"
                    testId="current-password-error"
                    message={passwordForm.formState.errors.currentPassword?.message}
                  />
                </div>

                <div className="space-y-1.5">
                  <Label htmlFor="new-password">{t("settings:profile.password.newLabel")}</Label>
                  <PasswordInput
                    id="new-password"
                    autoComplete="new-password"
                    hideLockIcon
                    disabled={changePasswordMutation.isPending}
                    aria-invalid={!!passwordForm.formState.errors.newPassword}
                    aria-describedby={
                      passwordForm.formState.errors.newPassword ? "new-password-error" : undefined
                    }
                    data-testid="new-password"
                    {...passwordForm.register("newPassword")}
                  />
                  <PasswordStrengthMeter
                    password={newPasswordValue}
                    userInputs={strengthInputs}
                    testId="change-password-strength"
                  />
                  <FieldError
                    id="new-password-error"
                    testId="new-password-error"
                    message={passwordForm.formState.errors.newPassword?.message}
                  />
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
                    aria-describedby={
                      passwordForm.formState.errors.confirmPassword
                        ? "confirm-password-error"
                        : undefined
                    }
                    data-testid="confirm-password"
                    {...passwordForm.register("confirmPassword")}
                  />
                  <FieldError
                    id="confirm-password-error"
                    testId="confirm-password-error"
                    message={passwordForm.formState.errors.confirmPassword?.message}
                  />
                </div>

                <ServerErrorBanner
                  error={passwordError}
                  className="error-banner"
                  testId="password-server-error"
                />

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
        ) : null}
      </Page>
    </>
  )
}
