import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { GithubGlyph } from "@/components/auth/icons/GithubGlyph"
import { GoogleGlyph } from "@/components/auth/icons/GoogleGlyph"
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

