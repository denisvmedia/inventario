// OAuth feature surface: provider discovery, linked-identities listing, and
// the unlink mutation. Used by:
//   - components/auth/OAuthRow (Login + Register entry points)
//   - components/settings/ConnectedAccountsCard (link/unlink in settings)
//   - pages/auth/LoginPage (the "link-required" banner reads via the query
//     params, but `oauthStartUrl` is shared)
//
// The BE's OAuth handlers live under /api/v1/auth/oauth/* (#1394). The
// start/link-start endpoints are server-side 302s, so the FE redirects the
// browser via window.location.assign — no fetch, no JSON.
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { http, HttpError } from "@/lib/http"
import type { Schema } from "@/types"

import { authKeys } from "./keys"

// Provider as the BE emits it on the wire. Display name is human copy;
// `name` is the URL token used in /api/v1/auth/oauth/{name}/start.
export type OAuthProviderName = "google" | "github"

export interface OAuthProviderEntry {
  name: OAuthProviderName
  displayName: string
}

interface OAuthProviderListResponseWire {
  providers?: Array<{ name?: string; display_name?: string }>
}

// One row from GET /auth/oauth/identities — the providers the caller has
// linked, with the email the provider returned at link time.
export type OAuthIdentity = Required<Schema<"apiserver.linkedIdentityEntry">>

interface OAuthIdentitiesResponseWire {
  identities?: Array<{ provider?: string; email?: string; linked_at?: string }>
}

// Resource paths under /api/v1.
const PROVIDERS_PATH = "/auth/oauth/providers"
const IDENTITIES_PATH = "/auth/oauth/identities"

// API_BASE_URL is the absolute URL prefix the BE OAuth handlers mount under.
// The OAuth start/link-start endpoints are server-driven 302s, so we send the
// browser straight there via window.location — `oauthStartUrl` builds the
// path so call sites don't sprinkle `/api/v1/auth/oauth/...` strings
// through the component tree.
const API_BASE = "/api/v1"

// adaptProviderList normalises the BE shape into a TS-friendly array. The
// wire shape uses optional fields and snake_case; the BE actually emits
// `display_name` as required when the provider is enabled, but the swagger
// codegen marks every field as optional.
function adaptProviderList(body: OAuthProviderListResponseWire): OAuthProviderEntry[] {
  const rows = body.providers ?? []
  return rows
    .map((row): OAuthProviderEntry | null => {
      const name = row.name?.toLowerCase()
      if (name !== "google" && name !== "github") return null
      return { name, displayName: row.display_name ?? row.name ?? name }
    })
    .filter((row): row is OAuthProviderEntry => row !== null)
}

function adaptIdentityList(body: OAuthIdentitiesResponseWire): OAuthIdentity[] {
  const rows = body.identities ?? []
  return rows
    .map((row): OAuthIdentity | null => {
      if (!row.provider || !row.email || !row.linked_at) return null
      return {
        provider: row.provider,
        email: row.email,
        linked_at: row.linked_at,
      }
    })
    .filter((row): row is OAuthIdentity => row !== null)
}

// Pure fetch wrappers — kept separate from the hooks so tests / boot probes
// can call them outside React.
export async function getOAuthProviders(signal?: AbortSignal): Promise<OAuthProviderEntry[]> {
  const body = await http.get<OAuthProviderListResponseWire>(PROVIDERS_PATH, { signal })
  return adaptProviderList(body)
}

export async function getOAuthIdentities(signal?: AbortSignal): Promise<OAuthIdentity[]> {
  const body = await http.get<OAuthIdentitiesResponseWire>(IDENTITIES_PATH, {
    signal,
    authCheck: "user-initiated",
  })
  return adaptIdentityList(body)
}

export async function unlinkOAuthProvider(provider: OAuthProviderName): Promise<void> {
  // The BE returns 204 on success and 409 when this is the user's last
  // remaining sign-in method. The 409 propagates as HttpError; the caller
  // (ConnectedAccountsCard) shows the dedicated last-method toast for that
  // status and falls back to the parsed server error otherwise.
  await http.del<void>(`/auth/oauth/${encodeURIComponent(provider)}`)
}

// useOAuthProviders reads the deployment's enabled-providers list. Cached for
// the session — the operator's provider config doesn't change inside a tab.
// Authentication is NOT required: the unauthenticated Login / Register pages
// both need to render the buttons.
export function useOAuthProviders() {
  return useQuery<OAuthProviderEntry[]>({
    queryKey: authKeys.oauthProviders(),
    queryFn: ({ signal }) => getOAuthProviders(signal),
    staleTime: 5 * 60 * 1000,
    // Operators don't toggle Google/GitHub off mid-session; suppress refetch
    // chatter so a tab that lingers on /login doesn't re-poll on every focus.
    refetchOnWindowFocus: false,
  })
}

// useOAuthIdentities reads the caller's linked providers. The ConnectedAccounts
// panel renders one row per identity plus a "Link <Provider>" row for each
// enabled-but-not-linked provider.
export function useOAuthIdentities() {
  return useQuery<OAuthIdentity[]>({
    queryKey: authKeys.oauthIdentities(),
    queryFn: ({ signal }) => getOAuthIdentities(signal),
    staleTime: 30 * 1000,
  })
}

// useUnlinkOAuthIdentity invalidates the identities query on success so the
// panel's row list and the OAuth row's link-state both refresh.
export function useUnlinkOAuthIdentity() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, OAuthProviderName>({
    mutationFn: (provider) => unlinkOAuthProvider(provider),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.oauthIdentities() })
    },
  })
}

// oauthStartUrl builds the absolute path the browser should be redirected to
// when the user clicks an OAuth provider button. The BE 302s into the
// provider's consent screen; on return the BE 302s back into the FE at the
// `redirect` param (or `/`). Use a `?redirect=...` only for paths the BE
// allow-listed in `allowedRedirectPrefixes` — see go/apiserver/oauth.go.
export function oauthStartUrl(provider: OAuthProviderName, redirect?: string): string {
  const params = new URLSearchParams()
  if (redirect) params.set("redirect", redirect)
  const qs = params.toString()
  const suffix = qs ? `?${qs}` : ""
  return `${API_BASE}/auth/oauth/${encodeURIComponent(provider)}/start${suffix}`
}

// oauthLinkStartUrl is the authenticated-user variant. Used from Settings →
// Connected Accounts to add a new provider to an existing account; the BE
// callback runs the link branch instead of find-or-create.
export function oauthLinkStartUrl(provider: OAuthProviderName, redirect?: string): string {
  const params = new URLSearchParams()
  if (redirect) params.set("redirect", redirect)
  const qs = params.toString()
  const suffix = qs ? `?${qs}` : ""
  return `${API_BASE}/auth/oauth/${encodeURIComponent(provider)}/link/start${suffix}`
}

// isLastMethodError reports whether an HttpError is the BE's
// "cannot remove the last sign-in method" conflict. Centralized so the
// settings panel can render the dedicated message rather than the generic
// parsed-server-error copy.
export function isLastMethodError(err: unknown): boolean {
  return err instanceof HttpError && err.status === 409
}
