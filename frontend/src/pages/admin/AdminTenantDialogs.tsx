import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

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
import { Textarea } from "@/components/ui/textarea"
import type { AdminTenant } from "@/features/admin/api"
import {
  useCreateAdminTenant,
  useDeleteAdminTenant,
  useReactivateAdminTenant,
  useSuspendAdminTenant,
  useUpdateAdminTenant,
} from "@/features/admin/hooks"
import {
  createTenantSchema,
  editTenantSchema,
  type CreateTenantInput,
  type EditTenantInput,
} from "@/features/admin/schemas"
import { applyServerFieldErrors } from "@/lib/form-errors"
import { getServerErrorCode } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// The BE caps the suspend / reactivate reason at 500 characters, counted in
// code points, as it does for a user block.
const REASON_MAX = 500

// The typed codes the tenant writes answer with, mapped to the flat i18n
// key segment under admin:tenantActions.errors (i18n keys cannot hold the
// dots of the BE codes).
const TENANT_ERROR_KEY: Record<string, string> = {
  "tenant.slug_taken": "slugTaken",
  "tenant.domain_taken": "domainTaken",
  "admin.tenant.reason_required": "reasonRequired",
  "admin.tenant.reason_too_long": "reasonTooLong",
  "admin.tenant.serves_this_host": "servesThisHost",
  "admin.tenant.confirmation_mismatch": "confirmationMismatch",
  "admin.role_required": "roleRequired",
}

function tenantErrorKey(err: unknown): string {
  const code = getServerErrorCode(err)
  return (code && TENANT_ERROR_KEY[code]) || "generic"
}

function ActionError({ errorKey, testId }: { errorKey: string | null; testId: string }) {
  const { t } = useTranslation("admin")
  if (!errorKey) return null
  return (
    <Alert variant="destructive" data-testid={testId}>
      <AlertDescription>{t(`tenantActions.errors.${errorKey}`)}</AlertDescription>
    </Alert>
  )
}

const FORM_FIELDS = ["name", "slug", "domain"] as const

type TenantFormProps =
  | {
      mode: "create"
      open: boolean
      onOpenChange: (open: boolean) => void
      onSaved: (tenant: AdminTenant) => void
      tenant?: undefined
    }
  | {
      mode: "edit"
      open: boolean
      onOpenChange: (open: boolean) => void
      onSaved?: (tenant: AdminTenant) => void
      tenant: AdminTenant
    }

