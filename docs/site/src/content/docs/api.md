---
title: API reference
description: The HTTP API behind the app, browsable and versioned alongside these docs.
---

Everything the Inventario web app does, it does through an HTTP API, and that
API is documented here:

<a href="../api/" class="not-content" style="display:inline-block;padding:0.6rem 1rem;border-radius:0.5rem;background:var(--sl-color-accent);color:var(--sl-color-black);font-weight:600;text-decoration:none">Open the API reference</a>

The reference is generated from the same OpenAPI specification the server
ships, so it describes the version of these docs you are reading — not
whatever is on the main branch. The raw spec is at
[`swagger.yaml`](../api/swagger.yaml) if you want to feed it to a client
generator or an HTTP client.

## Before you build against it

A few things are worth knowing up front, because they are easy to discover the
slow way:

- **It is the app's own API, not a stable integration contract.** It changes
  when the app changes. Endpoints are versioned under `/api/v1`, but within
  that version fields are added and behaviour is refined between releases.
  Pin the Inventario version you developed against and read the
  [changelog](https://github.com/denisvmedia/inventario/blob/master/CHANGELOG.md)
  before upgrading.
- **Most data is group-scoped.** Items, locations, areas and files live under
  `/api/v1/g/{groupSlug}/…`. A request to the unscoped path will not find
  them.
- **Authentication is a bearer token** from `POST /api/v1/auth/login`, and
  **mutating requests also need a CSRF token** returned by the same call, sent
  as `X-CSRF-Token`. A 403 on a POST that looks correct is usually this.
- **File downloads use short-lived signed URLs** rather than the bearer token,
  so a link you captured will stop working. Ask for a fresh one.

## The interactive explorer

A deployment can also expose Swagger UI at `/swagger` — it talks to that
server, with your session, so you can execute calls rather than read about
them. It is **off by default** and is meant for development: leaving it on in
production advertises the whole API surface to anonymous callers. Operators
turn it on with `app.enableApiDocs` in the Helm chart.
