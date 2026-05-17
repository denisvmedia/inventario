// FeedbackDialog (issue #1387). In-app channel for the user to file a
// bug / feature request / general feedback against the operator-
// configured support inbox.
//
// Wiring:
//   - Auth-required: the dialog is mounted under SettingsPage which
//     already enforces auth; the BE returns 401 if a stray request
//     somehow lands here without a session.
//   - Rate-limited at the BE (5/hour/user). On 429 we surface a
//     friendly "try again later" toast that points at the static
//     mailto fallback in Settings → Help.
//   - On 503 (SUPPORT_EMAIL not configured) we surface a "feedback
//     isn't set up on this deployment" toast that also points at the
//     mailto fallback. This is a real operator state — single-tenant
//     deployments may not bother wiring SUPPORT_EMAIL at all.
import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect, useId, useMemo } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

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
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Textarea } from "@/components/ui/textarea"
import { useAuth } from "@/features/auth/AuthContext"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { FEEDBACK_TYPES, type FeedbackType } from "@/features/feedback/api"
import { useSubmitFeedback } from "@/features/feedback/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { APP_VERSION } from "@/lib/app-version"
import { HttpError } from "@/lib/http"
import { parseServerError } from "@/lib/server-error"

// 5 KB to match the BE cap in apiserver/feedback.go. The two numbers
// must agree so the user can't hit a confusing "too long" 400 with
// "valid" text on the FE.
const MESSAGE_MAX_BYTES = 5 * 1024

const feedbackSchema = z.object({
  type: z.enum(FEEDBACK_TYPES as unknown as [FeedbackType, ...FeedbackType[]]),
  message: z
    .string()
    .trim()
    .min(1, "feedback:validation.messageRequired")
    .max(MESSAGE_MAX_BYTES, "feedback:validation.messageTooLong"),
  reply_to_email: z
    .union([z.literal(""), z.string().email("feedback:validation.replyToInvalid")])
    .optional()
    .default(""),
  include_diagnostics: z.boolean(),
})

export type FeedbackFormInput = z.input<typeof feedbackSchema>

export interface FeedbackDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

// collectDiagnostics gathers the documented diagnostics payload the
// user can opt into. Keys are stable strings the BE renders verbatim
// in the support email — see the issue brief for the canonical list.
// Anything we can't read safely (e.g. no window in SSR or test mode)
// is omitted rather than emitting an empty string.
function collectDiagnostics(args: {
  userID?: string
  userEmail?: string
  groupSlug?: string
}): Record<string, string> {
  const out: Record<string, string> = {}
  out.app_version = APP_VERSION
  if (typeof window !== "undefined") {
    out.url = window.location.href
    if (window.innerWidth && window.innerHeight) {
      out.window_size = `${window.innerWidth}x${window.innerHeight}`
    }
  }
  if (typeof navigator !== "undefined" && navigator.userAgent) {
    out.user_agent = navigator.userAgent
  }
  if (args.userID) out.user_id = args.userID
  if (args.userEmail) out.user_email = args.userEmail
  if (args.groupSlug) out.group_slug = args.groupSlug
  return out
}

