# Disaster recovery runbook

A backup you have never restored is a hypothesis. This runbook is the
procedure that turns it into a fact, plus what to do on the day something is
actually gone.

For *setting up* the backup layers, see [PRODUCTION.md](../PRODUCTION.md) §B11.
This document assumes they exist and covers testing them and using them.

## Objectives

State these in your own terms before an incident, because during one they are
the only thing that settles "restore or forward-fix".

| | Target | What it means |
| --- | --- | --- |
| **RPO** — how much data you can lose | **24 hours** | Nightly logical backups. A failure just before the next one loses a day of edits. Shorten it with more frequent backups or continuous archiving (WAL shipping / PITR). |
| **RTO** — how long you can be down | **4 hours** | Restore the database, confirm the object store, redeploy, verify. Most of it is the database restore; the rest is minutes. |

These are defaults, not promises the software makes. If a day of lost
inventory edits is unacceptable for you, the fix is PITR on the database tier,
not anything in Inventario.

## What has to survive

Four things, with different failure modes. A restore that recovers three of
them is a restore that does not work.

1. **The PostgreSQL database.** Everything except file bytes: items,
   locations, users, groups, audit logs, file *metadata*.
2. **The object store.** The file bytes themselves — photos, invoices,
   manuals. Restoring the database without them leaves rows pointing at
   nothing, and the app will show the item and fail the download.
3. **`INVENTARIO_RUN_JWT_SECRET` and `INVENTARIO_RUN_FILE_SIGNING_KEY`.** Lose
   the JWT secret and every session dies (an inconvenience). Lose the file
   signing key and every already-issued download URL breaks, which is also
   only an inconvenience — but if you *regenerate* rather than restore them,
   say so, because it looks like an outage.
4. **`INVENTARIO_RUN_BACKUP_SIGNING_KEY`, if you use `.inb` archives.** The
   Ed25519 key that signs them. Lose it and every existing archive fails
   verification. It cannot be regenerated into validity —
   `inventario backup resign` exists precisely because a key change
   invalidates archives, and it needs the archives *and* the new key, not the
   old one.

Store 3 and 4 in a secrets manager, not only in a Kubernetes Secret in the
cluster you are planning to lose.

## The drill

Run this **before go-live**, and again after any change to the database tier,
the object store, or the backup schedule. It takes under an hour and it is the
only part of this document that proves anything.

Restore into a **scratch namespace or a scratch database**. Never rehearse
into production; a rehearsal that can hurt you is one you will not run.

### 1. Take stock of what you are restoring from

```bash
# Managed Postgres: list the snapshots the provider kept.
# CloudNativePG:
kubectl -n <ns> get backups.postgresql.cnpg.io
# pg_dump CronJob: list what is in the bucket, or on the chart's PVC:
kubectl -n <ns> run backup-ls --rm -it --restart=Never \
  --image=busybox --overrides='{"spec":{"containers":[{"name":"backup-ls",
  "image":"busybox","command":["ls","-l","/backups"],
  "volumeMounts":[{"name":"b","mountPath":"/backups"}]}],
  "volumes":[{"name":"b","persistentVolumeClaim":{"claimName":"<release>-backups"}}]}}'
```

Note the timestamp of the most recent one. If it is older than your RPO, stop
here — you have found the problem, and it is not a restore problem.

### 2. Restore the database into a scratch target

```bash
createdb inventario_drill
pg_restore --dbname=inventario_drill --no-owner --clean --if-exists backup.dump
# or, for a plain SQL dump:
psql --dbname=inventario_drill --file=backup.sql
```

`--no-owner` matters: the dump references the roles of the source deployment,
which may not exist in the scratch one.

### 3. Point a scratch deployment at it

```bash
helm upgrade --install inventario-drill ./helm/inventario \
  -n inventario-drill --create-namespace \
  -f values-prod.yaml \
  --set-string secrets.dbDsn="postgres://…/inventario_drill?sslmode=require"
```

Two things run against the restored data, and both are safe:

- **`migrate up`** brings a backup older than the current release forward.
  Finding out that the migration chain replays cleanly over your actual
  backup is *part of what the drill is testing* — do not skip it.
