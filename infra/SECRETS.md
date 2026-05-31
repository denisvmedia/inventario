# Secrets setup (#1854)

Operating reference for the sops+age secret bundle that `infra/vm/bootstrap.sh`
consumes. Covers one-time setup, the GitHub App + Tailscale OAuth flows, and
the disaster-recovery runbook.

> Full beginner README will live in #1863. This file is the focused secrets
> walkthrough.

## Architecture in one paragraph

All identity-critical state lives in `infra/vm/secrets/secrets.enc.yaml`
encrypted with **sops + age**. The only thing that must live **off-VM** is one
age private key on the laptop (with a copy in your password manager). The
encrypted bundle is committed to the repo. `bootstrap.sh` decrypts it locally
on the laptop, ships the plaintext to the VM tmpfs, and `apply-secrets.sh`
turns the JSON into the admin / repo-creds / OAuth Kubernetes Secrets
(`inv-vcl01-master` + `inv-vcl01-longevity` `inventario-admin`,
`argocd/github-app-creds`, `tailscale/operator-oauth`). The `inventario-admin`
Secret carries the admin seed password and, when the optional `jwt.secret` /
`file_signing.key` / `oauth_state.key` fields are set, the matching stable
signing keys so sessions, back-office MFA, signed file URLs, and in-flight
OAuth sign-ins survive restarts (see §4b). When the `velero.*`
keys are filled, `vm-install.sh` additionally materializes
`velero/cloud-credentials` (R2 API token) and `velero/velero-repo-credentials`
(the kopia repo password) at install time — see "Velero / Cloudflare R2" below.

> **Note on the Velero repo password (`velero.encryption_key`).** Like the age
> key, it is a *second* off-VM-critical secret: a restore on a fresh VM
> re-derives the kopia repo from it, so losing it (or rotating it after the
> first backup) orphans every backup in R2. Back it up in your password
> manager next to the age key.

## Files in this layout

```
.sops.yaml                       ← repo-root: creation_rules + age recipients
                                   (sops walks up from $PWD to find this)
infra/vm/secrets/
├── .gitignore                   ← ignores *.plain.*, secrets.{yaml,json}
├── secrets.example.yaml         ← schema reference, safe to commit
└── secrets.enc.yaml             ← THE bundle, sops-encrypted
```

---

## One-time setup (you do this once, ever)

### 1. Generate the age keypair

On your laptop, **once** in your entire life:

```bash
mkdir -p ~/.config/sops/age
age-keygen -o ~/.config/sops/age/keys.txt
chmod 600 ~/.config/sops/age/keys.txt
```

This prints a `age1...` **public** key to stdout (and stores both halves in
`keys.txt`). The public key goes into `.sops.yaml` and into this repo. The
private key file stays on your laptop and never leaves it via git.

**macOS-specific gotcha**: sops on macOS looks for the age key at
`~/Library/Application Support/sops/age/keys.txt` (the OS-native
"Application Support" XDG path), NOT at `~/.config/sops/age/keys.txt`.
If you keep the key at the latter (cleaner), make a symlink so sops finds it:

```bash
mkdir -p "$HOME/Library/Application Support/sops/age"
ln -sf "$HOME/.config/sops/age/keys.txt" \
       "$HOME/Library/Application Support/sops/age/keys.txt"
```

Alternative: set `SOPS_AGE_KEY_FILE=~/.config/sops/age/keys.txt` in your
shell rc. The symlink is the less invasive option.

**Critical**: back up `~/.config/sops/age/keys.txt` to your password manager
(1Password / Bitwarden / etc.) right now, before doing anything else. If you
lose it, the secrets bundle becomes a brick — there's no recovery path. Other
team members who get added later will get their own age key and become an
additional recipient on the bundle (`age:` accepts a comma-separated list).

The public key (`age1...`) needs to land in the repo-root `.sops.yaml`. If
you're the first person setting this up, write it there directly (see
"Adding teammates" below for the multi-recipient layout). If `.sops.yaml`
already lists one recipient (the current case in this repo), add yours as
a second one and re-encrypt — also covered in "Adding teammates".

### 2. Create the GitHub App

ArgoCD's `repo-server` (for `git clone` of the inventario repo) and the
ApplicationSet PR generator (for listing PRs and reading labels) consume
a GitHub App credential — no PAT, no custom bot component. Permissions are
read-only; ArgoCD never writes to GitHub.

