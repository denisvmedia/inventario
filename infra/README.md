# Inventario preview-env infra

PR-preview environments served from a single Ubuntu 26.04 VM, exposed via
Tailscale. Continuous delivery via ArgoCD ApplicationSet PR Generator (label
`preview` on a PR → one `Application` per PR → namespace `inv-vcl01-prNNNN`).

Tracking: epic [#1852](https://github.com/denisvmedia/inventario/issues/1852),
this skeleton landed in [#1853](https://github.com/denisvmedia/inventario/issues/1853).

> Full beginner README lives in [#1863](https://github.com/denisvmedia/inventario/issues/1863).
> This file is a thin operating reference for the orchestration this issue ships.

## What this directory ships (#1853)

```
infra/
├── Makefile               targets: bootstrap upgrade recover destroy status logs shell
├── README.md              this file
├── SECRETS.md             secrets setup walkthrough (age, GH App, TS OAuth, DR) — #1854
└── vm/
    ├── bootstrap.sh       laptop orchestrator — ssh, sops, upload, apply
    ├── vm-install.sh      remote installer — tailscale, vcluster, TS-op, ArgoCD
    ├── destroy.sh         remote teardown — vcluster + data; tailnet preserved
    ├── scripts/
    │   └── apply-secrets.sh   translate sops bundle into k8s Secrets
    └── secrets/
        ├── .gitignore
        ├── .sops.yaml             creation_rules → age recipient (added in #1854)
        ├── secrets.example.yaml   schema reference (safe to commit)
        └── secrets.enc.yaml       sops-encrypted bundle (added in #1854)
```

## Quick start

```bash
# Laptop prerequisites (macOS Homebrew or Debian/Ubuntu apt).
make -C infra preflight     # checks: ssh scp sops age

# Bootstrap a fresh Ubuntu 26.04 VM with passwordless ssh.
make -C infra bootstrap VM=user@your-vm-ip

# Verify.
KUBECONFIG=~/.kube/inv-vcl01.config kubectl get nodes,pods -A
make -C infra status VM=user@your-vm-ip
```

The `bootstrap` target is idempotent — re-running it serves as `upgrade` and `recover`.

## What's still missing for end-to-end Phase 1

This issue ships the orchestration only. The following sub-issues fill in the
data that orchestration needs to be useful:

| Need | Filled by | Without it |
|---|---|---|
| `infra/vm/secrets/secrets.enc.yaml` (sops bundle) | [#1854](https://github.com/denisvmedia/inventario/issues/1854) — see [`SECRETS.md`](./SECRETS.md) | `tailscale up` must be run manually on the VM; TS-op and GH App creds unavailable. |
| Tailscale Operator helm values overlay | [#1855](https://github.com/denisvmedia/inventario/issues/1855) | Default chart values used (works for basic Service/Ingress). |
| `helm/inventario/` PR-preview overlay | [#1856](https://github.com/denisvmedia/inventario/issues/1856) | ApplicationSet template can't reference it yet. |
| `infra/argocd/*.yaml` (AppProject, ApplicationSet, master Application) | [#1858](https://github.com/denisvmedia/inventario/issues/1858) | ArgoCD installs but no Applications appear; PR labels do nothing. |
| Full README with first-time setup walkthrough and DR runbook | [#1863](https://github.com/denisvmedia/inventario/issues/1863) | Use this file + bootstrap.sh warnings as a stopgap. |

`bootstrap.sh` and `vm-install.sh` warn (don't fail) when these are missing, so
this issue can land and the others can fill in independently.

## Pinned versions

- vcluster standalone: `v0.34.0`
- Kubernetes (inside vcluster): `v1.34.0`
- Tailscale: latest from `tailscale.com/install.sh` (idempotent)
- ArgoCD chart: latest in `argo/argo-cd` repo (pin once #1858 lands a chart-version)
- Tailscale Operator chart: latest in `tailscale/tailscale-operator` repo (same, pin in #1855)

Pinning the latter two lands with the issues that own them.

## Tailscale hostname

The bootstrap registers the VM in the tailnet as `inv-vcl01`. Override via
environment if you need a different label (for a second VM, multi-environment,
etc.):

```bash
# On the laptop
TS_HOSTNAME=inv-vcl02 make -C infra bootstrap VM=...
# This is passed through to vm-install.sh via the env-preserving sudo -E.
```
