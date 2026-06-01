# Inventario preview-infra troubleshooting cheat-sheet

Fast, copy-pasteable commands for debugging the single-VM ArgoCD preview
cluster (`inv-vcl01`: the `inv-vcl01-master`, `inv-vcl01-longevity`, and
`inv-vcl01-pr<N>` environments).

This is the **quick reference**. For scenario walkthroughs (preview not
appearing, TS-operator issues, ArgoCD admin password reset, ingress, …) see
[README §5 Troubleshooting](./README.md#5-troubleshooting); for the secret
model see [SECRETS.md](./SECRETS.md).

The golden rules from real incidents are called out as **💡 Lesson** —
read those first; they're what actually saves time.

## 0. Access

The cluster lives on a Tailscale-reachable VM. There is **no `argocd` CLI**
installed locally — drive ArgoCD through `kubectl` against the Application
CRD.

```bash
export KUBECONFIG=~/.kube/inv-vcl01.config   # set once per shell

# Is the cluster reachable at all?
tailscale status | grep inv-vcl01            # expect argocd-inv-vcl01 + the env nodes
kubectl get ns | grep inv-vcl01              # sanity: API answers
```

> 💡 **Lesson.** If `kubectl` hangs with `TLS handshake timeout`, you're
> almost certainly pointed at the wrong context (e.g. a local `kind`
> cluster). `kubectl config current-context` — the VM cluster is **only**
> reachable via `~/.kube/inv-vcl01.config`, not your default kubeconfig.

## 1. ArgoCD at a glance

```bash
# All apps with both status axes (the view that's usually enough):
kubectl -n argocd get applications.argoproj.io -A \
  -o custom-columns='NAME:.metadata.name,SYNC:.status.sync.status,HEALTH:.status.health.status,REV:.status.sync.revision'

# One app — sync result + any blocking conditions:
kubectl -n argocd get application.argoproj.io <app> \
  -o jsonpath='SYNC={.status.sync.status} HEALTH={.status.health.status}{"\n"}OP={.status.operationState.phase} msg={.status.operationState.message}{"\n"}'

# App-level conditions (rendering/auth errors, comparison errors):
kubectl -n argocd get application.argoproj.io <app> \
  -o jsonpath='{range .status.conditions[*]}{.type}: {.message}{"\n"}{end}'

# ApplicationSet generator problems (preview never appears):
kubectl -n argocd describe appset inventario-pr-previews   # look at .status.conditions
kubectl -n argocd logs deploy/argocd-applicationset-controller --tail=100
```

### Sync vs Health — internalize this

Two **orthogonal** axes:

- **Sync**: `Synced` / `OutOfSync` — does the live cluster match git?
- **Health**: `Healthy` / `Progressing` / `Degraded` / `Missing` /
  `Suspended` — are the live resources actually well?

`Synced + Degraded` is the classic confusing state: **the manifests applied
correctly, but a live child resource is unhealthy.** Don't go hunting in git
or the chart — go look at the resources.

How ArgoCD decides `Degraded`:

| Resource   | `Degraded` when…                                                        |
| ---------- | ----------------------------------------------------------------------- |
| Deployment | `progressDeadlineSeconds` exceeded — pods never became Ready            |
| Job        | the Job has a `Failed` condition (backoffLimit or activeDeadline hit)   |
| Pod        | the pod is in a failed phase                                            |

## 2. Drill into a Degraded app

```bash
ns=inv-vcl01-master   # the namespace == the app name for these envs

kubectl -n $ns get pods -o wide
kubectl -n $ns get jobs
```

> 💡 **Lesson.** If the app pod is `Running 1/1` but the Application is
> `Degraded`, the culprit is usually a **Job** (`setup` or `init-data`)
> showing `Failed`/`0/1`. A failed Job drags the whole Application to
> `Degraded` even though the app itself is perfectly healthy.

## 3. Crash-looping pods (the high-value section)

```bash
ns=inv-vcl01-master
pod=$(kubectl -n $ns get pods -l app.kubernetes.io/component=init-data \
  --sort-by=.metadata.creationTimestamp -o name | tail -1)

# Current attempt:
kubectl -n $ns logs $pod -c init-data
# The crashed PREVIOUS attempt (restartPolicy: OnFailure restarts in place):
kubectl -n $ns logs $pod -c init-data --previous

# Exit code + reason of the last crash (works even while it loops):
kubectl -n $ns get pod ${pod#pod/} \
  -o jsonpath='{.status.initContainerStatuses[0].lastState.terminated}'
```

> 💡 **Lesson (this one cracked the case twice).** When you see
> `Back-off restarting failed container`, **immediately grab a live pod's
> logs with `--previous`** before theorizing. With `restartPolicy: OnFailure`
> the container restarts *in place*, so `logs` (no `-p`) is often empty while
> `logs --previous` shows the real error. If the Job already finished
> (`failed=1`) its pod is GC'd and the logs are gone — so catch a looping pod
> while it's alive.

Events persist ~1h even after pods are gone — a good fallback:

```bash
kubectl -n $ns get events --sort-by=.lastTimestamp | grep -iE 'init-data|backoff|error|unhealthy'
```

## 4. Jobs: `setup` and `init-data`

```bash
ns=inv-vcl01-master
job=$ns-inventario-init-data

# Why did it fail — BackoffLimitExceeded vs DeadlineExceeded:
kubectl -n $ns get job $job -o jsonpath='{.status.conditions}' | python3 -m json.tool
kubectl -n $ns get job $job -o jsonpath='failed={.status.failed} succeeded={.status.succeeded}{"\n"}'
```

These Jobs carry `argocd.argoproj.io/sync-options: "Force=true,Replace=true"`,
so ArgoCD **delete-then-creates them on every sync** — they re-run each merge.
That means anything they do must be **idempotent** (re-runnable against an
already-provisioned env), or the env goes `Degraded` on the next sync.

What the two Jobs do:

- `…-setup` (wave −5): bootstrap DB + run migrations + seed the app admin.
- `…-init-data` (wave +5): `inventario db migrate data` → optional
  `backoffice bootstrap` (#1967) → optional `/api/v1/seed`.

## 5. Read the **deployed** manifest, not your local tree

> 💡 **Lesson.** The deployed chart + image can be **ahead of your local
> checkout** (master/longevity track the `argocd/master-pin` branch, which
> CI force-pushes on every master image build). Debugging against your local
> files will mislead you. Always read the **live** object.

```bash
ns=inv-vcl01-master

# Which image (= which master commit) is actually running?
kubectl -n $ns get deploy $ns-inventario \
  -o jsonpath='{.spec.template.spec.containers[0].image}{"\n"}'   # ghcr.io/...:sha-<7>

# The rendered init-data script + env, straight from the cluster:
kubectl -n $ns get job $ns-inventario-init-data \
  -o jsonpath='{.spec.template.spec.initContainers[0].args[0]}'
kubectl -n $ns get job $ns-inventario-init-data -o json \
  | python3 -c 'import json,sys; [print(e["name"],"=",e.get("value","<from-secret>")) for e in json.load(sys.stdin)["spec"]["template"]["spec"]["initContainers"][0]["env"]]'
```

Map a `sha-<7>` back to a commit (GitHub MCP / `gh`), and read the pin:

```bash
gh api repos/denisvmedia/inventario/contents/infra/argocd/master-image.json?ref=argocd/master-pin \
  --jq '.content' | base64 -d        # -> { "sha": "...", "imageTag": "sha-<7>" }
```

## 6. Images & the `master-pin` flow

`inv-vcl01-master` **and** `inv-vcl01-longevity` both read the **same**
`argocd/master-pin` file, so they roll forward together on every master push:

```text
merge to master → .github/workflows/docker.yml builds + publishes sha-<7>
               → force-pushes argocd/master-pin (sha + imageTag)
               → both ApplicationSets re-generate at the new sha (poll 60s)
               → ArgoCD auto-syncs (Replace recreates the immutable Jobs)
```

`helm/**`, `infra/helm-overlays/**`, and `infra/argocd/**` are in the image
build's path filter, so even a **chart-only** change triggers a rebuild +
pin flip.

Stuck on `ImagePullBackOff`? The `sha-<7>` image isn't in GHCR yet:

```bash
crane manifest ghcr.io/denisvmedia/inventario:sha-<7>   # or docker manifest inspect …
# "manifest unknown" → the build hasn't published yet; wait/check Actions.
```

## 7. Secrets — inspect safely

```bash
ns=inv-vcl01-master

# KEY NAMES only — never dump values:
kubectl -n $ns get secret inventario-admin -o json \
  | python3 -c 'import json,sys; print("\n".join(sorted(json.load(sys.stdin)["data"].keys())))'
```

> 💡 **Lesson.** In a **sops** bundle the **keys are plaintext** and only the
> **values** are encrypted, so you can `grep` the encrypted file to check
> structure without decrypting anything:
>
> ```bash
> grep -nE '^[a-z_]+:|password|api_key|email' infra/vm/secrets/secrets.enc.yaml
> ```

`apply-secrets.sh` runs **out of band** — ArgoCD does **not** manage these
Secrets. After you add a key to the bundle (or a chart feature starts needing
one), re-run it or the env stays broken:

```bash
git pull origin master                       # get the latest apply-secrets.sh
make -C infra bootstrap VM=<user@vm>          # decrypts sops + applies Secrets over SSH
```

> 💡 **Lesson (recurring trap).** A chart feature that **fails loud** on a
> missing secret key (e.g. AI-vision `provider=anthropic` needs the API key
> #1976; back-office bootstrap needs `BACKOFFICE_USER_PASSWORD` #1967) will
> deploy via ArgoCD **before** `apply-secrets.sh` has materialized the key →
> the workload CrashLoops / the Job fails → app `Degraded`. The fix is
> almost never in the chart: re-run `apply-secrets`. When adding such a
> feature, run it **before** the commit syncs.

## 8. Postgres / DB poking (read-only first)

```bash
ns=inv-vcl01-master
pg="kubectl -n $ns exec deploy/$ns-inventario-demo-postgres -- psql -U inventario -d inventario"

$pg -c "select email, role, created_at from backoffice_users order by created_at;"
```

> 💡 **Lesson.** Before deleting a row, discover what references it — a blind
> `DELETE` trips foreign keys (`update or delete … violates foreign key
> constraint`):
>
> ```bash
> $pg -At -c "SELECT conrelid::regclass FROM pg_constraint
>             WHERE confrelid='backoffice_users'::regclass AND contype='f';"
> # e.g. backoffice_refresh_tokens, backoffice_user_mfa_secrets
> # → delete children first, then the parent row:
> $pg -c "DELETE FROM backoffice_refresh_tokens;
>         DELETE FROM backoffice_user_mfa_secrets;
>         DELETE FROM backoffice_users;"
> ```

Note the env split: `master` uses `emptyDir` demo stores (disposable);
`longevity` puts them on PVCs + Velero backup (**durable** — never wipe its
data wholesale). Treat destructive DB ops on `longevity` surgically.

## 9. Symptom → likely cause

| Symptom                                                            | Likely cause / next step                                                                 |
| ------------------------------------------------------------------ | ---------------------------------------------------------------------------------------- |
| `Synced` + `Degraded`, app pod `Running`                           | a Job is `Failed` → `kubectl get jobs`, then §3 logs `--previous`                         |
| init-data Job fails: `a backoffice user already exists`            | bootstrap not idempotent for that email → ensure `--ensure` is deployed (#1967)          |
| init-data Job fails: `BACKOFFICE_USER_PASSWORD is empty`           | secret key missing → re-run `apply-secrets` (§7)                                         |
| app pod CrashLoops: `database schema lags …`                       | `setup` Job hasn't migrated yet → check `kubectl get jobs` / its logs                    |
| app pod CrashLoops on boot, `aivision`/`wireCommodityScan`         | `provider=anthropic` but API key missing → `apply-secrets` (#1976)                       |
| `ImagePullBackOff` on `sha-<7>`                                    | image not published yet → check Actions / `crane manifest` (§6)                          |
| Preview never appears after labeling/merge                         | ApplicationSet generator → `describe appset` + controller logs (§1, README §5)           |
| ArgoCD UI returns 401 after a secret rotation                      | re-patch `argocd-secret` admin hash → README §5                                          |

## 10. See also

- [README §5 Troubleshooting](./README.md#5-troubleshooting) — full scenario
  walkthroughs.
- [SECRETS.md](./SECRETS.md) — the sops bundle schema + `apply-secrets` flow.
