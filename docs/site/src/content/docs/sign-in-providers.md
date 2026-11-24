---
title: Sign-in providers
description: Let people sign in with Google, GitHub, or an OpenID Connect provider you run yourself, such as Authelia, Keycloak or Authentik.
---

Besides email and password, Inventario can offer sign-in with Google, with
GitHub, and with one OpenID Connect provider of your own, such as Authelia,
Keycloak or Authentik. Each one is off until you configure it. Once a provider
is on, its button appears on the sign-in and sign-up pages, and people can link
it to the account they already have in
[Settings → Privacy & security](../settings-and-account/#connected-accounts).

This page is for the person who runs the server.

## How a sign-in finds its account

When someone comes back from the provider, Inventario looks for their account
in this order:

1. **The provider account was used here before.** They are signed in to the
   account it is linked to.
2. **An account has the same email address.** If the provider says the address
   is verified, the provider is linked to that account and they are signed in.
   If it does not, nothing is linked: they are asked to sign in with their
   password and link the provider from Settings.
3. **No account matches.** A new account is created, if the server's
   registration mode allows it: in `open` mode it is active at once, in
   `approval` mode it waits for an administrator, and in `closed` mode the
   sign-in is refused.
   [Troubleshooting](../troubleshooting/#signing-in-with-google-github-or-single-sign-on-brings-me-back-to-the-sign-in-page)
   lists the messages people see.

Someone who has [two-factor authentication](../settings-and-account/#two-factor-authentication)
turned on cannot sign in with a provider alone. They sign in with their password
and code; the provider stays linked.

## Settings every provider needs

The settings are environment variables, and each has a command-line flag of the
same name (`INVENTARIO_RUN_OAUTH_REDIRECT_BASE_URL` is `--oauth-redirect-base-url`).
With the Helm chart, see [Helm chart](#helm-chart) below.

| Variable | What it does |
| --- | --- |
| `INVENTARIO_RUN_OAUTH_REDIRECT_BASE_URL` | The public address of Inventario, for example `https://inventario.example.com`. Each provider's redirect URI is this address plus `/api/v1/auth/oauth/<provider>/callback`. No provider works without it. |
| `INVENTARIO_RUN_OAUTH_STATE_KEY` | Signs the short-lived state that a sign-in carries to the provider and back. At least 32 characters; `openssl rand -hex 32` makes one. Without it, each process generates its own at startup: set it when more than one process serves the API, or a sign-in that comes back to another process fails. |

## Google

1. In the Google Cloud console, create an OAuth client of type **Web
   application**.
2. Add the authorized redirect URI
   `https://inventario.example.com/api/v1/auth/oauth/google/callback`.
3. Set `INVENTARIO_RUN_OAUTH_GOOGLE_CLIENT_ID` and
   `INVENTARIO_RUN_OAUTH_GOOGLE_CLIENT_SECRET`.

## GitHub

1. In GitHub, open **Settings → Developer settings → OAuth Apps** and create an
   app.
2. Set its authorization callback URL to
   `https://inventario.example.com/api/v1/auth/oauth/github/callback`.
3. Set `INVENTARIO_RUN_OAUTH_GITHUB_CLIENT_ID` and
   `INVENTARIO_RUN_OAUTH_GITHUB_CLIENT_SECRET`.

Inventario uses the primary email address of the GitHub account, and only when
GitHub has verified it.

## Your own OpenID Connect provider

| Variable | What it does |
| --- | --- |
| `INVENTARIO_RUN_OAUTH_OIDC_ISSUER_URL` | The provider's issuer, for example `https://auth.example.com`. Inventario reads the provider's endpoints from `/.well-known/openid-configuration` under this address, and the issuer named in that document has to match. |
| `INVENTARIO_RUN_OAUTH_OIDC_CLIENT_ID` | The client id you register at the provider. |
| `INVENTARIO_RUN_OAUTH_OIDC_CLIENT_SECRET` | The client secret, in plain text. |
| `INVENTARIO_RUN_OAUTH_OIDC_DISPLAY_NAME` | The label of the sign-in button, for example `Authelia`. Without it the button says **Single sign-on**. |
| `INVENTARIO_RUN_OAUTH_OIDC_BACKCHANNEL_URL` | Optional. The issuer's address as the Inventario server reaches it, when the public address works in the browser but not from the server, for example inside a cluster that cannot resolve the public name. Inventario's own requests (the discovery document, the token, the user info) go there. The browser still goes to the issuer. |

The provider is on when the issuer, the client id and the client secret are all
set. Register the client at the provider like this:

- redirect URI `https://inventario.example.com/api/v1/auth/oauth/oidc/callback`;
- authorization code flow, with PKCE and the `S256` method;
- scopes `openid`, `email` and `profile`;
- client authentication `client_secret_basic` (Inventario uses
  `client_secret_post` only when the provider offers nothing else);
- the user info answered as plain JSON, not as a signed token.

Inventario reads the user's `sub`, `email`, `email_verified`, and `name` (or
`preferred_username`) from the user info endpoint.

If the provider cannot be reached when Inventario starts, Inventario starts
anyway and tries again on the next sign-in, at most once every 30 seconds. Until
then, the button brings people back to the sign-in page with "Sign-in with
*name* failed", and the log says why.

### Example: Authelia

Generate the client secret. Authelia keeps only a hash of it, and Inventario
gets the plain text:

```bash
authelia crypto hash generate pbkdf2 --variant sha512 --random --random.length 72 --random.charset rfc3986
```

Add the client to Authelia's configuration:

```yaml
identity_providers:
  oidc:
    clients:
      - client_id: 'inventario'
        client_name: 'Inventario'
        client_secret: '$pbkdf2-sha512$310000$...'  # the hash from the command above
        authorization_policy: 'two_factor'
        require_pkce: true
        pkce_challenge_method: 'S256'
        redirect_uris:
          - 'https://inventario.example.com/api/v1/auth/oauth/oidc/callback'
        scopes: ['openid', 'email', 'profile']
        response_types: ['code']
        grant_types: ['authorization_code']
        token_endpoint_auth_method: 'client_secret_basic'
        userinfo_signed_response_alg: 'none'
```

Then configure Inventario:

```bash
INVENTARIO_RUN_OAUTH_REDIRECT_BASE_URL=https://inventario.example.com
INVENTARIO_RUN_OAUTH_OIDC_ISSUER_URL=https://auth.example.com
INVENTARIO_RUN_OAUTH_OIDC_CLIENT_ID=inventario
INVENTARIO_RUN_OAUTH_OIDC_CLIENT_SECRET=<the plain-text secret from the command above>
INVENTARIO_RUN_OAUTH_OIDC_DISPLAY_NAME=Authelia
```

Authelia reports every email address as verified, so the first sign-in of a
person who already has an Inventario account with the same address links the
two.

## Helm chart

The chart puts the non-secret settings in its ConfigMap and the credentials in
its Secret:

- `oauth.redirectBaseUrl` (empty means the same address as `app.publicUrl`),
  `oauth.oidc.issuerUrl`, `oauth.oidc.displayName` and
  `oauth.oidc.backchannelUrl`;
- `secrets.oauthStateKey`, and `secrets.oauth<Provider>ClientId` /
  `secrets.oauth<Provider>ClientSecret` for `Google`, `Github` and `Oidc`.

With `secrets.existingSecret`, put the `INVENTARIO_RUN_OAUTH_*_CLIENT_ID`,
`INVENTARIO_RUN_OAUTH_*_CLIENT_SECRET` and `INVENTARIO_RUN_OAUTH_STATE_KEY` keys
in that Secret instead.
