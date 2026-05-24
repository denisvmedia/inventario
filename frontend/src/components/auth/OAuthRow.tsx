import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import {
  oauthStartUrl,
  useOAuthProviders,
  type OAuthProviderName,
} from "@/features/auth/oauth"
import { sanitizeRedirectPath } from "@/lib/safe-redirect"

// OAuthRow renders the "or continue with" surface above the password form
// on Login + Register. It's hidden when GET /auth/oauth/providers returns
// an empty list — i.e. the operator hasn't wired any provider in this
// deployment — so the buttons NEVER appear for a not-yet-supported
// provider (#1394).
//
// Clicking a provider 302s the browser to the BE start endpoint, which
// signs state + PKCE, sets the state cookie, and redirects to the
// provider's consent screen. After the consent round-trip the BE
// callback lands the user back in the app at the `redirect` param
// below (or `/`).
export function OAuthRow() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const { data: providers } = useOAuthProviders()

  // Hide the entire row when there are no enabled providers — including
  // while the providers query is still loading, since an empty render is
  // less jarring than a skeleton row that disappears half a frame later.
  if (!providers || providers.length === 0) return null

  // Reuse the same redirect chain the password form would have: ?redirect
  // (sanitised against open-redirect) or `/`. The BE's allow-list re-checks
  // it, but cleaning here keeps the URL the user sees on the provider's
  // consent screen short and predictable.
  const redirect = sanitizeRedirectPath(params.get("redirect"))

  return (
    <div className="space-y-4" data-testid="oauth-row">
      <div className="relative">
        <Separator />
        <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-background px-2 text-xs text-muted-foreground">
          {t("auth:oauth.divider")}
        </span>
      </div>
      <div
        className={
          providers.length === 1
            ? "grid grid-cols-1 gap-3"
            : "grid grid-cols-2 gap-3"
        }
      >
        {providers.map((p) => (
          <ProviderButton key={p.name} provider={p.name} redirect={redirect} />
        ))}
      </div>
    </div>
  )
}

function ProviderButton({
  provider,
  redirect,
}: {
  provider: OAuthProviderName
  redirect: string
}) {
  const { t } = useTranslation()
  function handleClick() {
    // Full-page navigation: the BE returns a 302 to the provider — fetch
    // doesn't follow cross-origin redirects, so we hand control to the
    // browser via location.assign. Using assign() (rather than href=)
    // keeps tests able to spy on the call without jsdom blowing up on a
    // navigation attempt.
    window.location.assign(oauthStartUrl(provider, redirect))
  }
  const label =
    provider === "google" ? t("auth:oauth.google") : t("auth:oauth.github")
  return (
    <Button
      variant="outline"
      className="gap-2 text-sm"
      type="button"
      onClick={handleClick}
      data-testid={`oauth-${provider}-button`}
    >
      {provider === "google" ? <GoogleGlyph /> : <GithubGlyph />}
      {label}
    </Button>
  )
}

function GoogleGlyph() {
  return (
    <svg className="size-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M12.48 10.92v3.28h7.84c-.24 1.84-.853 3.187-1.787 4.133-1.147 1.147-2.933 2.4-6.053 2.4-4.827 0-8.6-3.893-8.6-8.72s3.773-8.72 8.6-8.72c2.6 0 4.507 1.027 5.907 2.347l2.307-2.307C18.747 1.44 16.133 0 12.48 0 5.867 0 .307 5.387.307 12s5.56 12 12.173 12c3.573 0 6.267-1.173 8.373-3.36 2.16-2.16 2.84-5.213 2.84-7.667 0-.76-.053-1.467-.173-2.053H12.48z" />
    </svg>
  )
}

function GithubGlyph() {
  return (
    <svg className="size-4" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
    </svg>
  )
}
