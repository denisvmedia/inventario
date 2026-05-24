import { useEffect } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"
import { Link2, Plus } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  isLastMethodError,
  oauthLinkStartUrl,
  useOAuthIdentities,
  useOAuthProviders,
  useUnlinkOAuthIdentity,
  type OAuthIdentity,
  type OAuthProviderEntry,
  type OAuthProviderName,
} from "@/features/auth/oauth"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

// ConnectedAccountsCard — the Privacy & Security row that lists OAuth
// providers the caller has linked and offers Link/Unlink controls (#1394).
// Hidden entirely when the operator hasn't wired any provider in this
// deployment; renders as a divided card matching MFASettingsRow's
// surface so the panel composes inside the existing PrivacySection.
export function ConnectedAccountsCard() {
  const { t } = useTranslation()
  const providersQuery = useOAuthProviders()
  const identitiesQuery = useOAuthIdentities()
  const toast = useAppToast()
  const [params, setParams] = useSearchParams()

  const providers = providersQuery.data ?? []
  const identities = identitiesQuery.data ?? []

  // When the link-callback redirects back here with
  // `?oauth_linked=<provider>`, surface a success toast then strip the
  // query so a refresh doesn't re-fire the same toast. The provider name
  // is whatever string the BE sent; we resolve a friendlier display name
  // via the providers query when it's already loaded.
  const linkedQuery = params.get("oauth_linked")
  useEffect(() => {
    if (!linkedQuery) return
    const friendly =
      providers.find((p) => p.name === linkedQuery)?.displayName ?? linkedQuery
    toast.success(t("auth:oauth.linkSuccess", { provider: friendly }))
    const next = new URLSearchParams(params)
    next.delete("oauth_linked")
    setParams(next, { replace: true })
    // Run once per `oauth_linked` value; the toast call itself doesn't
    // need to participate in the dependency list — `useAppToast` returns
    // a stable surface backed by the sonner module.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [linkedQuery])

  // No deployment-level providers → the operator hasn't configured Google
  // or GitHub, so this entire surface is non-interactive. Hiding it (rather
  // than rendering an empty card) keeps the settings page from sprouting
  // dead chrome.
  if (providersQuery.isLoading) return <ConnectedAccountsSkeleton />
  if (providers.length === 0) return null

  // Map each enabled provider to either a linked-row (identity present) or
  // a link-row (no identity). Identities for providers the operator since
  // disabled fall through silently — they still exist on the user row but
  // the settings UI doesn't surface them.
  const rowsByProvider = new Map(identities.map((i) => [i.provider, i]))

  return (
    <section
      className="space-y-3"
      data-testid="connected-accounts-card"
      aria-labelledby="connected-accounts-title"
    >
      <header className="space-y-0.5">
        <h3
          id="connected-accounts-title"
          className="text-xs font-semibold uppercase tracking-wide text-muted-foreground"
        >
          {t("auth:oauth.connectedAccounts.title")}
        </h3>
        <p className="text-xs text-muted-foreground">
          {t("auth:oauth.connectedAccounts.subtitle")}
        </p>
      </header>
      <div className="rounded-xl border border-border divide-y divide-border">
        {providers.map((provider) => {
          const linked = rowsByProvider.get(provider.name)
          return (
            <ConnectedAccountRow
              key={provider.name}
              provider={provider}
              identity={linked}
              identityCount={identities.length}
            />
          )
        })}
      </div>
    </section>
  )
}

interface ConnectedAccountRowProps {
  provider: OAuthProviderEntry
  identity: OAuthIdentity | undefined
  identityCount: number
}

function ConnectedAccountRow({
  provider,
  identity,
  identityCount,
}: ConnectedAccountRowProps) {
  const { t } = useTranslation()
  const confirm = useConfirm()
  const toast = useAppToast()
  const unlinkMutation = useUnlinkOAuthIdentity()
  const isLinked = !!identity

  async function handleUnlink() {
    if (!identity) return
    const ok = await confirm({
      title: t("auth:oauth.unlink.confirmTitle", { provider: provider.displayName }),
      description: t("auth:oauth.unlink.confirmBody"),
      confirmLabel: t("auth:oauth.unlink.confirmCta"),
      destructive: true,
    })
    if (!ok) return
    try {
      await unlinkMutation.mutateAsync(provider.name)
      toast.success(
        t("auth:oauth.unlink.success", { provider: provider.displayName })
      )
    } catch (err) {
      if (isLastMethodError(err)) {
        toast.error(t("auth:oauth.unlink.errorLastMethod"))
        return
      }
      toast.error(parseServerError(err, t("auth:oauth.unlink.errorGeneric")))
    }
  }

  function handleLink() {
    // BE link-start endpoint is authenticated and 302s to the provider; the
    // callback runs the link branch and lands back at the `redirect=` path.
    window.location.assign(oauthLinkStartUrl(provider.name, "/settings"))
  }

  return (
    <div
      className="flex items-center justify-between gap-4 p-4"
      data-testid={`connected-account-row-${provider.name}`}
      data-linked={isLinked ? "true" : "false"}
    >
      <div className="flex min-w-0 items-center gap-3">
        <span
          className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted/60"
          aria-hidden="true"
        >
          <ProviderIcon provider={provider.name} />
        </span>
        <div className="min-w-0">
          <p className="text-sm font-medium">{provider.displayName}</p>
          {identity ? (
            <p
              className="mt-0.5 text-xs text-muted-foreground"
              data-testid={`connected-account-meta-${provider.name}`}
            >
              {identity.email}
              <span aria-hidden="true"> · </span>
              {t("auth:oauth.connectedAccounts.linkedAt", {
                date: formatDate(identity.linked_at, { style: "long" }),
              })}
            </p>
          ) : (
            <p className="mt-0.5 text-xs text-muted-foreground">
              {t("auth:oauth.connectedAccounts.unlinked")}
            </p>
          )}
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {isLinked ? (
          <>
            <Badge variant="secondary" data-testid={`connected-account-badge-${provider.name}`}>
              {t("auth:oauth.connectedAccounts.linked")}
            </Badge>
            <Button
              variant="outline"
              size="sm"
              onClick={handleUnlink}
              disabled={unlinkMutation.isPending}
              data-testid={`connected-account-unlink-${provider.name}`}
              data-identity-count={identityCount}
            >
              {t("auth:oauth.unlink.confirmCta")}
            </Button>
          </>
        ) : (
          <Button
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={handleLink}
            data-testid={`connected-account-link-${provider.name}`}
          >
            <Plus className="size-3.5" aria-hidden="true" />
            {t("auth:oauth.connectedAccounts.link", {
              provider: provider.displayName,
            })}
          </Button>
        )}
      </div>
    </div>
  )
}

function ProviderIcon({ provider }: { provider: OAuthProviderName }) {
  // lucide-react doesn't ship dedicated Google / GitHub brand glyphs in the
  // version we pull in; the row's text label disambiguates which provider
  // this is, so the icon is just a neutral chrome.
  if (provider === "github") {
    return <GithubGlyph />
  }
  return <Link2 className="size-4 text-muted-foreground" aria-hidden="true" />
}

function GithubGlyph() {
  return (
    <svg className="size-4 text-muted-foreground" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
    </svg>
  )
}

function ConnectedAccountsSkeleton() {
  return (
    <div
      className="rounded-xl border border-border bg-muted/30 p-4 text-xs text-muted-foreground"
      data-testid="connected-accounts-loading"
      aria-busy="true"
    >
      &nbsp;
    </div>
  )
}
