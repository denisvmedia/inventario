---
title: API tokens for scripts
description: Let a script or another app call the Inventario API with a personal token you can limit and revoke.
---

A personal API token lets a script call the API as you without your password,
a login step or a refresh loop. Create one under **Preferences → Privacy &
security → API tokens**; [Settings & account](../settings-and-account/#api-tokens)
walks through the form. The token is shown once and starts with `inv_pat_`,
which makes a leaked one easy to search for.

Send it in the `Authorization` header of every request:

```sh
curl -H "Authorization: Bearer $INVENTARIO_TOKEN" \
  https://inventario.example.com/api/v1/groups
```

- **No CSRF token and no cookie.** Writes work with the header alone, because
  nothing the browser sends on its own is involved. Send exactly one
  `Authorization` header: a request with two is refused.
- **Read only** tokens may use `GET`, `HEAD` and `OPTIONS`. **Read and write**
  tokens may use every method, but your role in the group still decides: a
  viewer's token cannot change anything, whatever its access.
- **A token limited to one group** gets 403 `api_token.group_not_allowed` for
  any other group, and `GET /api/v1/groups` lists only its group.
- **Rate limits** apply per address and per token. The default is 1,000
  requests an hour each; the operator sets it with
  `INVENTARIO_RUN_GLOBAL_RATE_LIMIT` and `INVENTARIO_RUN_GLOBAL_RATE_WINDOW`.
  Over the limit the answer is 429 with a `Retry-After` header.

## Which endpoints to script against

The API is the app's own and not a stable integration contract: it changes
when the app changes, so pin the Inventario version your script was written
against and read the
[changelog](https://github.com/denisvmedia/inventario/blob/master/CHANGELOG.md)
before upgrading. The [API reference](../api/) lists every endpoint. These are
the parts a token is meant for, and the ones least likely to move:

- `GET /api/v1/groups`, to find the slug of each group the token can reach.
- The group data under `/api/v1/g/{groupSlug}/`: `locations`, `areas`,
  `commodities` (items), `files`, `tags`, `loans`, `services`, `maintenance`,
  `search` and `exports`, plus `uploads` for adding files.
- File downloads: a file's response carries short-lived signed URLs. Fetch
  those without the `Authorization` header.

For example, to list the items in the group with the slug `home-x7k2`:

```sh
curl -H "Authorization: Bearer $INVENTARIO_TOKEN" \
  https://inventario.example.com/api/v1/g/home-x7k2/commodities
```

Everything about the account itself refuses a token with 403
`api_token.endpoint_not_allowed`, even a valid one: sign-in, sign-out and
`/auth/me`, two-factor settings, `/users/me` (profile, sessions, and the
token endpoints, so a token cannot create another), creating, leaving or
administering groups, invitations, the admin area and the back office. Do
those in the browser.

## Errors

| Status | Code | Meaning |
| --- | --- | --- |
| 401 | `api_token.invalid` | The token is unknown, malformed, revoked or expired, or its owner's account is inactive. The response does not say which. |
| 401 | `api_token.ambiguous_credentials` | The request had more than one `Authorization` header. |
| 403 | `api_token.read_only` | A read-only token tried to change something. |
| 403 | `api_token.group_not_allowed` | The token is limited to another group. |
| 403 | `api_token.endpoint_not_allowed` | Tokens are not accepted on this endpoint. |
| 429 | `api_token.rate_limited` | Too many requests. Wait for `Retry-After` seconds. |

Each token's use is recorded in the server's audit log (at most once every five
minutes per token), and so are its creation and revocation.
