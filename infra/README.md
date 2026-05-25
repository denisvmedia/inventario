# Inventario preview-env

PR previews on a single Ubuntu VM behind Tailscale, driven by ArgoCD's
ApplicationSet PR generator. Label an open PR `preview` → ~5 minutes later
the URL appears in a sticky PR comment. Tailnet members open it, click around,
log in with the dev admin, see the seeded demo data. Unlabel or close the PR →
preview goes away. Push a new commit → preview rolls forward.

This document is the start-to-finish guide a brand-new contributor can follow
without asking anyone questions. For the architectural deep-dive (component
diagrams, why-this-not-that) see [`devdocs/infra/dev/README.md`](../devdocs/infra/dev/README.md).
For the credentials walkthrough alone see [`SECRETS.md`](./SECRETS.md).

> Epic: [#1852](https://github.com/denisvmedia/inventario/issues/1852).
> Implementation issues: [#1853](https://github.com/denisvmedia/inventario/issues/1853),
> [#1854](https://github.com/denisvmedia/inventario/issues/1854),
> [#1855](https://github.com/denisvmedia/inventario/issues/1855),
> [#1856](https://github.com/denisvmedia/inventario/issues/1856),
> [#1858](https://github.com/denisvmedia/inventario/issues/1858),
> [#1863](https://github.com/denisvmedia/inventario/issues/1863).

---

## Table of contents

1. [What this is](#1-what-this-is)
2. [Prerequisites](#2-prerequisites)
3. [First-time setup](#3-first-time-setup)
4. [Day-to-day](#4-day-to-day)
5. [Troubleshooting](#5-troubleshooting)
6. [Disaster recovery](#6-disaster-recovery)
7. [Key rotation](#7-key-rotation)
8. [Phase 2 (future)](#8-phase-2-future)
9. [Files in this directory](#9-files-in-this-directory)
10. [Pinned versions](#10-pinned-versions)

---

## 1. What this is

`make -C infra bootstrap` brings a clean Ubuntu 26.04 VM up to a state where
labeling a PR with `preview` produces a working URL for any tailnet member to
click on. The cluster is single-VM [vcluster](https://www.vcluster.com)
standalone with [ArgoCD](https://argo-cd.readthedocs.io) and the
[Tailscale Kubernetes Operator](https://tailscale.com/kb/1236/kubernetes-operator)
on top.

```
Contributor's GitHub PR              Tailnet member's browser
        |                                       |
        | add label `preview`                   | https://inv-vcl01-prN.<tailnet>.ts.net/
        v                                       v
+----------------+                    +-------------------+
|  ArgoCD        |                    |  Tailscale        |
|  ApplicationSet|--(poll every 60s)->|  coordination     |
|  PR generator  |                    |  server (cloud)   |
+--------+-------+                    +---------+---------+
         |                                      |
         | create Application                   | route HTTPS
         | inv-vcl01-prN                        | to per-PR proxy
         v                                      v
+--------------------------------------------------------------+
|  VM: Ubuntu 26.04, tailscaled, vcluster standalone (k8s)     |
|                                                              |
|     +----------------------+   +--------------------------+  |
|     | argocd namespace     |   | tailscale namespace      |  |
|     |   ArgoCD core pods   |   |   TS Operator            |  |
|     |   ApplicationSet     |   |   per-Ingress proxy pods |  |
|     |   controller         |   |   ts-orphan-cleanup      |  |
|     +----------------------+   |     (hourly CronJob)     |  |
|                                +--------------------------+  |
|                                                              |
|     +---------------------------------------------+          |
|     | inv-vcl01-prN namespace (one per PR)        |          |
|     |   Inventario Deployment (Go API+workers)    |          |
|     |   demo Postgres / Redis / MinIO sidecars    |          |
|     |   setup Job (migrate + seed)                |          |
|     |   Ingress (className: tailscale)            |          |
|     +---------------------------------------------+          |
+--------------------------------------------------------------+
```

Nothing is exposed to the public internet. The VM joins your Tailscale tailnet
under a stable hostname (default `inv-vcl01`); every per-PR Ingress provisions
its own tailnet device under `inv-vcl01-prN` and serves HTTPS with a
Tailscale-managed Let's Encrypt cert for the FQDN
`inv-vcl01-prN.<tailnet>.ts.net`.

---

## 2. Prerequisites

You need these tools on your laptop:

| Tool | Why |
|---|---|
| `ssh`, `scp` | Bootstrap orchestrator talks to the VM over SSH. |
| `sops` | Decrypts the secrets bundle locally. |
| `age` | Asymmetric crypto sops uses for our recipients. |
| `make` | Orchestrator entry point. |
| `kubectl` | Optional — bootstrap installs it on the VM, but you'll want it locally to inspect Applications + port-forward ArgoCD. |
| `helm` | Optional — same. |
| `tailscale` | The tailnet client. You need to be a member of the same tailnet the VM joins, or you can't reach the previews. |

### macOS (Homebrew)

```bash
brew install sops age kubectl helm
brew install --cask tailscale         # menubar app + CLI
# ssh, scp, make ship with Xcode CLT (xcode-select --install)
```

### Linux

`kubectl` and `helm` are not in the default Debian/Ubuntu/Fedora apt/dnf
repositories — the official install methods (binary download / get-helm-3
script) work uniformly across distros and avoid per-distro repo-add ceremony.
`sops` ships as a `.deb`/`.rpm` from the upstream releases page.

```bash
# Debian / Ubuntu
sudo apt-get update
sudo apt-get install -y openssh-client age make curl jq

# kubectl: official binary install
curl -fsSLO "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && rm kubectl

# helm: official install script
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# sops: latest .deb from GitHub releases
SOPS_VER=$(curl -fsSL https://api.github.com/repos/getsops/sops/releases/latest \
  | jq -r '.tag_name | ltrimstr("v")')
curl -fsSL -o /tmp/sops.deb \
  "https://github.com/getsops/sops/releases/download/v${SOPS_VER}/sops_${SOPS_VER}_amd64.deb"
sudo dpkg -i /tmp/sops.deb && rm /tmp/sops.deb

# tailscale: official installer
curl -fsSL https://tailscale.com/install.sh | sh
```

```bash
# Arch — kubectl/helm/sops all in extra
sudo pacman -S openssh sops age kubectl helm make
yay -S tailscale-bin                  # AUR for the desktop GUI; or pacman for cli-only
```

```bash
# Fedora — same approach as Debian for kubectl/helm/sops since defaults vary
sudo dnf install -y openssh-clients age make curl jq

# kubectl: official binary install (same as Debian block above).
curl -fsSLO "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo install -o root -g root -m 0755 kubectl /usr/local/bin/kubectl && rm kubectl

# helm: official install script
curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash

# sops: latest .rpm from GitHub releases
SOPS_VER=$(curl -fsSL https://api.github.com/repos/getsops/sops/releases/latest \
  | jq -r '.tag_name | ltrimstr("v")')
sudo dnf install -y "https://github.com/getsops/sops/releases/download/v${SOPS_VER}/sops-${SOPS_VER}-1.x86_64.rpm"

sudo dnf install -y "https://pkgs.tailscale.com/stable/fedora/$(rpm -E %fedora)/tailscale.rpm"
```

Then verify:

```bash
make -C infra preflight
# → "Laptop prereqs OK"
```

`preflight` only checks `ssh scp sops age`; `kubectl`/`helm`/`tailscale` are
checked the first time you actually use them.

---

## 3. First-time setup

This walkthrough sets up the encryption key, the GitHub App, the Tailscale
OAuth client, the secrets bundle, and the VM, in that order. Allow about
45 minutes the first time; the long tail is Tailscale + GitHub UI clicks
that you do exactly once.

The detailed step-by-step for each credential lives in
[`SECRETS.md`](./SECRETS.md). What follows is the high-level order of
operations + cross-references.

### 3.1 Generate (and back up) your age key

```bash
mkdir -p ~/.config/sops/age
age-keygen -o ~/.config/sops/age/keys.txt
chmod 600 ~/.config/sops/age/keys.txt
```

The file contains both your public and private key. The public key (line
starting with `# public key:`) is what you add to `.sops.yaml` so the bundle
can be encrypted to you.

> [!WARNING]
> **Back up the entire `~/.config/sops/age/keys.txt` file to your password
> manager (1Password / Bitwarden / KeePass) right now.** Losing it means
> losing access to every encrypted secret in this repo with no recovery
> path. Disaster recovery (see [section 6](#6-disaster-recovery)) starts
> from "I have the age key" — without it you cannot bring the system back up
> on a new VM regardless of what else you have.

Full walkthrough including how to add your public key to `.sops.yaml` and
re-encrypt the bundle for additional teammates: [SECRETS.md §1 "Generate the
age keypair"](./SECRETS.md#1-generate-the-age-keypair).

### 3.2 Create the GitHub App

The ArgoCD ApplicationSet PR generator uses a GitHub App to:
- list open PRs on `denisvmedia/inventario` (so it knows which PRs are
  labeled `preview`)
- read repo contents (so ArgoCD's repo-server can `git clone` the chart
  at the PR's head commit)

Walkthrough: [SECRETS.md §2 "Create the GitHub App"](./SECRETS.md#2-create-the-github-app).

At the end you should have three values to keep:

- App ID (numeric, e.g. `1234567`)
- Installation ID (numeric, e.g. `87654321`)
- Private key (`.pem` file you download once; can't be re-downloaded)

### 3.3 Create the Tailscale OAuth client

The Tailscale Operator uses an OAuth client to mint auth keys for each
per-Ingress proxy device. The same client carries the `devices:core` scope
that the hourly orphan-cleanup CronJob uses to delete stale device records.

Walkthrough: [SECRETS.md §3 "Create the Tailscale OAuth client"](./SECRETS.md#3-create-the-tailscale-oauth-client).

> [!IMPORTANT]
> One OAuth client, two scopes both checked: **Auth Keys → Write** AND
> **Devices → Core: Read+Write**. Both are scoped to tag `tag:k8s`.
> Without `devices:core`, the orphan cleanup CronJob will fail every
> hour with "OAuth client cannot grant scopes 'devices:core'".

You also need to make sure your tailnet ACL has the right `tagOwners`
section — see [SECRETS.md §3 "Tailscale ACL — tagOwners"](./SECRETS.md#tailscale-acl--tagowners-one-time-easy-to-forget).

### 3.4 Pick the admin email + password

The `admin.email` / `admin.password` fields in the sops bundle are reserved
for a future production overlay — they are NOT what PR-preview environments
log in with today. The preview overlay
[`infra/helm-overlays/preview-base.values.yaml`](./helm-overlays/preview-base.values.yaml)
hard-codes `admin@example.com` / `PreviewAdmin123` for the seeded admin user,
and that's what you use to sign into a preview URL. Reading the sops bundle
for prod-style admin credentials is tracked under
[#1883](https://github.com/denisvmedia/inventario/issues/1883); for Phase 1
you can put any non-empty placeholder values in `admin.email` /
`admin.password` and they'll simply be ignored by the chart.

Walkthrough: [SECRETS.md §4 "Pick the admin email + password"](./SECRETS.md#4-pick-the-admin-email--password).

### 3.5 Fill the encrypted bundle

```bash
# 1. Start from the schema reference.
cp infra/vm/secrets/secrets.example.yaml /tmp/secrets.plain.yaml

# 2. Fill in real values: admin.{email,password},
#    tailscale.{auth_key, oauth_client_id, oauth_client_secret, tailnet_name},
#    github.{app_id, app_installation_id, app_private_key, url}.
$EDITOR /tmp/secrets.plain.yaml

# 3. Encrypt into the repo. `.sops.yaml` at the repo root picks the right
#    age recipient automatically.
sops --encrypt /tmp/secrets.plain.yaml > infra/vm/secrets/secrets.enc.yaml

# 4. Round-trip check BEFORE deleting the plaintext.
sops --decrypt infra/vm/secrets/secrets.enc.yaml | diff - /tmp/secrets.plain.yaml

# 5. Remove the plaintext. `shred` is GNU coreutils — present on most Linux
#    distros but NOT on macOS (and `srm` was removed from macOS too). On a
#    FileVault-encrypted Mac, `rm` is fine — `/tmp` lives on the encrypted
#    volume, so the plaintext is unrecoverable after shutdown. On Linux
#    without coreutils, install `shred` (`apt install coreutils`, etc.) or
#    fall back to `rm` if your disk is LUKS-encrypted.
rm /tmp/secrets.plain.yaml
# Linux with coreutils, defence-in-depth:
#   shred -u /tmp/secrets.plain.yaml
```

Full walkthrough: [SECRETS.md §"Filling the bundle"](./SECRETS.md#filling-the-bundle).

### 3.6 Provision a VM

Any Ubuntu 26.04 server with passwordless SSH and outbound internet works.
Sizing target for Phase 1: 4 vCPU, 8 GiB RAM, ~40 GiB disk.

A few options:

- **Hetzner Cloud** ([https://www.hetzner.com/cloud](https://www.hetzner.com/cloud))
  CX32 (€7-8/mo). Cheapest reliable. Pick the Ubuntu 26.04 image, paste your
  SSH public key during creation.
- **DigitalOcean** ([https://www.digitalocean.com](https://www.digitalocean.com))
  Basic 4 vCPU / 8 GiB droplet (~$48/mo). Slightly pricier; nicer dashboard.
- **Proxmox / homelab** — manual create from the Ubuntu 26.04 cloud image,
  bring up with cloud-init or your usual workflow. Snapshot support is a
  bonus (lets you roll back during testing).

Once it's up, make sure SSH works passwordlessly:

```bash
ssh-copy-id user@your-vm-ip       # one-shot install of your pubkey
ssh user@your-vm-ip 'echo ok'     # should print "ok" with no password prompt
```

### 3.7 Bootstrap

```bash
make -C infra bootstrap VM=user@your-vm-ip
```

Expected timing on a fresh VM: 3-5 minutes. The script:

1. Forces NTP resync on the VM (defensive against cloud-image clock skew).
2. Waits up to 10 minutes for `unattended-upgrades` to release the dpkg lock.
3. Installs `curl jq ca-certificates conntrack socat` via apt.
4. Installs and authenticates Tailscale on the VM (hostname: `inv-vcl01`).
5. Installs vcluster standalone (kubectl + helm bundled).
6. Installs the Tailscale Kubernetes Operator via helm (OAuth from sops bundle).
7. Installs ArgoCD via helm.
8. Materializes Kubernetes Secrets from the sops bundle.
9. Applies the `ts-orphan-cleanup` CronJob to the `tailscale` namespace.
10. Applies the ArgoCD `AppProject`, `Application` (master), and
    `ApplicationSet` (PR previews).
11. Copies the kubeconfig back to `~/.kube/inv-vcl01.config` on your laptop.

The script is **idempotent** — re-running it is the upgrade path. The
`make upgrade` and `make recover` targets are aliases for `make bootstrap`.

### 3.8 Verify

```bash
# 1. The VM is on your tailnet.
ssh user@your-vm-ip 'tailscale status --self=true --peers=false'
# expect: "inv-vcl01 ..." with an IP starting 100.x

# 2. The cluster is up.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl get nodes
# expect: 1 node, "Ready"

# 3. All ArgoCD pods are Ready.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd get pods
# expect: 5 pods (server, repo-server, application-controller,
#         applicationset-controller, redis), all 1/1 Running

# 4. The ApplicationSet exists.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd get appset inventario-pr-previews
# expect: "inventario-pr-previews ..."

# 5. The static master Application exists.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd get application inv-vcl01-master
# expect: STATUS becomes Synced/Healthy within ~5 min (waits for first
#         sha-master image build to finish on GHCR)

# 6. The ArgoCD UI loads.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd port-forward svc/argocd-server 8080:80 &
open http://localhost:8080         # macOS; on Linux: xdg-open
# Admin password:
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath='{.data.password}' | base64 -d
```

If any of those don't behave as expected: [section 5 Troubleshooting](#5-troubleshooting).

---

## 4. Day-to-day

### Trigger a preview

Add the `preview` label to any open PR on
[denisvmedia/inventario](https://github.com/denisvmedia/inventario). Within
~60 seconds, ArgoCD's ApplicationSet PR generator polls GitHub, sees the
label, and spawns `Application inv-vcl01-pr<N>`. Within ~5 minutes
(faster on cached image builds), the preview is reachable at
`https://inv-vcl01-pr<N>.<your-tailnet-name>.ts.net/`.

A sticky comment on the PR shows the deployed hostname, the head SHA, and
the admin credentials (`admin@example.com` / `PreviewAdmin123` for the seeded
dev login). The hostname is shown as plain text on purpose — see
[devdocs note](../devdocs/infra/dev/README.md#7-github-workflows) for why.

### Tear down a preview

Either remove the `preview` label from the PR, OR close/merge the PR. The
ApplicationSet PR generator picks up the change on its next poll cycle
(~60 s), deletes the Application, and ArgoCD's `automated.prune: true`
removes the namespace + every resource in it. Timeline: ~1.5-2 minutes from
unlabel to namespace gone.

The Tailscale device record for the proxy is removed within the next hour
by the `ts-orphan-cleanup` CronJob (in-cluster, runs at the top of every
hour). To force immediately:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n tailscale \
  create job --from=cronjob/ts-orphan-cleanup ts-cleanup-now
```

Note: the script only deletes devices that have been **offline for > 1
hour**, so an immediate trigger right after unlabel won't catch it — the
1 h guard is intentional, it prevents flap-during-reschedule deletes. Wait
about an hour OR delete the device manually in the
[TS admin Machines](https://login.tailscale.com/admin/machines) view.

### Inspect a running preview

There is no direct ArgoCD-UI exposure on the tailnet today (planned in
[#1892](https://github.com/denisvmedia/inventario/issues/1892)). Until then,
port-forward from your laptop:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd \
  port-forward svc/argocd-server 8080:80
# open http://localhost:8080

# Admin password (only needed first time; you can rotate after):
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd \
  get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d
```

For raw kubectl access:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n inv-vcl01-pr<N> get pods
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n inv-vcl01-pr<N> logs deploy/inv-vcl01-pr<N>-inventario
```

### master deployment

`Application inv-vcl01-master` tracks the master branch and is reachable at
`https://inv-vcl01-master.<your-tailnet-name>.ts.net/`. **Important caveat**:
master pushes do NOT trigger an automatic re-deploy today. The chart
references the moving `image.tag: master` (constant text across commits),
so ArgoCD sees no manifest diff and never re-applies; the running pods stay
on whatever digest they pulled at last restart. To pick up a new master
image manually:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n inv-vcl01-master \
  rollout restart deploy/inv-vcl01-master-inventario
```

(`imagePullPolicy: Always` ensures the next pull grabs the new digest.)
Proper auto-rollout — a git-generator ApplicationSet that templates the
head SHA into `image.tag` so each master push produces a real manifest
diff — is tracked under
[#1885](https://github.com/denisvmedia/inventario/issues/1885).

### Redeploy at the same commit

| You want | Use |
|---|---|
| Fully fresh state (fresh DB, re-seed) | Remove `preview` label, wait ~60 s, re-add. ApplicationSet destroys + recreates. Most predictable. |
| Just restart the app pod (Postgres preserved) | `kubectl -n inv-vcl01-pr<N> rollout restart deploy/inv-vcl01-pr<N>-inventario` |
| Re-apply chart manifests at same commit | ArgoCD UI → application → Sync with **Force + Replace** checked. |

selfHeal handles cluster-side drift continuously — `kubectl delete` anything
ArgoCD knows about, and it gets recreated within seconds. So "I broke a pod
with kubectl" is self-correcting; no manual re-sync needed.

### Make targets

| Target | What it does |
|---|---|
| `make -C infra preflight` | Verifies laptop tools (`ssh`, `scp`, `sops`, `age`). Doesn't need `VM=`. |
| `make -C infra bootstrap VM=user@host` | Brings a clean VM up to a working preview-env host (or upgrades an existing one — idempotent). |
| `make -C infra upgrade VM=user@host` | Alias for `bootstrap`. Use semantically when running a version bump on an already-working VM. |
| `make -C infra recover VM=user@host` | Alias for `bootstrap`. Use semantically after total VM loss → fresh VM → `ssh-copy-id` → `make recover`. |
| `make -C infra status VM=user@host` | Snapshot of vcluster + tailscaled + kubectl get nodes,pods -A. |
| `make -C infra logs VM=user@host` | Tails `journalctl -u vcluster.service`. Override the unit with `SVC=tailscaled.service`, etc. |
| `make -C infra shell VM=user@host` | Plain SSH into the VM. |
| `make -C infra destroy VM=user@host` | Stops vcluster and wipes `/var/lib/vcluster` + `/etc/vcluster`. Tailscale membership preserved. Doesn't touch the VM's OS. |

---

## 5. Troubleshooting

### Preview not appearing after labeling

1. Confirm the label is exactly `preview` (case-sensitive — `Preview` won't match).
2. Wait 60-90 seconds for the next ApplicationSet poll.
3. If still nothing:
   ```bash
   KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd \
     describe appset inventario-pr-previews
   ```
   Look at `.status.conditions` for `ParametersGenerated: False` with an
   error message — typically "401 Bad credentials" → GitHub App credentials
   problem ([SECRETS.md §"GH App"](./SECRETS.md#2-create-the-github-app)).
4. Check the controller logs:
   ```bash
   KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd \
     logs deploy/argocd-applicationset-controller --tail=100
   ```

### Application stuck `Progressing` with `ImagePullBackOff`

The PR's image (`sha-<7>`) hasn't been published to GHCR yet. Open the
PR's Actions tab on GitHub and check the `Docker image` workflow:

- Still running → wait 3-5 minutes for the build, ArgoCD will pull on retry.
- `success` with `build` skipped → check `.github/filters.yml` `image_inputs`
  filter; the PR may have only touched paths that don't trigger a build.
  Workaround: push an empty commit to force a fresh build, OR add the path
  to the filter.
- Failed → fix the build, push, sync continues.

If you're certain the image exists:

```bash
docker manifest inspect ghcr.io/denisvmedia/inventario:sha-<head-7-chars>
# should return a JSON manifest, not "manifest unknown"
```

### Application stuck `Progressing` with the app pod in CrashLoop

Look at the app pod's logs:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n inv-vcl01-pr<N> \
  logs deploy/inv-vcl01-pr<N>-inventario --tail=50
```

- `"database schema lags the binary's embedded migrations"` — the setup Job
  hasn't run yet (or is still running). Check:
  ```bash
  KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n inv-vcl01-pr<N> \
    get jobs
  KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n inv-vcl01-pr<N> \
    logs job/inv-vcl01-pr<N>-inventario-setup --all-containers --tail=50
  ```
  Once the Job is Completed, the app pod will recover within its next
  backoff retry (~30 s).
- Other crash reason — file an issue with the log.

### TS Operator pod stuck or crashlooping

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n tailscale \
  logs deploy/operator --tail=100
```

Common causes:
- `OAuth client cannot grant scopes` → wrong scope name in the script
  (must be `auth_keys` and `devices:core` for the operator + cleanup
  CronJob to share the client). See [SECRETS.md §"Tailscale OAuth"](./SECRETS.md#3-create-the-tailscale-oauth-client).
- `unauthorized: invalid client_id or client_secret` → OAuth client rotated
  or revoked; update the sops bundle and re-run `make bootstrap`.

### Per-PR Ingress :443 not reachable from a tailnet peer

```bash
# From any tailnet machine:
tailscale status | grep inv-vcl01-pr<N>
# expect: an IP starting 100.x and "Connected" or recent activity
```

If absent or offline, check the operator-provisioned StatefulSet:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n tailscale \
  get sts,pods -l tailscale.com/parent-resource-ns=inv-vcl01-pr<N>
```

If the device shows as `inv-vcl01-pr<N>-1` (or `-2`) instead of the
plain hostname, you've hit the hostname-collision case — a previous
deploy left an orphan tailnet device that hasn't been cleaned up yet.
Force the cleanup CronJob or wait an hour; see
[section 4 "Tear down a preview"](#tear-down-a-preview).

### ArgoCD UI returns 401 after rotating the admin secret

The initial admin Secret (`argocd-initial-admin-secret`) only contains the
bootstrap-time password and is sometimes removed by the chart after first
login. `argocd admin initial-password` only reads that Secret back — it does
NOT generate a new password. To actually reset, patch `argocd-secret` with
a fresh bcrypt hash:

```bash
# 1. Pick a new password and bcrypt it. htpasswd is in apache2-utils
#    (Debian/Ubuntu) or httpd-tools (Fedora). On macOS: brew install httpd.
NEW_PASS='your-new-password'
NEW_HASH=$(htpasswd -nbBC 10 "" "$NEW_PASS" | tr -d ':\n' | sed 's/^\$2y/\$2a/')

# 2. Patch the Secret. passwordMtime forces ArgoCD to re-read on next request.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd patch secret argocd-secret \
  -p "{\"stringData\": {\"admin.password\": \"${NEW_HASH}\", \"admin.passwordMtime\": \"$(date +%FT%T%Z)\"}}"

# 3. Restart argocd-server to drop any cached auth.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd \
  rollout restart deploy/argocd-server
```

Log in with `admin` / `$NEW_PASS`.

### GitHub App API rate limit ("403 API rate limit exceeded")

Less common — the App PR generator has a per-installation budget that's
generous. If you hit it:

```bash
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd \
  logs deploy/argocd-applicationset-controller | grep -i ratelimit
```

Bump `requeueAfterSeconds` in `infra/argocd/applicationset-pr.yaml` (default
60 → 180) and re-apply. The trade-off is slower preview discovery.

### Stale Tailscale device records accumulating

In normal operation, the `ts-orphan-cleanup` CronJob handles this hourly.
If you see devices piling up despite the CronJob:

```bash
# Find the most recent ts-orphan-cleanup Job (CronJobs suffix the Job name
# with a timestamp), then tail its pod's logs. `kubectl logs job/<name>`
# resolves to the Job's pods automatically.
JOB=$(KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n tailscale get jobs \
        -o jsonpath='{range .items[*]}{.metadata.name} {.metadata.creationTimestamp}{"\n"}{end}' \
      | grep '^ts-orphan-cleanup-' | sort -k2 | tail -1 | awk '{print $1}')
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n tailscale \
  logs "job/${JOB}" --tail=50 | grep -E 'orphan|OAuth|tag:k8s|Deleted'
```

- `OAuth client cannot grant scopes "devices:core"` → the OAuth client is
  missing the Devices Core scope (or only has Auth Keys). Fix in
  [TS admin OAuth settings](https://login.tailscale.com/admin/settings/oauth):
  edit the existing client, tick **Devices → Core: Read + Write** on
  `tag:k8s`, save. Within an hour the next CronJob run will succeed.
- "No orphan tailnet devices to clean up" → CronJob is working; devices that
  show up in the admin UI may have been offline < 1 hour, or fail the
  `tag:k8s` / hostname-regex filter (anything not matching
  `inv-vcl01-(prN|master)(-N)?$` is intentionally ignored to avoid surprise
  deletes).

Manual cleanup via the [admin Machines view](https://login.tailscale.com/admin/machines)
is always an option — filter by `tag:k8s`, remove offline rows.

### bootstrap fails on `apt-get update` with "Release file ... is not valid yet"

VM clock skew. The bootstrap NTP resync didn't converge in 30 s — check VM
network egress to NTP servers, or wait a minute and re-run `make bootstrap`.

### bootstrap fails with "Could not get lock /var/lib/dpkg/lock-frontend"

`unattended-upgrades` is running first-boot refresh, the bootstrap wait
timed out (default 10 min). Wait another 5-10 minutes and re-run
`make bootstrap`. On a chronically slow image, bump `dpkg_timeout` near the
top of [`vm/vm-install.sh`](./vm/vm-install.sh).

---

## 6. Disaster recovery

> Full walkthrough in [SECRETS.md §"Disaster recovery"](./SECRETS.md#disaster-recovery--the-vm-is-gone).
> What follows is the abbreviated runbook.

Scenario: "the VM is gone" (host died, someone `qm destroy`d it, datacenter
fire). You have your laptop and your age key in your password manager.

```bash
# 1. Restore age key from password manager if it isn't on this laptop already
mkdir -p ~/.config/sops/age
$EDITOR ~/.config/sops/age/keys.txt        # paste contents, save
chmod 600 ~/.config/sops/age/keys.txt

# 2. Spin up a fresh Ubuntu 26.04 VM (any provider).

# 3. Passwordless ssh.
ssh-copy-id user@new-vm-ip

# 4. Run recover (alias for bootstrap; idempotent).
make -C infra recover VM=user@new-vm-ip

# 5. Verify (~5 minutes after recover finishes).
make -C infra status VM=user@new-vm-ip
KUBECONFIG=~/.kube/inv-vcl01.config kubectl -n argocd get applications
# expect: inv-vcl01-master + any inv-vcl01-prN matching currently-labeled PRs
```

Recovery preserves:

- **Tailscale hostname.** The new VM joins under the same `inv-vcl01`
  identity (via the same OAuth client minting a fresh auth key). MagicDNS
  routes to the new VM automatically — bookmarks and saved URLs keep
  working with no user-side action.
- **ArgoCD admin credentials, GitHub App, OAuth client.** All sourced from
  the sops bundle on disk.
- **Open PR previews.** ApplicationSet polls GitHub on first run and
  re-creates an Application for every PR still labeled `preview`. ~5
  minutes after recover, all previews are back.

What is NOT preserved:

- **Per-preview Postgres / Redis / MinIO data.** By design — previews are
  ephemeral. Setup Job re-seeds on first install.

> [!IMPORTANT]
> The single off-VM dependency is your age private key. If you lose both the
> VM and the age key, there is no recovery path — you'd start over from
> [section 3 "First-time setup"](#3-first-time-setup) with a fresh sops
> bundle. **Keep the age key backed up in your password manager.**

---

## 7. Key rotation

Walked through in detail in [SECRETS.md §"Key rotation"](./SECRETS.md#key-rotation):

- Adding a teammate as a sops recipient
- Rotating the age private key
- Rotating the GitHub App private key
- Rotating the Tailscale OAuth client secret

In all four cases the workflow is the same: edit `infra/vm/secrets/secrets.enc.yaml`
via `sops`, re-apply with `make -C infra bootstrap VM=user@host`.

---

## 8. Phase 2 (future)

Out of scope for the current preview-env work. Each has its own dedicated
issue:

- **[#1865](https://github.com/denisvmedia/inventario/issues/1865) Velero
  backup/restore for cluster state.** Today, `make recover` re-bootstraps
  from scratch; Velero will let it restore additional cluster state (e.g.,
  manually-created namespaces, persistent volumes) so the recovery is
  closer to a clone.
- **[#1866](https://github.com/denisvmedia/inventario/issues/1866) Resource
  quotas per preview namespace.** Today each preview can consume up to the
  VM's total budget; under load this would degrade other previews. Quotas
  cap per-PR CPU/memory/storage.
- **[#1892](https://github.com/denisvmedia/inventario/issues/1892) Expose
  ArgoCD UI + vcluster API as Tailscale services.** Today both reach via
  `kubectl port-forward` from a laptop with the kubeconfig; the issue
  surfaces them as proper tailnet hostnames (`argocd-inv-vcl01.<tailnet>.ts.net`,
  `k8s-inv-vcl01.<tailnet>.ts.net`) so any tailnet member can bookmark.

Other open follow-ups touch the chart or workflow rather than the runtime:
[#1882](https://github.com/denisvmedia/inventario/issues/1882) (TLS
secretName chart cleanup), [#1883](https://github.com/denisvmedia/inventario/issues/1883)
(preview admin password from sops), [#1884](https://github.com/denisvmedia/inventario/issues/1884)
(in-place ArgoCD upgrades with migrations), [#1885](https://github.com/denisvmedia/inventario/issues/1885)
(master Application auto-rollout on master push).

---

## 9. Files in this directory

```
.sops.yaml                    ← REPO ROOT: creation_rules + age recipients
                                (sops walks up from $PWD to find it)
infra/
├── Makefile                  preflight bootstrap upgrade recover destroy status logs shell
├── README.md                 this file — the start-to-finish guide
├── SECRETS.md                credentials walkthrough (age, GH App, TS OAuth, DR)
├── argocd/                   ArgoCD manifests applied by bootstrap.sh
│   ├── appproject.yaml         AppProject scoping previews + master to inv-vcl01-* ns
│   ├── application-master.yaml static Application tracking master branch
│   └── applicationset-pr.yaml  ApplicationSet PR generator (label=preview)
├── helm-overlays/
│   └── preview-base.values.yaml  shared overlay layered on top of the chart
└── vm/
    ├── bootstrap.sh          laptop orchestrator (ssh, sops, upload, apply)
    ├── vm-install.sh         remote installer (tailscale, vcluster, TS-op, ArgoCD)
    ├── destroy.sh            remote teardown (vcluster + data; tailnet preserved)
    ├── helm-values/          static chart overlays vm-install.sh layers OAuth on top of
    ├── cluster-extras/
    │   └── ts-orphan-cleanup.yaml  hourly TS device GC CronJob
    ├── scripts/
    │   └── apply-secrets.sh  translate sops bundle into k8s Secrets
    └── secrets/
        ├── .gitignore
        ├── secrets.example.yaml  schema reference (safe to commit)
        └── secrets.enc.yaml      sops-encrypted bundle (you create this in 3.5)
```

---

## 10. Pinned versions

| Component | Version | Where pinned |
|---|---|---|
| vcluster standalone | `v0.34.0` | [`infra/vm/vm-install.sh`](./vm/vm-install.sh) `VCLUSTER_VERSION` |
| Kubernetes (inside vcluster) | `v1.34.0` | [`infra/vm/vm-install.sh`](./vm/vm-install.sh) `K8S_VERSION` |
| Tailscale CLI | latest from the official installer | [`infra/vm/vm-install.sh`](./vm/vm-install.sh) (idempotent re-install) |
| Tailscale Operator chart | latest in `tailscale/tailscale-operator` repo | (not pinned yet) |
| ArgoCD chart | latest in `argo/argo-cd` repo | (not pinned yet) |

The two "not pinned yet" rows will get version pins in a follow-up; the
trade-off in Phase 1 was time-to-first-preview over reproducibility.
