import { useId, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { ClipboardCopy, KeyRound, TriangleAlert } from "lucide-react"

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
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { API_TOKEN_SCOPES, type CreateAPITokenResponse } from "@/features/api-tokens/api"
import { useCreateAPIToken } from "@/features/api-tokens/hooks"
import {
  API_TOKEN_EXPIRY_PRESETS,
  createAPITokenSchema,
  expiresAtFor,
  type CreateAPITokenFormInput,
} from "@/features/api-tokens/schemas"
import { useHasPassword, useMFAStatus } from "@/features/auth/hooks"
import { useGroups } from "@/features/group/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { getServerErrorCode, parseServerError } from "@/lib/server-error"
import { withId } from "@/lib/with-id"

// Server error codes the form maps to its own copy. Anything else falls back
// to the server's message.
const KNOWN_ERROR_CODES = new Set([
  "api_token.name_required",
  "api_token.name_too_long",
  "api_token.invalid_scope",
  "api_token.expiry_in_past",
  "api_token.group_not_allowed",
  "api_token.limit_reached",
  "api_token.invalid_password",
  "api_token.invalid_mfa_code",
  "api_token.impersonation_forbidden",
])

// A sentinel for "every group" — Radix Select cannot hold an empty value.
const ALL_GROUPS = "__all__"

interface Props {
  open: boolean
  onOpenChange: (next: boolean) => void
}

// CreateAPITokenDialog collects the token settings, re-authenticates the
// user, and then shows the new token exactly once. The secret lives only in
// this component's state: closing the dialog unmounts it and the value is
// gone. It is never written to the query cache.
export function CreateAPITokenDialog({ open, onOpenChange }: Props) {
  // While the token is on screen a stray click outside the dialog must not
  // dismiss it: the value cannot be shown again. Escape and the buttons
  // still close it.
  const [revealing, setRevealing] = useState(false)
  const handleOpenChange = (next: boolean) => {
    if (!next) setRevealing(false)
    onOpenChange(next)
  }
  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        data-testid="api-token-create-dialog"
        className="sm:max-w-md"
        onInteractOutside={(e) => {
          if (revealing) e.preventDefault()
        }}
      >
        {open ? (
          <CreateFlow
            onRevealed={() => setRevealing(true)}
            onClose={() => handleOpenChange(false)}
          />
        ) : null}
      </DialogContent>
    </Dialog>
  )
}

function CreateFlow({ onRevealed, onClose }: { onRevealed: () => void; onClose: () => void }) {
  const [created, setCreated] = useState<CreateAPITokenResponse | null>(null)
  if (created?.secret) {
    return <RevealStep secret={created.secret} name={created.token?.name ?? ""} onDone={onClose} />
  }
  return (
    <CreateForm
      onCreated={(res) => {
        setCreated(res)
        onRevealed()
      }}
      onCancel={onClose}
    />
  )
}

