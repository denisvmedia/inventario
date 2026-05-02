import { useEffect, useMemo, useState } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, Navigate, useNavigate, useParams } from "react-router-dom"
import { ArrowLeft, ArrowRight, LogOut, Trash2, Users } from "lucide-react"

import { IconPicker } from "@/components/groups/IconPicker"
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
import { useAuth } from "@/features/auth/AuthContext"
import {
  useDeleteGroup,
  useGroup,
  useLeaveGroup,
  useMembers,
  useUpdateGroup,
} from "@/features/group/hooks"
import {
  deleteGroupSchema,
  updateGroupSchema,
  type DeleteGroupInput,
  type UpdateGroupInput,
} from "@/features/group/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { HttpError } from "@/lib/http"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// /groups/:groupId/settings — admin panel: rename / icon, currency
// (read-only, immutable), members link, leave-group, danger-zone
// delete with typed confirm-word + password.
//
// Group `id` (UUID) is the path key here, not the slug — slugs are
// random and not in URLs that admin tools reach for. The members
// section lives at /g/{slug}/members so we link there using the
// group's slug.
export function GroupSettingsPage() {
  const { groupId } = useParams<{ groupId: string }>()
  if (!groupId) return <Navigate to="/no-group" replace />
  return <GroupSettingsBody groupId={groupId} />
}