// TenantFormDialog creates a tenant or edits the name, slug and domain of
// one. The status is not on the form: it changes through Suspend and
// Reactivate, which ask for a reason the audit log keeps.
export function TenantFormDialog(props: TenantFormProps) {
  const { mode, open, onOpenChange, tenant } = props
  const { t } = useTranslation("admin")
  const createMutation = useCreateAdminTenant()
  const updateMutation = useUpdateAdminTenant(tenant?.id ?? "")
  const pending = createMutation.isPending || updateMutation.isPending
  const [errorKey, setErrorKey] = useState<string | null>(null)

  const form = useForm<CreateTenantInput | EditTenantInput>({
    resolver: zodResolver(mode === "create" ? createTenantSchema : editTenantSchema),
    defaultValues: { name: "", slug: "", domain: "" },
  })

  // Start from the tenant's current values (or blank) every time the
  // dialog opens, so a cancelled edit does not linger.
  useEffect(() => {
    if (open) {
      form.reset({
        name: tenant?.name ?? "",
        slug: tenant?.slug ?? "",
        domain: tenant?.domain ?? "",
      })
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setErrorKey(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- re-run on open only; form is a stable RHF handle
  }, [open])

  function placeServerError(err: unknown) {
    const key = tenantErrorKey(err)
    // A taken slug or domain belongs next to its field.
    if (key === "slugTaken" || key === "domainTaken") {
      const field = key === "slugTaken" ? "slug" : "domain"
      form.setError(field, { message: `admin:tenantActions.errors.${key}` })
      return
    }
    const placed = applyServerFieldErrors(err, form.setError, { fields: FORM_FIELDS })
    if (placed && Object.keys(placed.unmapped).length === 0) return
    setErrorKey(key)
  }

  async function onSubmit(values: CreateTenantInput | EditTenantInput) {
    setErrorKey(null)
    try {
      if (mode === "create") {
        const created = await createMutation.mutateAsync({
          name: values.name,
          ...(values.slug ? { slug: values.slug } : {}),
          ...(values.domain ? { domain: values.domain } : {}),
        })
        toast.success(t("tenantActions.create.success", { name: created.name ?? "" }))
        onOpenChange(false)
        props.onSaved(created)
        return
      }
      // Send only what changed, so a rename does not rewrite the slug.
      const changes: { name?: string; slug?: string; domain?: string } = {}
      if (values.name !== (tenant?.name ?? "")) changes.name = values.name
      if (values.slug !== (tenant?.slug ?? "")) changes.slug = values.slug
      if (values.domain !== (tenant?.domain ?? "")) changes.domain = values.domain
      const updated = await updateMutation.mutateAsync(changes)
      toast.success(t("tenantActions.edit.success", { name: updated.name ?? "" }))
      onOpenChange(false)
      props.onSaved?.(updated)
    } catch (err) {
      placeServerError(err)
    }
  }

  const errors = form.formState.errors
  const isCreate = mode === "create"

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!pending) onOpenChange(next)
      }}
    >
      <DialogContent className="sm:max-w-md" data-testid="admin-tenant-form-dialog">
        <DialogHeader>
          <DialogTitle>
            {isCreate ? t("tenantActions.create.title") : t("tenantActions.edit.title")}
          </DialogTitle>
          <DialogDescription>
            {isCreate
              ? t("tenantActions.create.body")
              : t("tenantActions.edit.body", { name: tenant?.name ?? "" })}
          </DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-tenant-name">{t("tenantForm.name.label")}</Label>
            <Input
              id="admin-tenant-name"
              autoComplete="off"
              disabled={pending}
              aria-invalid={!!errors.name}
              data-testid="admin-tenant-name"
              {...form.register("name")}
            />
            {errors.name?.message ? (
              <p className="text-xs text-destructive" data-testid="admin-tenant-name-error">
                {t(errors.name.message)}
              </p>
            ) : null}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-tenant-slug">{t("tenantForm.slug.label")}</Label>
            <Input
              id="admin-tenant-slug"
              autoComplete="off"
              className="font-mono"
              disabled={pending}
              aria-invalid={!!errors.slug}
              data-testid="admin-tenant-slug"
              {...form.register("slug")}
            />
            {errors.slug?.message ? (
              <p className="text-xs text-destructive" data-testid="admin-tenant-slug-error">
                {t(errors.slug.message)}
              </p>
            ) : (
              <p className="text-xs text-muted-foreground">
                {isCreate ? t("tenantForm.slug.hintCreate") : t("tenantForm.slug.hintEdit")}
              </p>
            )}
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-tenant-domain">{t("tenantForm.domain.label")}</Label>
            <Input
              id="admin-tenant-domain"
              autoComplete="off"
              placeholder="inventory.example.com"
              disabled={pending}
              aria-invalid={!!errors.domain}
              data-testid="admin-tenant-domain"
              {...form.register("domain")}
            />
            {errors.domain?.message ? (
              <p className="text-xs text-destructive" data-testid="admin-tenant-domain-error">
                {t(errors.domain.message)}
              </p>
            ) : (
              <p className="text-xs text-muted-foreground">{t("tenantForm.domain.hint")}</p>
            )}
          </div>

          <ActionError errorKey={errorKey} testId="admin-tenant-form-error" />

          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={pending}
            >
              {t("tenantActions.cancel")}
            </Button>
            <Button type="submit" disabled={pending} data-testid="admin-tenant-form-submit">
              {isCreate ? t("tenantActions.create.submit") : t("tenantActions.edit.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

// TenantStatusDialog suspends or reactivates a tenant. Both take a reason,
// as blocking and unblocking a user do; the audit row keeps it.
export function TenantStatusDialog({
  action,
  tenant,
  open,
  onOpenChange,
}: {
  action: "suspend" | "reactivate"
  tenant: AdminTenant
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation("admin")
  const tenantId = tenant.id ?? ""
  const suspendMutation = useSuspendAdminTenant(tenantId)
  const reactivateMutation = useReactivateAdminTenant(tenantId)
  const mutation = action === "suspend" ? suspendMutation : reactivateMutation
  const [reason, setReason] = useState("")
  const [errorKey, setErrorKey] = useState<string | null>(null)

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setReason("")
      setErrorKey(null)
    }
  }, [open])

  // Code points of the trimmed reason, the count the BE applies.
  const reasonLength = [...reason.trim()].length
  const reasonInvalid = reasonLength === 0 || reasonLength > REASON_MAX

  function handleConfirm() {
    setErrorKey(null)
    mutation.mutate(
      { reason: reason.trim() },
      {
        onSuccess: (updated) => {
          onOpenChange(false)
          const name = updated.name ?? ""
          toast.success(
            action === "suspend"
              ? t("tenantActions.suspend.success", { name })
              : t("tenantActions.reactivate.success", { name })
          )
        },
        onError: (err) => setErrorKey(tenantErrorKey(err)),
      }
    )
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!mutation.isPending) onOpenChange(next)
      }}
    >
      <DialogContent data-testid="admin-tenant-status-dialog">
        <DialogHeader>
          <DialogTitle>
            {action === "suspend"
              ? t("tenantActions.suspend.title", { name: tenant.name ?? "" })
              : t("tenantActions.reactivate.title", { name: tenant.name ?? "" })}
          </DialogTitle>
          <DialogDescription>
            {action === "suspend"
              ? t("tenantActions.suspend.body")
              : t("tenantActions.reactivate.body")}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="admin-tenant-status-reason">{t("tenantActions.reason.label")}</Label>
          <Textarea
            id="admin-tenant-status-reason"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            placeholder={t("tenantActions.reason.placeholder")}
            data-testid="admin-tenant-status-reason"
          />
          <p
            className={cn(
              "text-xs text-muted-foreground",
              reasonLength > REASON_MAX && "text-destructive"
            )}
          >
            {t("tenantActions.reason.counter", { current: reasonLength, max: REASON_MAX })}
          </p>
        </div>

        <ActionError errorKey={errorKey} testId="admin-tenant-status-error" />

        <DialogFooter className="gap-2">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={mutation.isPending}
          >
            {t("tenantActions.cancel")}
          </Button>
          <Button
            variant={action === "suspend" ? "destructive" : "default"}
            onClick={handleConfirm}
            disabled={mutation.isPending || reasonInvalid}
            data-testid="admin-tenant-status-confirm"
          >
            {action === "suspend"
              ? t("tenantActions.suspend.submit")
              : t("tenantActions.reactivate.submit")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// DeleteTenantDialog deletes a tenant once the operator has typed its slug,
// the confirmation `inventario tenants delete` asks for. The BE checks the
// slug again and refuses a mismatch.
export function DeleteTenantDialog({
  tenant,
  open,
  onOpenChange,
  onDeleted,
}: {
  tenant: AdminTenant
  open: boolean
  onOpenChange: (open: boolean) => void
  onDeleted: () => void
}) {
  const { t } = useTranslation("admin")
  const deleteMutation = useDeleteAdminTenant(tenant.id ?? "")
  const [typed, setTyped] = useState("")
  const [errorKey, setErrorKey] = useState<string | null>(null)
  const slug = tenant.slug ?? ""
  const matches = slug !== "" && typed.trim() === slug

  useEffect(() => {
    if (open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setTyped("")
      setErrorKey(null)
    }
  }, [open])

  function handleDelete() {
    if (!matches) return
    setErrorKey(null)
    deleteMutation.mutate(typed.trim(), {
      onSuccess: () => {
        onOpenChange(false)
        toast.success(t("tenantActions.delete.success", { name: tenant.name ?? "" }))
        onDeleted()
      },
      onError: (err) => setErrorKey(tenantErrorKey(err)),
    })
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!deleteMutation.isPending) onOpenChange(next)
      }}
    >
      <DialogContent data-testid="admin-tenant-delete-dialog">
        <DialogHeader>
          <DialogTitle>{t("tenantActions.delete.title", { name: tenant.name ?? "" })}</DialogTitle>
          <DialogDescription>{t("tenantActions.delete.body")}</DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-4"
          onSubmit={(event) => {
            event.preventDefault()
            handleDelete()
          }}
          noValidate
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-tenant-delete-slug">
              {t("tenantActions.delete.confirmLabel")}{" "}
              <span className="font-mono text-foreground">{slug}</span>
            </Label>
            <Input
              id="admin-tenant-delete-slug"
              autoComplete="off"
              className="font-mono"
              value={typed}
              onChange={(event) => setTyped(event.target.value)}
              disabled={deleteMutation.isPending}
              data-testid="admin-tenant-delete-slug"
            />
          </div>

          <ActionError errorKey={errorKey} testId="admin-tenant-delete-error" />

          <DialogFooter className="gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={deleteMutation.isPending}
            >
              {t("tenantActions.cancel")}
            </Button>
            <Button
              type="submit"
              variant="destructive"
              disabled={!matches || deleteMutation.isPending}
              data-testid="admin-tenant-delete-confirm"
            >
              {t("tenantActions.delete.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