function CreateForm({
  onCreated,
  onCancel,
}: {
  onCreated: (res: CreateAPITokenResponse) => void
  onCancel: () => void
}) {
  const { t } = useTranslation()
  const formId = useId()
  const hasPassword = useHasPassword()
  const mfaStatus = useMFAStatus()
  const mfaActive = mfaStatus.data?.state === "active"
  const { data: groups } = useGroups()
  const createMutation = useCreateAPIToken()
  const [codeMode, setCodeMode] = useState<"totp" | "backup">("totp")
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<CreateAPITokenFormInput>({
    resolver: zodResolver(
      createAPITokenSchema({ requirePassword: hasPassword, requireCode: mfaActive })
    ),
    defaultValues: {
      name: "",
      scope: "read_only",
      groupId: "",
      expiry: "90d",
      currentPassword: "",
      code: "",
    },
    mode: "onSubmit",
  })
  const { register, control, handleSubmit, formState } = form
  const { errors } = formState

  async function onSubmit(values: CreateAPITokenFormInput) {
    setServerError(null)
    const code = values.code.trim()
    try {
      const res = await createMutation.mutateAsync({
        name: values.name,
        scope: values.scope,
        group_id: values.groupId || undefined,
        expires_at: expiresAtFor(values.expiry),
        current_password: hasPassword ? values.currentPassword : undefined,
        totp_code: mfaActive && codeMode === "totp" ? code : undefined,
        backup_code: mfaActive && codeMode === "backup" ? code : undefined,
      })
      onCreated(res)
    } catch (err) {
      const errorCode = getServerErrorCode(err)
      if (errorCode && KNOWN_ERROR_CODES.has(errorCode)) {
        setServerError(t(`settings:apiTokens.errors.${errorCode.replace("api_token.", "")}`))
      } else {
        setServerError(parseServerError(err, t("settings:apiTokens.errors.generic")))
      }
    }
  }

  const pending = createMutation.isPending

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4" noValidate>
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          <KeyRound className="size-5" aria-hidden="true" />
          {t("settings:apiTokens.create.title")}
        </DialogTitle>
        <DialogDescription>{t("settings:apiTokens.create.description")}</DialogDescription>
      </DialogHeader>

      <div className="space-y-1.5">
        <Label htmlFor={`${formId}-name`}>{t("settings:apiTokens.create.name")}</Label>
        <Input
          id={`${formId}-name`}
          placeholder={t("settings:apiTokens.create.namePlaceholder")}
          autoComplete="off"
          disabled={pending}
          aria-invalid={errors.name ? true : undefined}
          data-testid="api-token-name"
          {...register("name")}
        />
        {errors.name?.message ? (
          <p className="text-xs text-destructive">{t(errors.name.message)}</p>
        ) : null}
      </div>

      <div className="space-y-1.5">
        <Label id={`${formId}-scope-label`}>{t("settings:apiTokens.create.scope")}</Label>
        <Controller
          control={control}
          name="scope"
          render={({ field }) => (
            <RadioGroup
              value={field.value}
              onValueChange={field.onChange}
              aria-labelledby={`${formId}-scope-label`}
              className="grid gap-2"
              disabled={pending}
              data-testid="api-token-scope"
            >
              {API_TOKEN_SCOPES.map((scope) => {
                const id = `${formId}-scope-${scope}`
                return (
                  <Label
                    key={scope}
                    htmlFor={id}
                    className="flex cursor-pointer items-start gap-3 rounded-md border border-input px-3 py-2 font-normal transition-colors has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/10"
                  >
                    <RadioGroupItem
                      id={id}
                      value={scope}
                      className="mt-0.5"
                      data-testid={`api-token-scope-${scope}`}
                    />
                    <span className="space-y-0.5">
                      <span className="block text-sm font-medium">
                        {t(`settings:apiTokens.scopes.${scope}`)}
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        {t(`settings:apiTokens.scopeDescriptions.${scope}`)}
                      </span>
                    </span>
                  </Label>
                )
              })}
            </RadioGroup>
          )}
        />
      </div>

      <div className="grid gap-4 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor={`${formId}-group`}>{t("settings:apiTokens.create.group")}</Label>
          <Controller
            control={control}
            name="groupId"
            render={({ field }) => (
              <Select
                value={field.value || ALL_GROUPS}
                onValueChange={(v) => field.onChange(v === ALL_GROUPS ? "" : v)}
                disabled={pending}
              >
                <SelectTrigger
                  id={`${formId}-group`}
                  ref={field.ref}
                  className="w-full"
                  data-testid="api-token-group"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={ALL_GROUPS}>
                    {t("settings:apiTokens.create.allGroups")}
                  </SelectItem>
                  {withId(groups).map((g) => (
                    <SelectItem key={g.id} value={g.id}>
                      {g.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor={`${formId}-expiry`}>{t("settings:apiTokens.create.expiry")}</Label>
          <Controller
            control={control}
            name="expiry"
            render={({ field }) => (
              <Select value={field.value} onValueChange={field.onChange} disabled={pending}>
                <SelectTrigger
                  id={`${formId}-expiry`}
                  ref={field.ref}
                  className="w-full"
                  data-testid="api-token-expiry"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {API_TOKEN_EXPIRY_PRESETS.map((preset) => (
                    <SelectItem key={preset} value={preset}>
                      {t(`settings:apiTokens.expiry.${preset}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
      </div>
      <p className="text-xs text-muted-foreground">{t("settings:apiTokens.create.groupHelp")}</p>

      {hasPassword || mfaActive ? (
        <div className="space-y-3 rounded-md border border-border bg-muted/30 p-3">
          <p className="text-xs text-muted-foreground">
            {t("settings:apiTokens.create.reauthHint")}
          </p>
          {hasPassword ? (
            <div className="space-y-1.5">
              <Label htmlFor={`${formId}-password`}>
                {t("settings:apiTokens.create.currentPassword")}
              </Label>
              <PasswordInput
                id={`${formId}-password`}
                autoComplete="current-password"
                disabled={pending}
                data-testid="api-token-password"
                {...register("currentPassword")}
              />
              {errors.currentPassword?.message ? (
                <p className="text-xs text-destructive">{t(errors.currentPassword.message)}</p>
              ) : null}
            </div>
          ) : null}
          {mfaActive ? (
            <div className="space-y-1.5">
              <Label htmlFor={`${formId}-code`}>
                {codeMode === "totp"
                  ? t("settings:privacy.mfa.disable.totpLabel")
                  : t("settings:privacy.mfa.disable.backupLabel")}
              </Label>
              <Input
                id={`${formId}-code`}
                inputMode={codeMode === "totp" ? "numeric" : "text"}
                autoComplete="one-time-code"
                disabled={pending}
                data-testid="api-token-code"
                {...register("code")}
              />
              {errors.code?.message ? (
                <p className="text-xs text-destructive">{t(errors.code.message)}</p>
              ) : null}
              <button
                type="button"
                onClick={() => {
                  form.setValue("code", "")
                  setCodeMode(codeMode === "totp" ? "backup" : "totp")
                }}
                className="text-xs text-muted-foreground transition-colors hover:text-foreground"
              >
                {codeMode === "totp"
                  ? t("settings:privacy.mfa.disable.useBackupInstead")
                  : t("settings:privacy.mfa.disable.useTotpInstead")}
              </button>
            </div>
          ) : null}
        </div>
      ) : null}

      {serverError ? (
        <Alert variant="destructive" data-testid="api-token-create-error">
          <AlertDescription>{serverError}</AlertDescription>
        </Alert>
      ) : null}

      <DialogFooter className="gap-2">
        <Button type="button" variant="outline" onClick={onCancel} disabled={pending}>
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={pending} data-testid="api-token-create-submit">
          {pending
            ? t("settings:apiTokens.create.submitting")
            : t("settings:apiTokens.create.submit")}
        </Button>
      </DialogFooter>
    </form>
  )
}

function RevealStep({
  secret,
  name,
  onDone,
}: {
  secret: string
  name: string
  onDone: () => void
}) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const inputId = useId()

  async function copy() {
    try {
      await navigator.clipboard.writeText(secret)
      toast.success(t("settings:apiTokens.reveal.copied"))
    } catch {
      toast.error(t("settings:apiTokens.reveal.copyFailed"))
    }
  }

  return (
    <div className="space-y-4">
      <DialogHeader>
        <DialogTitle className="flex items-center gap-2">
          <KeyRound className="size-5" aria-hidden="true" />
          {t("settings:apiTokens.reveal.title")}
        </DialogTitle>
        <DialogDescription>
          {t("settings:apiTokens.reveal.description", { name })}
        </DialogDescription>
      </DialogHeader>

      <Alert>
        <TriangleAlert className="size-4" aria-hidden="true" />
        <AlertDescription>{t("settings:apiTokens.reveal.warning")}</AlertDescription>
      </Alert>

      <div className="space-y-1.5">
        <Label htmlFor={inputId}>{t("settings:apiTokens.reveal.label")}</Label>
        <div className="flex items-center gap-2">
          <Input
            id={inputId}
            readOnly
            value={secret}
            className="font-mono text-xs"
            onFocus={(e) => e.currentTarget.select()}
            data-testid="api-token-secret"
          />
          <Button
            type="button"
            variant="outline"
            size="icon"
            aria-label={t("settings:apiTokens.reveal.copy")}
            onClick={() => void copy()}
            data-testid="api-token-secret-copy"
          >
            <ClipboardCopy className="size-4" aria-hidden="true" />
          </Button>
        </div>
        <p className="text-xs text-muted-foreground">{t("settings:apiTokens.reveal.usage")}</p>
        <code className="block rounded-md bg-muted px-2 py-1.5 font-mono text-[11px] break-all">
          Authorization: Bearer {secret.slice(0, 12)}…
        </code>
      </div>

      <DialogFooter>
        <Button type="button" onClick={onDone} data-testid="api-token-reveal-done">
          {t("settings:apiTokens.reveal.done")}
        </Button>
      </DialogFooter>
    </div>
  )
}