export function FeedbackDialog({ open, onOpenChange }: FeedbackDialogProps) {
  const { t } = useTranslation(["feedback", "common"])
  const { user } = useAuth()
  // useOptionalCurrentGroup: the feedback dialog may open from a
  // chrome surface (sidebar, settings) that is mounted before a group
  // is active. Falling back to `undefined` is safer than throwing.
  const currentGroup = useOptionalCurrentGroup()
  const submitFeedback = useSubmitFeedback()
  const toast = useAppToast()
  const formId = useId()

  // Reply-to defaults to the logged-in user's address. The user can
  // clear it to "don't reply, just listen" or replace it with another
  // address (e.g., a shared team inbox).
  const defaultReplyTo = user?.email ?? ""

  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
    setValue,
    watch,
  } = useForm<FeedbackFormInput>({
    resolver: zodResolver(feedbackSchema),
    defaultValues: {
      type: "feedback",
      message: "",
      reply_to_email: defaultReplyTo,
      include_diagnostics: true,
    },
  })

  // Reset on open so a previously submitted message doesn't leak back
  // into the form the next time the user opens the dialog.
  useEffect(() => {
    if (open) {
      reset({
        type: "feedback",
        message: "",
        reply_to_email: defaultReplyTo,
        include_diagnostics: true,
      })
    }
  }, [open, reset, defaultReplyTo])

  const watchedType = watch("type")
  const watchedIncludeDiagnostics = watch("include_diagnostics")

  // Memoise so the diagnostics preview doesn't recompute on every keystroke.
  const currentGroupSlug = currentGroup?.currentGroup?.slug ?? undefined
  const diagnosticsPreview = useMemo(
    () =>
      collectDiagnostics({
        userID: user?.id,
        userEmail: user?.email,
        groupSlug: currentGroupSlug,
      }),
    [user?.id, user?.email, currentGroupSlug]
  )

  const onSubmit = handleSubmit(async (values) => {
    const diagnostics = values.include_diagnostics
      ? collectDiagnostics({
          userID: user?.id,
          userEmail: user?.email,
          groupSlug: currentGroupSlug,
        })
      : undefined
    try {
      await submitFeedback.mutateAsync({
        type: values.type,
        message: values.message,
        replyToEmail: values.reply_to_email?.trim() || undefined,
        diagnostics,
      })
      toast.success(t("feedback:toasts.success"))
      onOpenChange(false)
    } catch (err) {
      if (err instanceof HttpError) {
        if (err.status === 429) {
          toast.error(t("feedback:toasts.rateLimited"))
          return
        }
        if (err.status === 503) {
          toast.error(t("feedback:toasts.notConfigured"))
          return
        }
      }
      toast.error(parseServerError(err, t("feedback:toasts.error")))
    }
  })

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg" data-testid="feedback-dialog">
        <DialogHeader>
          <DialogTitle>{t("feedback:dialog.title")}</DialogTitle>
          <DialogDescription>{t("feedback:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form className="flex flex-col gap-4" noValidate onSubmit={onSubmit}>
          <div className="flex flex-col gap-1.5">
            <Label>{t("feedback:fields.type")}</Label>
            <RadioGroup
              value={watchedType}
              onValueChange={(value) => setValue("type", value as FeedbackType)}
              className="grid grid-cols-2 gap-2 sm:grid-cols-4"
              data-testid="feedback-type"
            >
              {FEEDBACK_TYPES.map((type) => {
                const id = `${formId}-type-${type}`
                return (
                  <Label
                    key={type}
                    htmlFor={id}
                    className="flex cursor-pointer items-center gap-2 rounded-md border border-input px-3 py-2 text-sm transition-colors has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/10"
                  >
                    <RadioGroupItem id={id} value={type} data-testid={`feedback-type-${type}`} />
                    <span>{t(`feedback:types.${type}`)}</span>
                  </Label>
                )
              })}
            </RadioGroup>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${formId}-message`}>{t("feedback:fields.message")}</Label>
            <Textarea
              id={`${formId}-message`}
              rows={6}
              placeholder={t("feedback:fields.messagePlaceholder")}
              data-testid="feedback-message"
              {...register("message")}
            />
            {errors.message?.message ? (
              <p className="text-xs text-destructive" data-testid="feedback-message-error">
                {t(errors.message.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor={`${formId}-reply-to`}>{t("feedback:fields.replyTo")}</Label>
            <Input
              id={`${formId}-reply-to`}
              type="email"
              autoComplete="email"
              placeholder={t("feedback:fields.replyToPlaceholder")}
              data-testid="feedback-reply-to"
              {...register("reply_to_email")}
            />
            {errors.reply_to_email?.message ? (
              <p className="text-xs text-destructive" data-testid="feedback-reply-to-error">
                {t(errors.reply_to_email.message)}
              </p>
            ) : (
              <p className="text-xs text-muted-foreground">{t("feedback:fields.replyToHelp")}</p>
            )}
          </div>

          <div className="flex items-start gap-2">
            <Checkbox
              id={`${formId}-include-diagnostics`}
              checked={watchedIncludeDiagnostics}
              onCheckedChange={(value) => setValue("include_diagnostics", value === true)}
              data-testid="feedback-include-diagnostics"
            />
            <div className="flex flex-col gap-0.5">
              <Label
                htmlFor={`${formId}-include-diagnostics`}
                className="text-sm font-medium cursor-pointer"
              >
                {t("feedback:fields.includeDiagnostics")}
              </Label>
              <p className="text-xs text-muted-foreground">
                {t("feedback:fields.includeDiagnosticsHelp")}
              </p>
              {watchedIncludeDiagnostics ? (
                <ul
                  className="mt-1 max-h-32 overflow-y-auto rounded-md border border-input bg-muted/30 p-2 text-xs text-muted-foreground"
                  data-testid="feedback-diagnostics-preview"
                >
                  {Object.entries(diagnosticsPreview)
                    .sort(([a], [b]) => a.localeCompare(b))
                    .map(([key, value]) => (
                      <li key={key} className="break-words">
                        <span className="font-medium text-foreground/80">{key}:</span> {value}
                      </li>
                    ))}
                </ul>
              ) : null}
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting || submitFeedback.isPending}
              data-testid="feedback-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || submitFeedback.isPending}
              data-testid="feedback-submit"
            >
              {submitFeedback.isPending
                ? t("feedback:dialog.submitting")
                : t("feedback:dialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
