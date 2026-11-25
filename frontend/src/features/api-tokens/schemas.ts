import { z } from "zod"

// Matches api_tokens.name (VARCHAR(100)) and services.MaxAPITokenNameLength.
// The server counts runes; Array.from counts code points, which is the same
// measure for every name a person types.
export const API_TOKEN_NAME_MAX = 100

export const API_TOKEN_EXPIRY_PRESETS = ["30d", "90d", "365d", "never"] as const
export type APITokenExpiryPreset = (typeof API_TOKEN_EXPIRY_PRESETS)[number]

const PRESET_DAYS: Record<Exclude<APITokenExpiryPreset, "never">, number> = {
  "30d": 30,
  "90d": 90,
  "365d": 365,
}

// expiresAtFor turns a preset into the RFC 3339 time the API takes, or
// undefined for a token that does not expire.
export function expiresAtFor(
  preset: APITokenExpiryPreset,
  now: Date = new Date()
): string | undefined {
  if (preset === "never") return undefined
  return new Date(now.getTime() + PRESET_DAYS[preset] * 24 * 60 * 60 * 1000).toISOString()
}

// The factors the account has decide which re-authentication fields are
// required, so the schema is built per form instance.
export function createAPITokenSchema(opts: { requirePassword: boolean; requireCode: boolean }) {
  return z
    .object({
      name: z
        .string()
        .trim()
        .min(1, "settings:apiTokens.validation.nameRequired")
        .refine(
          (v) => Array.from(v).length <= API_TOKEN_NAME_MAX,
          "settings:apiTokens.validation.nameTooLong"
        ),
      scope: z.enum(["read_only", "read_write"]),
      // "" means every group the user belongs to.
      groupId: z.string(),
      expiry: z.enum(API_TOKEN_EXPIRY_PRESETS),
      currentPassword: z.string(),
      code: z.string(),
    })
    .superRefine((value, ctx) => {
      if (opts.requirePassword && !value.currentPassword) {
        ctx.addIssue({
          code: "custom",
          path: ["currentPassword"],
          message: "settings:apiTokens.validation.passwordRequired",
        })
      }
      if (opts.requireCode && !value.code.trim()) {
        ctx.addIssue({
          code: "custom",
          path: ["code"],
          message: "settings:apiTokens.validation.codeRequired",
        })
      }
    })
}

export type CreateAPITokenFormInput = z.infer<ReturnType<typeof createAPITokenSchema>>