- **`migrate data`** creates the default tenant and the admin only when they
  are absent. On a restored database it finds them and leaves the password
  alone; it only re-syncs the admin's `tenant_id`. So pass the same
  `setupJob.initData.*` values production uses — a *different* tenant id there
  would move the admin into a tenant that holds none of the restored data.

Leave `setupJob.initData.seedDatabase` at its default of `false`. Seeding demo
data on top of a restore is how a rehearsal turns into a mess.

### 4. Verify what actually matters

Not "the pods are Running". Four checks, in this order, because each one can
pass while the next fails:

```bash
# a. The schema is at the version this image expects.
kubectl -n inventario-drill exec deploy/inventario-drill -- inventario db status

# b. The data is there — counts should match production, not zero.
psql inventario_drill -c "SELECT
  (SELECT count(*) FROM users)       AS users,
  (SELECT count(*) FROM commodities) AS commodities,
  (SELECT count(*) FROM files)       AS files;"
```

- **c. Sign in.** As a real user, with a real password. A restored database
  with an unreachable auth path is not a restored service.
- **d. Download a file.** This is the check people skip and the one that
  catches the real failure: the database restored, the object store did not,
  and every file row points at bytes that are not there. Open an item that has
  a photo and download it.

### 5. Write down what surprised you

Something will. The dump took longer than expected, a role was missing, the
object store was a separate step nobody had documented. Fix the runbook, not
your memory.

### 6. Tear the scratch namespace down

```bash
helm uninstall inventario-drill -n inventario-drill
kubectl delete namespace inventario-drill
dropdb inventario_drill
```

## On the day

### Decide what you are recovering from

| Situation | Action |
| --- | --- |
| Bad release, no schema change | Roll the image back. Do not restore. |
| Bad release with a migration | Roll the image back first — if you followed expand-contract the old image runs against the new schema. Restore only if that fails. |
| Accidental deletion by a user | Restore a scratch copy, extract what was lost, put it back. Do **not** restore over production — you would undo everyone else's work since the backup. |
| Database lost or corrupted | Full restore, below. |
| Cluster lost | Velero restore of the namespace, then the database restore, then §4's verification. |

### Full restore

1. **Stop the writers.** `kubectl -n <ns> scale deploy --replicas=0 --all`.
   Restoring underneath a running app produces a database that disagrees with
   the caches in front of it.
2. **Restore the database** as in §2, into the real target this time.
3. **Confirm the object store.** If the files are gone too, restore them
   before starting the app: a running app with missing bytes generates
   support requests and thumbnail-worker failures for something you are
   already fixing.
4. **Bring the app back.** `helm upgrade --install …` with the normal values.
   The migrate step brings an older backup's schema forward.
5. **Run §4's four checks** against production. Especially (d).
6. **Tell people what they lost.** Everything between the backup and the
   incident is gone, and the people who wrote it are the only ones who can
   re-enter it. A restore announced as "we're back" without naming the window
   leaves users to discover it one missing item at a time.

## Monitoring the backups themselves

A backup job that has been failing for three weeks is the normal way this goes
wrong — it fails quietly, and nothing asks about it until the day it matters.

Whatever runs your backups, alert on **the age of the most recent successful
backup**, not on job failures. A failure alert misses the job that stopped
being scheduled at all.

- **CloudNativePG** exposes `cnpg_collector_last_available_backup_timestamp`.
- **A `pg_dump` CronJob** — alert on the age of the newest successful Job, and
  on the absence of a recent object in the destination bucket. The chart's own
  CronJob (`backup.enabled`) ships both rules with its PrometheusRule:
  `InventarioBackupStale` for the age, `InventarioBackupNeverRan` for the case
  where there is nothing to measure yet. Both need kube-state-metrics.
- **Managed providers** publish snapshot status in their own console; most
  can forward it to an alerting channel.

Inventario's chart ships alerts for the application
(`metrics.prometheusRule.enabled`), and for its own backup CronJob when you use
it. It cannot know what runs your backups otherwise. Wire an alert anyway; it is
the one most likely to earn its keep.