function GroupSettingsBody({ groupId }: { groupId: string }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { user } = useAuth()
  const groupQuery = useGroup(groupId)
  const membersQuery = useMembers(groupId)
  const updateMutation = useUpdateGroup()
  const leaveMutation = useLeaveGroup()
  const toast = useAppToast()
  const [serverError, setServerError] = useState<string | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const myMembership = useMemo(
    () => membersQuery.data?.find((m) => m.member_user_id === user?.id),
    [membersQuery.data, user?.id]
  )
  const isAdmin = myMembership?.role === "admin"
  const adminCount = useMemo(
    () => membersQuery.data?.filter((m) => m.role === "admin").length ?? 0,
    [membersQuery.data]
  )
  const isLastAdmin = isAdmin && adminCount === 1

  const form = useForm<UpdateGroupInput>({
    resolver: zodResolver(updateGroupSchema),
    defaultValues: { name: "", icon: "" },
  })

  // Reset the form once the group lands. useForm reads defaults at
  // mount; a hard refresh races the GET /groups/:id round-trip.
  useEffect(() => {
    if (!groupQuery.data) return
    form.reset({
      name: groupQuery.data.name ?? "",
      icon: groupQuery.data.icon ?? "",
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupQuery.data?.id, groupQuery.data?.updated_at])

  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  if (groupQuery.isLoading) {
    return <div className="text-sm text-muted-foreground p-6">{t("groups:settings.title")}…</div>
  }
  if (groupQuery.isError || !groupQuery.data) {
    return (
      <Alert variant="destructive" className="max-w-xl mx-auto mt-6">
        <AlertDescription>{t("groups:settings.errorGeneric")}</AlertDescription>
      </Alert>
    )
  }

  const group = groupQuery.data

  async function onSave(values: UpdateGroupInput) {
    setServerError(null)
    try {
      await updateMutation.mutateAsync({
        groupId,
        patch: { name: values.name.trim(), icon: values.icon },
      })
      toast.success(t("groups:settings.saved"))
    } catch (err) {
      setServerError(parseServerError(err, t("groups:settings.errorGeneric")))
    }
  }

  async function onLeave() {
    try {
      await leaveMutation.mutateAsync({ groupId })
      toast.success(t("groups:settings.leaveSuccess"))
      navigate("/no-group")
    } catch (err) {
      toast.error(parseServerError(err, t("groups:settings.leaveError")))
    }
  }

  return (
    <>
      <RouteTitle title={t("groups:settings.title")} />
      <div
        className="mx-auto flex w-full max-w-2xl flex-col gap-8"
        data-testid="group-settings-page"
      >
        <div className="space-y-1">
          {/* Back returns to the previous location rather than hard-coding
              /profile or /no-group: this page is reachable from the
              GroupSelector dropdown, the members page, and onboarding;
              any single destination would be wrong for some of them. */}
          <button
            type="button"
            onClick={() => navigate(-1)}
            data-testid="group-settings-back"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("common:actions.back")}
          </button>
          <h1 className="text-2xl font-semibold tracking-tight">
            {group.icon ? <span aria-hidden="true">{group.icon} </span> : null}
            {group.name}
          </h1>
          <p className="text-sm text-muted-foreground">{t("groups:settings.subtitle")}</p>
        </div>

        {/* Identity (admins only — non-admins see read-only summary). */}
        {isAdmin ? (
          <form
            className="space-y-4 rounded-xl border border-border bg-card p-5"
            onSubmit={form.handleSubmit(onSave)}
            noValidate
          >
            <div className="space-y-1.5">
              <Label htmlFor="settings-group-name">{t("groups:settings.nameLabel")}</Label>
              <Input
                id="settings-group-name"
                maxLength={100}
                disabled={updateMutation.isPending}
                aria-invalid={!!form.formState.errors.name}
                data-testid="settings-name-input"
                {...form.register("name")}
              />
              {form.formState.errors.name ? (
                <p className="text-xs text-destructive" data-testid="settings-name-error">
                  {t(form.formState.errors.name.message ?? "")}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label>{t("groups:settings.iconLabel")}</Label>
              <Controller
                control={form.control}
                name="icon"
                render={({ field }) => (
                  <IconPicker
                    value={field.value}
                    onChange={field.onChange}
                    disabled={updateMutation.isPending}
                    testId="group-settings-icon-picker"
                  />
                )}
              />
              {form.formState.errors.icon ? (
                <p className="text-xs text-destructive" data-testid="settings-icon-error">
                  {t(form.formState.errors.icon.message ?? "")}
                </p>
              ) : null}
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="settings-group-slug">{t("groups:settings.slugLabel")}</Label>
              <Input
                id="settings-group-slug"
                value={group.slug ?? ""}
                readOnly
                disabled
                className="font-mono text-xs"
              />
              <p className="text-[11px] text-muted-foreground">{t("groups:settings.slugHelp")}</p>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="settings-group-currency">{t("groups:settings.currencyLabel")}</Label>
              <Input
                id="settings-group-currency"
                value={group.main_currency ?? "—"}
                readOnly
                disabled
                className="font-mono uppercase"
              />
              <p className="text-[11px] text-muted-foreground">
                {t("groups:settings.currencyImmutableHelp")}
              </p>
            </div>

            {serverError ? (
              <Alert variant="destructive" data-testid="settings-server-error">
                <AlertDescription>{serverError}</AlertDescription>
              </Alert>
            ) : null}

            <div className="flex justify-end pt-2">
              <Button
                type="submit"
                className="gap-2"
                disabled={updateMutation.isPending}
                data-testid="settings-save"
              >
                {updateMutation.isPending ? t("groups:settings.saving") : t("groups:settings.save")}
                {!updateMutation.isPending ? <ArrowRight className="size-4" /> : null}
              </Button>
            </div>
          </form>
        ) : (
          <div className="rounded-xl border border-border bg-muted/30 p-5 text-sm text-muted-foreground">
            {t("members:adminOnlyHelp")}
          </div>
        )}

        {/* Members shortcut — works for non-admins too (they see the list,
            actions are gated inside MembersPage). */}
        {group.slug ? (
          <div className="rounded-xl border border-border bg-card p-4 flex items-center justify-between">
            <div>
              <p className="text-sm font-semibold">{t("groups:settings.membersLink")}</p>
              <p className="text-xs text-muted-foreground">{t("members:subtitle")}</p>
            </div>
            <Button asChild variant="outline" size="sm" className="gap-1.5">
              <Link
                to={`/g/${encodeURIComponent(group.slug)}/members`}
                data-testid="settings-members-link"
              >
                <Users className="size-3.5" aria-hidden="true" />
                {t("groups:settings.membersLink")}
              </Link>
            </Button>
          </div>
        ) : null}

        {/* Leave-group panel. The BE rejects "leave as last admin" with
            a 422; we mirror that as a disabled button + explanation so
            the user doesn't waste a round-trip. */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-3">
          <div>
            <p className="text-sm font-semibold">{t("groups:settings.leaveTitle")}</p>
            <p
              className="text-xs text-muted-foreground mt-0.5"
              data-testid={isLastAdmin ? "last-admin-notice" : undefined}
            >
              {isLastAdmin
                ? t("groups:settings.leaveLastAdmin")
                : t("groups:settings.leaveDescription")}
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-1.5 text-amber-600 border-amber-500/40 hover:bg-amber-500/10"
            // Also gate on the membership query: while it's loading,
            // adminCount defaults to 0 and isLastAdmin is false, so the
            // last-admin guard would briefly let the click through. The
            // BE rejects with 422 anyway, but the UX is cleaner if the
            // button stays unclickable until we know the answer.
            disabled={membersQuery.isLoading || isLastAdmin || leaveMutation.isPending}
            aria-disabled={isLastAdmin || undefined}
            title={isLastAdmin ? t("groups:settings.leaveLastAdminTitle") : undefined}
            onClick={onLeave}
            data-testid="leave-group-btn"
          >
            <LogOut className="size-3.5" aria-hidden="true" />
            {t("groups:settings.leaveCta")}
          </Button>
        </div>

        {/* Danger zone (admins only). The dialog form lives below. */}
        {isAdmin ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-5 space-y-3">
            <p className="text-sm font-semibold text-destructive">
              {t("groups:settings.dangerTitle")}
            </p>
            <p className="text-xs text-muted-foreground">
              {t("groups:settings.dangerDescription")}
            </p>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5 text-destructive border-destructive/40 hover:bg-destructive/10"
              onClick={() => setDeleteOpen(true)}
              data-testid="delete-group-open"
            >
              <Trash2 className="size-3.5" aria-hidden="true" />
              {t("groups:settings.deleteCta")}
            </Button>
          </div>
        ) : null}

        <DeleteGroupDialog
          open={deleteOpen}
          onOpenChange={setDeleteOpen}
          group={{ id: groupId, name: group.name ?? "" }}
        />
      </div>
    </>
  )
}

function DeleteGroupDialog({
  open,
  onOpenChange,
  group,
}: {
  open: boolean
  onOpenChange: (next: boolean) => void
  group: { id: string; name: string }
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const deleteMutation = useDeleteGroup()
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<DeleteGroupInput>({
    resolver: zodResolver(deleteGroupSchema),
    defaultValues: { confirmWord: "", password: "" },
  })

  // Reset the form whenever the dialog opens — we don't want a stale
  // password lingering across re-opens.
  useEffect(() => {
    if (open) {
      form.reset({ confirmWord: "", password: "" })
      setServerError(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  async function onSubmit(values: DeleteGroupInput) {
    setServerError(null)
    // Client-side guard: confirm_word must match the group's current
    // name. The server checks this too, but catching it here saves a
    // round-trip on the very common typo case.
    if (values.confirmWord.trim() !== group.name) {
      form.setError("confirmWord", { message: "groups:validation.confirmWordMismatch" })
      return
    }
    try {
      await deleteMutation.mutateAsync({
        groupId: group.id,
        confirm_word: values.confirmWord.trim(),
        password: values.password,
      })
      onOpenChange(false)
      navigate("/no-group")
    } catch (err) {
      // The client-side guard above already rejected mismatched confirm-words
      // before the request was sent, so a 422 from the BE here can only mean
      // the password was wrong. Surface that on the password field directly
      // (#1289 Gap A: wrong password must be distinguishable from wrong
      // confirm-word in the UX, not just at the handler).
      if (err instanceof HttpError && err.status === 422) {
        // Pre-translate so the i18next-cli extractor sees the key statically
        // (the form renderer feeds errors.password.message back through t(),
        // but t() of an already-translated string is a no-op lookup that
        // returns the same string — which is what we want here).
        form.setError("password", {
          message: t("groups:settings.deleteDialog.wrongPassword"),
        })
        return
      }
      setServerError(parseServerError(err, t("groups:settings.deleteDialog.errorGeneric")))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="delete-group-dialog">
        <DialogHeader>
          <DialogTitle>{t("groups:settings.deleteDialog.title", { name: group.name })}</DialogTitle>
          <DialogDescription>{t("groups:settings.deleteDialog.body")}</DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="delete-group-name">
              {t("groups:settings.deleteDialog.confirmWordLabel")}
            </Label>
            <Input
              id="delete-group-name"
              autoComplete="off"
              placeholder={group.name}
              disabled={deleteMutation.isPending}
              aria-invalid={!!form.formState.errors.confirmWord}
              data-testid="delete-confirm-word"
              {...form.register("confirmWord")}
            />
            {form.formState.errors.confirmWord ? (
              <p className="text-xs text-destructive" data-testid="delete-confirm-word-error">
                {t(form.formState.errors.confirmWord.message ?? "")}
              </p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="delete-group-password">
              {t("groups:settings.deleteDialog.passwordLabel")}
            </Label>
            <Input
              id="delete-group-password"
              type="password"
              autoComplete="current-password"
              disabled={deleteMutation.isPending}
              aria-invalid={!!form.formState.errors.password}
              data-testid="delete-password"
              {...form.register("password")}
            />
            {form.formState.errors.password ? (
              <p className="text-xs text-destructive" data-testid="delete-password-error">
                {t(form.formState.errors.password.message ?? "")}
              </p>
            ) : null}
          </div>
          {serverError ? (
            <Alert variant="destructive" data-testid="delete-server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={deleteMutation.isPending}
            >
              {t("groups:settings.deleteDialog.cancel")}
            </Button>
            <Button
              type="submit"
              variant="destructive"
              disabled={deleteMutation.isPending}
              data-testid="delete-group-submit"
            >
              {deleteMutation.isPending
                ? t("groups:settings.deleteDialog.deleting")
                : t("groups:settings.deleteDialog.confirm")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