1. Open <https://github.com/settings/apps/new>
2. Fill in:
   - **GitHub App name**: `inventario-deploy-bot` (must be globally unique on
     GitHub; suffix with your handle if taken)
   - **Homepage URL**: `https://github.com/denisvmedia/inventario`
   - **Webhook** → **Active**: uncheck (we poll, not webhook — see [#1852 pivot](https://github.com/denisvmedia/inventario/issues/1852#issuecomment-4528693792))
   - **Repository permissions**:
     - **Contents**: Read-only
     - **Metadata**: Read-only (always required by GitHub)
     - **Pull requests**: Read-only
     - (everything else stays "No access")
   - **Subscribe to events**: none (we poll)
   - **Where can this GitHub App be installed?**: **Only on this account**
3. Click **Create GitHub App**.
4. On the resulting App page, record the **App ID** (top of page, e.g.
   `App ID: 123456`).
5. Click **Generate a private key**. A `.pem` file downloads — keep it
   somewhere safe for the next step.
6. In the left sidebar, click **Install App** → next to your username, click
   **Install** → choose **Only select repositories** → pick
   `denisvmedia/inventario` → **Install**.
7. After install, the URL becomes `https://github.com/settings/installations/<NNNNN>`.
   Record the `NNNNN` — that's the **Installation ID**.

Stash these three values (App ID, Installation ID, private key PEM) for
the "Filling the bundle" step below.

### 3. Create the Tailscale OAuth client

For the Tailscale Kubernetes Operator (#1855) to create ephemeral tailnet
nodes for each Service/Ingress AND to bootstrap its own tailnet identity.

1. Open <https://login.tailscale.com/admin/settings/oauth>
2. Click **Generate OAuth client**.
3. **Description**: `inventario-vcluster-operator`
4. **Scopes** — tick BOTH (one alone is not enough):
   - **Devices → Core → Write** (Read is implied; operator creates ephemeral
     proxy devices).
   - **Keys → Auth Keys → Write** (operator mints one-shot auth-keys per
     proxy device).
   Everything else stays unchecked, including `API Access Tokens` which is
   sometimes pre-ticked — uncheck it (principle of least privilege).
5. **Tags** — pick BOTH (one alone is not enough):
   - `tag:k8s-operator` — the operator pod's own tailnet identity.
   - `tag:k8s` — every proxy node the operator spawns for a Service/Ingress.
   The `tagOwners` configuration in your Tailscale ACL must match (see
   "Tailscale ACL — `tagOwners`" sub-section below). Get the ACL right
   BEFORE generating the client — Tailscale checks tag ownership at
   client-create time, and a mis-configured ACL forces you to regenerate.
6. Click **Generate Client**.
7. Copy the **Client ID** (visible from then on in the table) and the
   **Client Secret** (visible **once**, can't be retrieved later — save it
   immediately).

Stash these two values for the "Filling the bundle" step below.

#### Tailscale ACL — `tagOwners` (one-time, easy to forget)

After creating the OAuth client, you also need the tags it'll use to be
**owned** by something in the Tailscale ACL — otherwise the operator
boots with `creating operator authkey: requested tags [tag:k8s-operator]
are invalid or not permitted (400)`.

Open <https://login.tailscale.com/admin/acls> and add (or extend) the
`tagOwners` block — **note the empty owners list on `tag:k8s-operator`,
this is intentional** (see "Why empty `[]`" below):

```json
"tagOwners": {
  "tag:k8s-operator": [],
  "tag:k8s":          ["tag:k8s-operator"]
}
```

What this means:

- **`tag:k8s-operator`** — the operator pod's own tailnet identity
  (one node, permanent). Empty owners list `[]` means "anyone with
  `auth_keys:write` scope on this tailnet can mint auth-keys for this
  tag", which is exactly what the operator needs to bootstrap itself.
- **`tag:k8s`** — the tag the operator stamps on every ephemeral proxy
  node it spawns per `Service` / `Ingress`. Owning this with
  `["tag:k8s-operator"]` is the standard "chain of trust": only a device
  carrying `tag:k8s-operator` (the operator itself) can issue auth-keys
  for `tag:k8s` proxy nodes.

#### Why empty `[]` and not `["<your-email>"]`?

Counter-intuitive but correct: Tailscale's OAuth-as-caller permission
model treats an OAuth client as a **tagged device**, not as the user who
created it. Listing a user email there would let the user-the-human mint
auth-keys for `tag:k8s-operator` — but the OAuth client isn't acting as
that user. The empty list (`[]`) is the canonical operator-bootstrap pattern
documented at <https://tailscale.com/kb/1236/kubernetes-operator>.

Symptom of getting this wrong: operator crashes with
`creating operator authkey: requested tags [tag:k8s-operator] are invalid
or not permitted (400)`. We've been there.

Save in the ACL editor — changes apply instantly. The operator pod
(if already deployed and CrashLoopBackoff'ing on this) will recover on
its next restart, or force it with
`kubectl -n tailscale rollout restart deploy/operator`.

### 4. Pick the admin email + password

The Inventario `admin.password` field in the sops bundle becomes the
**master** super-admin login the app seeds on first start (#1883).
`apply-secrets.sh` materializes it as `inv-vcl01-master/inventario-admin`
with key `SETUP_ADMIN_PASSWORD`; the master ArgoCD Application points
`secrets.existingSecret` at this Secret and the chart's setup Job reads
the password from it.

The `admin.email` field is informational — the chart sources the admin
email from its values (`setupJob.initData.adminEmail`, default
`admin@example.com`), not from the sops bundle. Filling `admin.email`
in the bundle keeps the docs honest about "who is the admin" but does
not affect what gets seeded.

PR previews are deliberately NOT covered by this Secret: their
namespaces (`inv-vcl01-pr{N}`) are created dynamically by the
ApplicationSet, so there's nowhere for `apply-secrets.sh` to write the
Secret pre-emptively. Per-PR previews use the well-known dev password
`PreviewAdmin123` inlined in `infra/argocd/applicationset-pr.yaml` —
acceptable because the URL sits behind a Tailscale ACL and the
environment is tearable.

```bash
# Generate a strong random password (or use your own)
openssl rand -base64 24
```

Stash both for "Filling the bundle".

The `backoffice.password` field seeds the back-office (platform-operator)
login for `/backoffice/login` on the demo/preview envs (#1967), in the same
flow as `admin.password`: `apply-secrets.sh` writes it into the
`inventario-admin` Secret under key `BACKOFFICE_USER_PASSWORD`, and the chart's
setup / init-data Job reads it via `secretKeyRef` to run
`inventario backoffice bootstrap --password` (idempotent on email) when
`backofficeUser.enabled=true` — which `preview-base.values.yaml` sets on master
+ longevity, with `mfaEnforced=false` so the operator can sign in password-only
(a Helm Job can't complete interactive TOTP enrollment, and an MFA-enforced
operator with no secret row fails closed with HTTP 501 at login). Same
complexity rule as the admin password (min 8 chars + upper + lower + digit).
Per-PR previews inline a dev value (`setupJob.initData.backofficeUserPassword`)
in `applicationset-pr.yaml` instead. Leave `backoffice.password` empty only if
you do NOT want a back-office operator on these envs — the Job's bootstrap step
then fails loudly until it is set. Production never enables `backofficeUser`, so
the field is inert there (operators are created by hand so the one-time password
is never stored, and MFA is enrolled out-of-band via
`inventario backoffice mfa setup`).

### 4b. Generate the stable signing keys (recommended for master + longevity)

Three runtime keys pin the apiserver's signing material on the persistent envs.
`apply-secrets.sh` writes each into the same `inventario-admin` Secret, and the
chart loads the whole Secret via `envFrom`:

- **`jwt.secret`** → `INVENTARIO_RUN_JWT_SECRET` — signs auth tokens.
- **`file_signing.key`** → `INVENTARIO_RUN_FILE_SIGNING_KEY` — signs time-limited
  file-download URLs.
- **`oauth_state.key`** → `INVENTARIO_RUN_OAUTH_STATE_KEY` — signs the OAuth
  `state` parameter during social sign-in.

Why they matter: when a field is empty, each apiserver pod generates a fresh
**random** value at startup (`getJWTSecret()` / `getFileSigningKey()` /
`getOAuthStateKey()` in `go/cmd/inventario/run/bootstrap/crypto.go`). On a
long-lived env:

- An ephemeral **JWT secret** logs **everyone out on every redeploy**, and —
  because the back-office (super-admin) plane encrypts each operator's TOTP
  secret with an HKDF subkey of it — makes a back-office MFA enrollment
  **undecryptable after the next restart**, locking operators out of
  `/backoffice/login`.
- An ephemeral **file-signing key** invalidates previously-issued signed
  file-download URLs on every restart (the SPA re-fetches them, so it mostly
  self-heals, but bookmarked / in-flight links break).
- An ephemeral **OAuth state key** fails state validation for any OAuth
  sign-in that began before a restart/redeploy or lands on a different replica
  after the provider redirect (only relevant when OAuth sign-in is enabled).

All three are optional (an absent value keeps the ephemeral fallback, and
`apply-secrets.sh` warns rather than fails). Generate each once and keep it
stable:

```bash
# one value per field — 64 hex chars each; set once, don't rotate casually
openssl rand -hex 32   # -> jwt.secret
openssl rand -hex 32   # -> file_signing.key
openssl rand -hex 32   # -> oauth_state.key
```

Rotating `jwt.secret` later is a deliberate act: it logs every session out and
invalidates existing back-office MFA enrollments (re-run
`inventario backoffice mfa setup` afterwards). Per-PR previews are not covered —
they stay on the ephemeral per-pod values by design.

Stash all three for "Filling the bundle".

### 5. Optional: Tailscale auth-key (for `make recover` on a fresh VM)

vcluster-dev is already authenticated to your tailnet, so `make bootstrap`
on this VM doesn't need an auth-key. But for the disaster-recovery scenario
(VM totally gone → spin up new VM → `make recover`), you'll need a reusable
auth-key so `tailscale up` on the fresh VM can join the tailnet headless.

1. <https://login.tailscale.com/admin/settings/keys>
2. **Generate auth key** → tick **Reusable** + **Ephemeral: NO** + **Tags**: same as #3.
3. Set TTL to whatever you're comfortable with (90 days is the max, then it
   needs rotation).

This one is optional for now — if missing, `vm-install.sh` falls back to
warning "run `tailscale up` manually". For Phase 1 with just one VM it's
acceptable to skip.

### 6. Optional: Velero / Cloudflare R2 backup credentials (#1865)

Velero takes a daily, encrypted, off-VM backup of the persistent
`inv-vcl01-longevity` namespace (the demo Postgres + MinIO data) to a
Cloudflare R2 bucket. Skip this whole section to skip the Velero install —
`vm-install.sh` is gated on `velero.s3_*` being present and the preview-env
core works without it.

**6a. Generate the kopia repository password.** This is the encryption key for
the backup repository — set it ONCE and never change it (see the warning in
the architecture paragraph above):

```bash
openssl rand -base64 32     # → velero.encryption_key
```

**6b. Create the R2 bucket.**

1. Cloudflare dashboard → **R2** → **Create bucket**. Name it e.g.
   `inventario-longevity-backups` (→ `velero.s3_bucket`). Pick a location
   close to the VM. Leave it private (R2 buckets are private by default).
2. The bucket's S3 API endpoint is `https://<accountid>.r2.cloudflarestorage.com`
   (R2 → **Overview** shows your account ID; or open the bucket →
   **Settings** → **S3 API**). That whole URL → `velero.s3_endpoint`.
3. Leave `velero.s3_region` empty — it defaults to `auto`, Cloudflare's
   placeholder region.

**6c. Create a scoped R2 API token.**

1. R2 → **Manage R2 API Tokens** → **Create API token**.
2. **Permissions**: **Object Read & Write**.
3. **Specify bucket(s)**: scope it to just the bucket from 6b (least privilege).
4. **Create**. Copy the **Access Key ID** (→ `velero.s3_access_key`) and the
   **Secret Access Key** (→ `velero.s3_secret_key`) — the secret is shown
   **once**.

> R2 (like Backblaze B2 / Ceph) rejects the AWS SDK's default trailing
> checksum; `vm-install.sh` already sets `checksumAlgorithm: ""` on the
> BackupStorageLocation to work around it. Nothing to configure here.

Stash all five values for "Filling the bundle".

---

### 7. Optional: AI-vision provider API key (#1976)

The AI-vision photo-scan feature (#1720) lets users prefill the Add-Item form
from product photos. It's wired but defaults OFF; all preview-stack envs —
`inv-vcl01-master`, `inv-vcl01-longevity`, AND the per-PR previews — pin
`aivision.provider: anthropic`, so they need an Anthropic key to serve scans.

1. Get a key at <https://console.anthropic.com/> → **API keys** (`sk-ant-...`).
2. Put it in `anthropic.api_key` in the bundle (OpenAI users: `openai.api_key`
   instead, and set `aivision.provider: openai` in the ApplicationSet).

`apply-secrets.sh` delivers it two ways:

- **master + longevity**: materialized into the static `inventario-admin` Secret
  as `INVENTARIO_RUN_AI_VISION_ANTHROPIC_API_KEY`; the chart loads it via
  `envFrom`.
- **per-PR previews** (#1976): the namespaces are created on the fly by ArgoCD,
  so the key goes into a separate `inventario-ai-vision` Secret (AI keys only)
  in the `inventario-shared` namespace, annotated for **emberstack/reflector**
  (installed by `vm-install.sh`) to auto-copy into every `inv-vcl01-pr{N}`
  namespace. The PR chart's `extraEnvFrom` then loads it.

> ⚠️ **Ordering / dependency hazard.** The apiserver **fails to boot** when
> `aivision.provider=anthropic` but the key env is empty (intentional fail-loud
> in `wireCommodityScan`). Fill `anthropic.api_key` and run `apply-secrets.sh`
> **before** the `aivision.provider: anthropic` config syncs — otherwise the
> master/longevity apiservers, and **every** PR preview, CrashLoop. A freshly
> created PR preview may also CrashLoop briefly until reflector copies the
> Secret into its new namespace — the kubelet then restarts the pod, which
> re-reads the now-present Secret. To stage it the other
> way round, flip the relevant ApplicationSet's `aivision.provider` back to
> `none`/`mock` until the key is in place.

---

## Filling the bundle

With all the values from steps 1-5 in hand, populate the encrypted bundle:

```bash
cd <repo-root>

# 1. Start from the schema reference (safe placeholders).
cp infra/vm/secrets/secrets.example.yaml infra/vm/secrets/secrets.local.yaml
chmod 600 infra/vm/secrets/secrets.local.yaml

# 2. Fill in real values — admin.{email,password}, jwt.secret +
#    file_signing.key + oauth_state.key (step 4b; recommended for
#    master + longevity, optional),
#    tailscale.{auth_key, oauth_client_id, oauth_client_secret, tailnet_name},
#    github.{app_id, app_installation_id, app_private_key, url}.
#    Fill anthropic.api_key (step 7 above) ONLY to turn the AI-vision feature
#    on for master + longevity — and mind the boot-ordering hazard noted there.
#    Fill the velero.* block (step 6 above) ONLY if you want the daily R2
#    backup of inv-vcl01-longevity; leave it empty to skip the Velero install.
#    Don't leave half-filled velero.* keys — vm-install.sh treats the block as
#    "configured" (and installs Velero) once bucket+endpoint+access+secret are
#    all present. encryption_key is NOT part of that gate, but set it anyway:
#    a backup taken without it gets a random, auto-generated kopia password and
#    CANNOT be restored on a fresh VM (step 6a). Treat encryption_key as
#    restore-critical, same as the age key.
$EDITOR infra/vm/secrets/secrets.local.yaml

# 3. Encrypt. The repo-root .sops.yaml picks the right age recipient
#    automatically because the *.local.yaml path matches a creation_rule.
sops -e infra/vm/secrets/secrets.local.yaml > infra/vm/secrets/secrets.enc.yaml
chmod 600 infra/vm/secrets/secrets.enc.yaml

# 4. Verify round-trip BEFORE deleting plaintext. All expected keys should
#    appear "filled" (velero.* only if you opted into backups in step 2).
sops -d --output-type json infra/vm/secrets/secrets.enc.yaml | jq -r '
  paths(scalars) as $p |
  ($p|join(".")) + ": " +
  (getpath($p) | tostring | if length > 0 then "filled" else "empty" end)
'

# 5. Shred the plaintext.
SIZE=$(wc -c < infra/vm/secrets/secrets.local.yaml)
dd if=/dev/urandom of=infra/vm/secrets/secrets.local.yaml bs=$SIZE count=1 conv=notrunc 2>/dev/null
rm -f infra/vm/secrets/secrets.local.yaml

# 6. End-to-end test against a real VM.
make -C infra bootstrap VM=user@vm-host

# 7. Commit secrets.enc.yaml (+ .sops.yaml if first time) and open a PR.
```

---

## Day-to-day: editing the bundle

After initial setup, edit values with the `sops` interactive editor:

```bash
sops infra/vm/secrets/secrets.enc.yaml
# Opens $EDITOR with the plaintext. Save and exit → sops re-encrypts in place.
```

Then re-run `make -C infra bootstrap VM=...` — bootstrap is idempotent and
will re-apply changed Secret values, then trigger rolling restarts of the
affected workloads (argocd-repo-server / argocd-applicationset-controller on
GitHub App cred changes; tailscale-operator on TS OAuth changes).

---

## Disaster recovery — "the VM is gone"

Scenario: h3 is gone, or VM 101 is unrecoverable, or someone `qm destroy`d it.

What you need on-hand:
- Your laptop with `~/.config/sops/age/keys.txt` intact (or restore from your
  password manager)
- This repo checked out (or re-clonable)
- A fresh Ubuntu 26.04 VM somewhere with ssh access

Recovery:

```bash
# 1. Restore age key from password manager if needed
mkdir -p ~/.config/sops/age
# paste contents into ~/.config/sops/age/keys.txt
chmod 600 ~/.config/sops/age/keys.txt

# 2. Spin up the new VM (any provider — Hetzner, DigitalOcean, Proxmox...)

# 3. ssh-copy-id so passwordless ssh works
ssh-copy-id user@new-vm-ip

# 4. Run recover (alias for bootstrap)
make -C infra recover VM=user@new-vm-ip
```

`bootstrap.sh` decrypts the bundle locally with your age key, ships it to the
fresh VM, and re-creates **the same** admin credentials, **the same** tailnet
hostname (`inv-vcl01`), **the same** GitHub App association, **the same**
Tailscale operator OAuth. From any tailnet peer, `inv-vcl01.<tailnet>.ts.net`
resolves to the new VM, certificates auto-renew, ArgoCD picks up the master
Application again.

You don't need to notify anyone or change any URL. The only off-VM dependency
is your age private key — protect it accordingly.

---

## Key rotation

### Adding a teammate (or rotating the age private key — same flow)

`.sops.yaml` at the repo root accepts a comma-separated list of age
recipients; each can decrypt the bundle independently.

1. Generate a new age key (or get the teammate's public key from theirs):
   `age-keygen -o ~/.config/sops/age/keys2.txt`
2. Edit repo-root `.sops.yaml` so BOTH rules list the new recipient
   alongside the existing one (keep the scoping prefix and both file-type
   regexes — drift here means some files get encrypted with the old key
   only):
   ```yaml
   creation_rules:
     - path_regex: ^infra/vm/secrets/.*\.enc\.yaml$
       age: >-
         age1old...,
         age1new...
     - path_regex: ^infra/vm/secrets/.*\.local\.yaml$
       age: >-
         age1old...,
         age1new...
   ```
3. Re-encrypt the bundle with both recipients: `sops updatekeys infra/vm/secrets/secrets.enc.yaml`
4. Commit + push.
5. (Rotation only) Switch your `~/.config/sops/age/keys.txt` to the new key.
6. (Rotation only) Edit `.sops.yaml` to **remove** the old recipient from
   BOTH rules, run `sops updatekeys` again, commit.
7. (Rotation only) Securely shred `~/.config/sops/age/keys.txt.old` and the
   password-manager copy.

### Rotating the master admin seed password

`admin.password` is the **seed** value the chart's setup Job uses to create
the super-admin on first install of the master Application. After first
install the password lives in the app's own user table; rotating
`admin.password` in the sops bundle does NOT change the running app's
admin login — for that, sign in to the master deployment and rotate from
the app UI.

The bundle field still matters for two reasons:
1. Fresh deploys / disaster recovery seed from this value (see "Disaster
   recovery — the VM is gone" below).
2. The `inv-vcl01-master/inventario-admin` Secret is created from this
   field on every `bootstrap`/`upgrade`. If you ever delete the master
   namespace and re-sync, the chart re-seeds from the current Secret —
   so keep `admin.password` aligned with what's actually in the app's
   user table, or document the divergence.

To update the seed value:

1. `sops infra/vm/secrets/secrets.enc.yaml` → replace `admin.password`.
2. `make -C infra upgrade VM=...` → re-applies the Secret. The setup Job
   reruns under `argocdMode: Force=true,Replace=true` but is idempotent
   in `migrate-data`: it skips re-creating an existing admin user, so
   the new value only takes effect on a clean re-install.

### Rotating the GitHub App private key (App-side compromise, or just hygiene)

GH Apps allow multiple active private keys. Painless flow:

1. App settings → **Generate a private key** → download new PEM.
2. `sops infra/vm/secrets/secrets.enc.yaml` → replace `github.app_private_key`
   with new PEM contents.
3. `make -C infra upgrade VM=...` → applies new Secret to argocd namespace,
   rolling-restarts `argocd-repo-server` + `argocd-applicationset-controller`.
4. Wait 24h (sanity buffer) → App settings → delete old key.

### Rotating the AI-vision provider API key (`anthropic.api_key` / `openai.api_key`)

Safe to rotate any time — at worst an in-flight scan request fails and the user
retries:

1. Provider console → create a new key → keep the old one live for the handoff.
2. `sops infra/vm/secrets/secrets.enc.yaml` → replace `anthropic.api_key` (or
   `openai.api_key`).
3. `make -C infra upgrade VM=...` → re-applies the `inventario-admin` Secret;
   the apiserver picks up the new key on its next rollout.
4. Revoke the old key in the provider console.

> Do NOT blank the key while `aivision.provider` is still `anthropic`/`openai` —
> the apiserver fails to boot on an empty key for a selected real provider. Flip
> the provider to `none` first if you intend to disable the feature.

### Rotating the Tailscale OAuth client secret

1. <https://login.tailscale.com/admin/settings/oauth> → **Revoke** old client (or generate new one first then revoke).
2. Update `tailscale.oauth_client_id` + `tailscale.oauth_client_secret` in
   the sops bundle.
3. `make -C infra upgrade VM=...` → rolls the tailscale-operator pod.

### Rotating the R2 API token (`velero.s3_access_key` / `velero.s3_secret_key`)

Safe to rotate any time — it's the access credential, NOT the repo encryption
key:

1. R2 → **Manage R2 API Tokens** → create a new token (Object Read & Write,
   same bucket) → then revoke the old one.
2. `sops infra/vm/secrets/secrets.enc.yaml` → replace `velero.s3_access_key`
   + `velero.s3_secret_key`.
3. `make -C infra upgrade VM=...` → re-applies `velero/cloud-credentials`.
   Velero/kopia pick up the new credentials on the next backup/maintenance.

### Never rotate `velero.encryption_key`

⚠️ The kopia backup repository in R2 is encrypted with this password. Changing it
means a fresh Velero install can no longer decrypt ANY existing backup — every
prior restore point becomes unrecoverable. If it is ever genuinely compromised,
the only safe path is: provision a brand-new R2 bucket, set a new
`encryption_key` + `s3_bucket`, re-bootstrap, and accept that the old backup
history is gone. Treat the value as write-once, like the age key.

---

## Troubleshooting

- **`sops -d` says "Failed to get the data key required to decrypt"**:
  your age private key doesn't match any recipient in the file. Either
  you're on the wrong laptop, the key was rotated and you lost the password-
  manager backup, or `SOPS_AGE_KEY_FILE` env var points at the wrong path.
- **`bootstrap.sh` warns "secrets.enc.yaml missing"**: file isn't in
  `infra/vm/secrets/`. You're either on a branch where it hasn't landed yet,
  or you haven't pulled the latest master.
- **ArgoCD applicationset-controller crashloops with "github auth failed"**:
  GH App credentials in `secrets.enc.yaml` are stale or wrong. Re-check the
  3 fields, re-encrypt, re-bootstrap.
- **TS operator pod crashloops**: same with `tailscale.oauth_client_{id,secret}`.
  Check Tailscale admin console hasn't revoked the client.
- **`velero backup-location get` shows `PhaseUnavailable`**: the R2 backend
  isn't reachable. Check `kubectl -n velero logs deploy/velero` — usual causes
  are a wrong `velero.s3_endpoint` (must be the full
  `https://<accountid>.r2.cloudflarestorage.com`, no bucket suffix), a revoked
  R2 token, or — if you see `XAmzContentSHA256Mismatch` — a Velero install that
  predates the `checksumAlgorithm: ""` fix (re-run `make upgrade`).
- **Backups exist but a fresh-VM restore fails to decrypt**: the
  `velero-repo-credentials` Secret on the new VM holds a different
  `repository-password` than the one the kopia repo in R2 was created with.
  This happens if `velero.encryption_key` was empty at first backup (Velero
  auto-generated one) or was changed. There is no recovery — see "NEVER rotate
  `velero.encryption_key`" above.
