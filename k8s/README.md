# Inventario Kubernetes baseline

This directory contains a raw-YAML baseline for running Inventario on Kubernetes.
It is intentionally **not** Helm or Kustomize based.

## Layout overview

- `k8s/prod/`: production-oriented app baseline that expects **external PostgreSQL and Redis**.
- `k8s/dev/`: local/full-stack development baseline with **PostgreSQL, Redis, MinIO, and Inventario** in one namespace.

Both baselines preserve the same startup flow used by `docker-compose.yaml`:

1. Bootstrap database roles/extensions.
2. Run schema migrations.
3. Run initial data setup.
4. Optionally seed example data.
5. Start the web application on port `3333`.

## Image and tag assumptions

- The manifests currently reference `ghcr.io/denisvmedia/inventario:latest`.
- `.goreleaser.yaml` publishes the multi-arch image set under:
  - `ghcr.io/denisvmedia/inventario:<tag>`
  - `ghcr.io/denisvmedia/inventario:v<major>.<minor>`
  - `ghcr.io/denisvmedia/inventario:v<major>`
  - `ghcr.io/denisvmedia/inventario:latest`
  - `ghcr.io/denisvmedia/inventario:SNAPSHOT-<commit>` for snapshot builds
- For production rollouts, prefer replacing `:latest` with a specific published release tag before applying.
- The CI smoke workflow at `.github/workflows/kind-smoke-test.yml` does **not** push a registry image. It builds `inventario:kind-ci` locally, loads that image into the ephemeral kind cluster, and rewrites a temporary copy of `k8s/dev/inventario/job-setup.yaml` plus `k8s/dev/inventario/deployment.yaml` to use the loaded tag for that run.
- The container image is built to run as the non-root `inventario` user (`uid: 1001`, `gid: 1001`) and the manifests match that runtime contract.

## Namespace conventions

- Production resources live in `inventario-prod`.
- Development resources live in `inventario-dev`.
- All namespaced manifests already declare their namespace explicitly.
- On a live empty cluster, apply `namespace.yaml` **before** any namespaced resources. Do not rely on a blind recursive apply order for `k8s/dev`.

## Secret placeholders and runtime inputs

### Production

`k8s/prod/secret.yaml` is a template and must be edited before use.

- Generate `jwt-secret` and `file-signing-key` with `openssl rand -hex 32`.
- Set `bootstrap-db-dsn` to a **privileged PostgreSQL DSN** capable of creating extensions and managing roles.
- Set `migrator-db-dsn` to the migration user DSN.
- Set `app-db-dsn` to the application user DSN.
- Set `redis-url` to the external Redis instance used for token blacklist and other Redis-backed runtime features.
- Set `bootstrap-username` and `bootstrap-username-for-migrations` to the operational and migration usernames that `inventario db bootstrap apply` should provision/grant.
- Replace `admin-password`, `smtp-username`, and `smtp-password` placeholders.

`k8s/prod/configmap.yaml` contains the non-secret runtime defaults for the `inventario run` section, including `INVENTARIO_RUN_ADDR`, `INVENTARIO_RUN_PUBLIC_URL`, `INVENTARIO_RUN_UPLOAD_LOCATION`, and seed toggles.

### Development

`k8s/dev/postgres/secret.yaml`, `k8s/dev/minio/secret.yaml`, and `k8s/dev/inventario/secret.yaml` contain local-only defaults for a throwaway dev cluster. They are not production-safe.

## Storage choices

### Production storage

- The production baseline defaults to `INVENTARIO_RUN_UPLOAD_LOCATION=file:///data/uploads?create_dir=1`.
- `k8s/prod/pvc.yaml` provides the `inventario-data` PVC mounted at `/data`.
- That PVC stores both uploaded files and the setup-state markers used by `k8s/prod/job-setup.yaml` and `k8s/prod/deployment.yaml` to coordinate bootstrap/migrate/init-data ordering.
- If you switch uploads to object storage (`s3://`, `gs://`, or `azblob://` are supported by the application), update `k8s/prod/configmap.yaml` and then revisit whether the PVC should still exist for setup-state coordination.

### Development storage

- PostgreSQL uses `k8s/dev/postgres/pvc.yaml`.
- MinIO uses `k8s/dev/minio/pvc.yaml`.
- Redis is intentionally ephemeral in dev (`emptyDir` in `k8s/dev/redis/deployment.yaml`).
- The dev app stores uploads in MinIO through the `INVENTARIO_RUN_UPLOAD_LOCATION` value from `k8s/dev/inventario/configmap.yaml`:
  `s3://inventario?prefix=uploads/&region=us-east-1&endpoint=minio:9000&disableSSL=true&s3ForcePathStyle=true`
- `k8s/dev/minio/job.yaml` creates the `inventario` bucket expected by that upload location.

## Quick start: production baseline

Prerequisites:

- A cluster with dynamic PVC provisioning (or a matching pre-provisioned volume).
- Reachable external PostgreSQL and Redis endpoints.
- Edited values in `k8s/prod/secret.yaml` and `k8s/prod/configmap.yaml`.

Apply the baseline:

```sh
kubectl apply -f k8s/prod/namespace.yaml
kubectl apply -f k8s/prod/secret.yaml
kubectl apply -f k8s/prod/configmap.yaml
kubectl apply -f k8s/prod/pvc.yaml
kubectl apply -f k8s/prod/service.yaml
kubectl apply -f k8s/prod/job-setup.yaml
kubectl apply -f k8s/prod/deployment.yaml
kubectl apply -f k8s/prod/ingress.yaml
```

Wait for setup and app readiness:

```sh
kubectl -n inventario-prod wait --for=condition=complete job/inventario-setup --timeout=10m
kubectl -n inventario-prod rollout status deployment/inventario --timeout=10m
kubectl -n inventario-prod port-forward service/inventario 3333:3333
```

Notes:

- `k8s/prod/ingress.yaml` is optional; omit it if traffic is terminated elsewhere.
- The setup Job and the Deployment cooperate through the shared PVC, so both must be present for first startup to complete.

## Quick start: development baseline

Use the following order for a **live empty cluster**. Apply `k8s/dev/namespace.yaml` first, then apply the namespaced resources explicitly. This order matches the shipped CI smoke workflow so the documented dev flow and `.github/workflows/kind-smoke-test.yml` stay aligned.

```sh
kubectl apply -f k8s/dev/namespace.yaml

kubectl apply -f k8s/dev/postgres/secret.yaml
kubectl apply -f k8s/dev/postgres/configmap.yaml
kubectl apply -f k8s/dev/postgres/pvc.yaml
kubectl apply -f k8s/dev/postgres/service.yaml
kubectl apply -f k8s/dev/postgres/deployment.yaml

kubectl apply -f k8s/dev/redis/service.yaml
kubectl apply -f k8s/dev/redis/deployment.yaml

kubectl apply -f k8s/dev/minio/secret.yaml
kubectl apply -f k8s/dev/minio/pvc.yaml
kubectl apply -f k8s/dev/minio/service.yaml
kubectl apply -f k8s/dev/minio/deployment.yaml
kubectl apply -f k8s/dev/minio/job.yaml

kubectl apply -f k8s/dev/inventario/configmap.yaml
kubectl apply -f k8s/dev/inventario/secret.yaml
kubectl apply -f k8s/dev/inventario/job-setup.yaml
```

Wait for infrastructure and setup completion before applying the web app:

```sh
kubectl -n inventario-dev rollout status deployment/postgres --timeout=10m
kubectl -n inventario-dev rollout status deployment/redis --timeout=10m
kubectl -n inventario-dev rollout status deployment/minio --timeout=10m
kubectl -n inventario-dev wait --for=condition=complete job/minio-create-bucket --timeout=10m
kubectl -n inventario-dev wait --for=condition=complete job/inventario-setup --timeout=10m

kubectl apply -f k8s/dev/inventario/service.yaml
kubectl apply -f k8s/dev/inventario/deployment.yaml

kubectl -n inventario-dev rollout status deployment/inventario --timeout=10m
kubectl -n inventario-dev port-forward service/inventario 3333:3333
```

After port-forwarding, the app is available on `http://127.0.0.1:3333`.
The dev baseline seeds example data and uses the default admin email from `k8s/dev/inventario/configmap.yaml` with the password from `k8s/dev/inventario/secret.yaml`.

## CI kind smoke workflow

- Workflow entry point: `.github/workflows/kind-smoke-test.yml`
- It runs on pushes to `master` and on pull requests when the app build inputs, init/setup scripts, `k8s/dev/**`, or the workflow file itself change.
- The job builds the production container image locally as `inventario:kind-ci`, creates a kind cluster, loads that image into kind, and keeps the checked-in manifests unchanged by copying `k8s/dev/` to a temporary directory before rewriting only the two Inventario image references used by the setup Job and Deployment.
- The manifest apply order is explicit and namespace-first: `k8s/dev/namespace.yaml`, then PostgreSQL, Redis, MinIO, and the Inventario config/setup resources, followed by readiness waits, then `k8s/dev/inventario/service.yaml` and `k8s/dev/inventario/deployment.yaml`.
- The smoke phase port-forwards `service/inventario` and verifies:
  - `GET /healthz` returns an alive response.
  - `GET /readyz` reports overall ready status plus healthy database and Redis checks.
  - `POST /api/v1/auth/login` succeeds for the seeded admin credentials.
  - `GET /api/v1/locations` includes the seeded `Home` location after login.
- On failure, the workflow collects `kubectl get`, events, `kubectl describe`, logs for the key deployments/jobs, and `kind export logs`, then uploads them as the `kind-smoke-diagnostics` artifact.

## Validation notes

- Safe client-side validation is:
  `kubectl apply --dry-run=client -f <manifest>`
- For dev, prefer validating the same explicit apply order shown above instead of assuming directory recursion will create the namespace and RBAC objects in time.
- This directory is intentionally a raw-YAML baseline only; no Helm chart or Kustomize overlay is delivered here. The CI smoke entry point for this baseline lives in `.github/workflows/kind-smoke-test.yml`.
