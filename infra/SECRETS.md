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
turns the JSON into three Kubernetes Secrets (`inv-system/inventario-admin`,
`argocd/github-app-creds`, `tailscale/operator-oauth`).

## Files in this layout

```
infra/vm/secrets/
├── .gitignore               ← already in repo; ignores *.plain.*, secrets.{yaml,json}
├── .sops.yaml               ← sops creation_rules (added once you have an age recipient)
├── secrets.example.yaml     ← schema reference, safe to commit
└── secrets.enc.yaml         ← THE bundle, sops-encrypted
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

Tell me the public key (`age1...`) and I'll write `.sops.yaml` and the
encrypted bundle.

### 2. Create the GitHub App

Inventario's bot uses a GitHub App (not a PAT) to read PR state and clone
the repo. Permissions are read-only — the bot doesn't post comments or
labels.

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

Tell me the **App ID**, **Installation ID**, and paste the **private key PEM**
(I'll put it in the encrypted bundle, never in clear repo).

### 3. Create the Tailscale OAuth client

For the Tailscale Kubernetes Operator (#1855) to create ephemeral tailnet
nodes for each Service/Ingress.

1. Open <https://login.tailscale.com/admin/settings/oauth>
2. Click **Generate OAuth client**.
3. **Description**: `inventario-vcluster-operator`
4. **Scopes**: tick `Devices` → `Write` (operator needs to create devices).
   No other scope.
5. **Tags**: pick or create a tag the operator will own (e.g. `tag:k8s`).
   You'll need to make sure that tag is **owned by** the user/group that has
   permissions to use this OAuth client — see the
   [Tailscale ACL docs](https://tailscale.com/kb/1018/acls#tag-owners).
6. Click **Generate Client**.
7. Copy the **Client ID** (visible from then on in the table) and the
   **Client Secret** (visible **once**, can't be retrieved later — save it
   immediately).

Tell me the **Client ID** and **Client Secret**.

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

The Inventario `inv-system/inventario-admin` Secret holds the super-admin
login the app uses on first start.

```bash
# Generate a strong random password (or use your own)
openssl rand -base64 24
```

Tell me an **email** + the **password** you want baked in.

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

---

## What I'll do once you give me the values above

1. Write `infra/vm/secrets/.sops.yaml` with your age public key as the recipient.
2. Build `infra/vm/secrets/secrets.enc.yaml` populated with all 8-ish values
   (admin / tailscale.{auth_key?, oauth_client_id, oauth_client_secret, tailnet_name} /
   github.{app_id, app_installation_id, app_private_key, url}).
3. Verify locally: `sops -d --output-type json infra/vm/secrets/secrets.enc.yaml | jq .` round-trips.
4. Run `make -C infra bootstrap VM=buster@vcluster-dev` end-to-end on the
   clean VM to confirm:
   - Tailscale operator helm install succeeds
   - `kubectl -n tailscale get pods` shows operator Ready
   - `kubectl -n argocd get secret github-app-creds` exists with our label
   - `kubectl -n inv-system get secret inventario-admin` exists
5. Commit + PR.

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

### Rotating the age private key (you got phished, or laptop stolen)

1. Generate a new age key: `age-keygen -o ~/.config/sops/age/keys2.txt`
2. Edit `.sops.yaml` to add the **new** public key alongside the **old** one:
   ```yaml
   creation_rules:
     - path_regex: \.enc\.yaml$
       age: >-
         age1old...,
         age1new...
   ```
3. Re-encrypt the bundle with both recipients: `sops updatekeys infra/vm/secrets/secrets.enc.yaml`
4. Commit + push.
5. Switch your `~/.config/sops/age/keys.txt` to the new key.
6. Edit `.sops.yaml` to **remove** the old recipient, run `sops updatekeys` again, commit.
7. Securely shred `~/.config/sops/age/keys.txt.old` and the password-manager copy.

### Rotating the GitHub App private key (App-side compromise, or just hygiene)

GH Apps allow multiple active private keys. Painless flow:

1. App settings → **Generate a private key** → download new PEM.
2. `sops infra/vm/secrets/secrets.enc.yaml` → replace `github.app_private_key`
   with new PEM contents.
3. `make -C infra upgrade VM=...` → applies new Secret to argocd namespace,
   rolling-restarts `argocd-repo-server` + `argocd-applicationset-controller`.
4. Wait 24h (sanity buffer) → App settings → delete old key.

### Rotating the Tailscale OAuth client secret

1. <https://login.tailscale.com/admin/settings/oauth> → **Revoke** old client (or generate new one first then revoke).
2. Update `tailscale.oauth_client_id` + `tailscale.oauth_client_secret` in
   the sops bundle.
3. `make -C infra upgrade VM=...` → rolls the tailscale-operator pod.

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
