{{- define "inventario.name" -}}
{{- .Chart.Name | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.fullname" -}}
{{- printf "%s-%s" .Release.Name (include "inventario.name" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.labels" -}}
helm.sh/chart: {{ include "inventario.chart" . }}
app.kubernetes.io/name: {{ include "inventario.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- with .Chart.AppVersion }}
app.kubernetes.io/version: {{ . | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end -}}

{{- define "inventario.selectorLabels" -}}
app.kubernetes.io/name: {{ include "inventario.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "inventario.appSelectorLabels" -}}
{{- include "inventario.selectorLabels" . }}
app.kubernetes.io/component: web
{{- end -}}

{{/*
Canonical, stable-ordered list of worker group keys. Each entry must
match both a run.workers.<key> values block and exactly one worker-group
identifier accepted by `run workers --workers-only=<cli-id>`. Groups
consolidate individual worker families sharing an operational profile;
see go/cmd/inventario/run/workers/selector.go for composition.
*/}}
{{- define "inventario.workerRoles" -}}
archive emails housekeeping media
{{- end -}}

{{/*
Translate a worker-group key (as it appears in values.run.workers.*)
to the CLI identifier accepted by `--workers-only`. Group ids are
already CLI-shaped, so this is an identity mapping kept as a helper
in case future aliases are needed.
Usage: include "inventario.workerCliId" "housekeeping"
*/}}
{{- define "inventario.workerCliId" -}}
{{ . }}
{{- end -}}

{{/*
Compute whether any worker role is enabled. Emits "true" or empty.
*/}}
{{- define "inventario.anyWorkerEnabled" -}}
{{- $any := false -}}
{{- range $role := splitList " " (include "inventario.workerRoles" .) -}}
  {{- $cfg := index $.Values.run.workers $role -}}
  {{- if and $cfg $cfg.enabled -}}{{- $any = true -}}{{- end -}}
{{- end -}}
{{- if $any -}}true{{- end -}}
{{- end -}}

{{/*
Compute whether split mode is active. Split mode is gated on the
API-server: run.apiserver.enabled=true. Workers without an apiserver
are rejected by validateRunMode.
*/}}
{{- define "inventario.splitEnabled" -}}
{{- if .Values.run.apiserver.enabled -}}true{{- end -}}
{{- end -}}

{{/*
Validate the run topology. Fails the render when:
- run.all and any split role (apiserver or worker) are both enabled
  (mutually exclusive), or
- any worker is enabled while run.apiserver.enabled=false
  (workers-only mode is not supported — there is nobody to serve
  the HTTP API and the chart-managed Service / Ingress / NOTES
  would dangle), or
- no topology is active.
*/}}
{{- define "inventario.validateRunMode" -}}
{{- $all := .Values.run.all.enabled -}}
{{- $apiserver := .Values.run.apiserver.enabled -}}
{{- $anyWorker := eq (include "inventario.anyWorkerEnabled" .) "true" -}}
{{- if and $all (or $apiserver $anyWorker) -}}
{{- fail "run.all.enabled and split roles (run.apiserver or run.workers.<group>) are mutually exclusive. Set run.all.enabled=false when using split Deployments." -}}
{{- end -}}
{{- if and $anyWorker (not $apiserver) -}}
{{- fail "run.apiserver.enabled=true is required when any run.workers.<group>.enabled=true. Workers-only mode is not supported by this chart — enable run.apiserver alongside the workers." -}}
{{- end -}}
{{- if not (or $all $apiserver) -}}
{{- fail "No run topology is active. Enable run.all (combined) or run.apiserver (split; workers optional)." -}}
{{- end -}}
{{- end -}}

{{/*
Base name used to derive split-mode resource names. The base is
truncated short enough to leave room for the longest known suffix
(`-worker-housekeeping`, 20 chars) so concatenated names never
collapse onto each other or onto inventario.fullname after the
trailing trunc 63 in printf-then-trunc patterns. Resulting names
remain unique per release because both inventario.fullname and
this base are derived from .Release.Name + chart name.
*/}}
{{- define "inventario.splitBaseName" -}}
{{- printf "%s-%s" .Release.Name (include "inventario.name" .) | trunc 43 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name for the API-server split Deployment. The base is
pre-truncated by inventario.splitBaseName so the `-apiserver`
suffix is never lost to `trunc 63`.
*/}}
{{- define "inventario.apiserverName" -}}
{{- printf "%s-apiserver" (include "inventario.splitBaseName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name for a per-worker-group Deployment. The base is
pre-truncated by inventario.splitBaseName so the `-worker-<role>`
suffix is never lost to `trunc 63`.
Usage: include "inventario.workerName" (dict "root" . "role" "media")
*/}}
{{- define "inventario.workerName" -}}
{{- $cli := include "inventario.workerCliId" .role -}}
{{- printf "%s-worker-%s" (include "inventario.splitBaseName" .root) $cli | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Prometheus scrape annotations for a pod, emitted ONLY when
.Values.metrics.podAnnotations.enabled is true. The app always serves
/metrics; these annotations just let an operator-less Prometheus
(kubernetes_sd_configs role: pod) discover the pod. The port differs by
role — the API pods pass "3333", workers pass their probe port — so it is
taken as an argument rather than hard-coded.
Usage (drop under spec.template.metadata.annotations):
  {{- include "inventario.metricsPodAnnotations" (dict "root" . "port" "3333") | nindent 8 }}
Renders nothing when disabled, so callers must still guard the parent
`annotations:` key so it is never emitted empty.
*/}}
{{- define "inventario.metricsPodAnnotations" -}}
{{- if .root.Values.metrics.podAnnotations.enabled -}}
prometheus.io/scrape: "true"
prometheus.io/port: {{ .port | quote }}
prometheus.io/path: "/metrics"
{{- end -}}
{{- end -}}

{{/*
Component selector labels for the combined (run.all) Deployment.
Alias of appSelectorLabels kept for explicitness.
*/}}
{{- define "inventario.allSelectorLabels" -}}
{{ include "inventario.appSelectorLabels" . }}
{{- end -}}

{{/*
Component selector labels for the split API-server Deployment.
*/}}
{{- define "inventario.apiserverSelectorLabels" -}}
{{- include "inventario.selectorLabels" . }}
app.kubernetes.io/component: apiserver
{{- end -}}

{{/*
Component selector labels for a worker Deployment.
Usage: include "inventario.workerSelectorLabels" (dict "root" . "role" "media")
*/}}
{{- define "inventario.workerSelectorLabels" -}}
{{- $cli := include "inventario.workerCliId" .role -}}
{{ include "inventario.selectorLabels" .root }}
app.kubernetes.io/component: {{ printf "worker-%s" $cli }}
{{- end -}}

{{/*
Deep-merge the role-specific worker values block over run.workers.common.
Returns the merged dict. The caller is expected to deepCopy the result
before mutating.
Usage: $cfg := include "inventario.workerConfig" (dict "root" . "role" "media") | fromYaml
(or use a with-$ pattern when only selected fields are read).
*/}}
{{- define "inventario.workerConfig" -}}
{{- $common := deepCopy .root.Values.run.workers.common -}}
{{- $role := index .root.Values.run.workers .role -}}
{{- $override := deepCopy (default (dict) $role) -}}
{{- $merged := mustMergeOverwrite $common $override -}}
{{- toYaml $merged -}}
{{- end -}}

{{/*
Build the HPA metrics list for an autoscaling block. CPU and memory
target utilization percentages turn into Resource metric entries;
any extra entries from `<block>.metrics` are appended verbatim.
Fails the render when autoscaling.enabled=true but no metric ends
up configured (empty `spec.metrics` is invalid for autoscaling/v2).
The `scope` arg names the offending values path in the error.
Usage:
  include "inventario.hpaMetrics" (dict "cfg" .Values.run.all.autoscaling "scope" "run.all")
Returns the indented YAML list ready to drop under `metrics:` with
`{{- include "inventario.hpaMetrics" ... | nindent 4 }}`.
*/}}
{{- define "inventario.hpaMetrics" -}}
{{- $cfg := .cfg -}}
{{- $metrics := list -}}
{{- if $cfg.targetCPUUtilizationPercentage -}}
{{- $metrics = append $metrics (dict "type" "Resource" "resource" (dict "name" "cpu" "target" (dict "type" "Utilization" "averageUtilization" (int $cfg.targetCPUUtilizationPercentage)))) -}}
{{- end -}}
{{- if $cfg.targetMemoryUtilizationPercentage -}}
{{- $metrics = append $metrics (dict "type" "Resource" "resource" (dict "name" "memory" "target" (dict "type" "Utilization" "averageUtilization" (int $cfg.targetMemoryUtilizationPercentage)))) -}}
{{- end -}}
{{- with $cfg.metrics -}}
{{- $metrics = concat $metrics . -}}
{{- end -}}
{{- if not $metrics -}}
{{- fail (printf "%s.autoscaling.enabled=true but no metrics are configured. Set %s.autoscaling.targetCPUUtilizationPercentage, .targetMemoryUtilizationPercentage, or add entries to .metrics — an empty spec.metrics list is invalid for autoscaling/v2." .scope .scope) -}}
{{- end -}}
{{- toYaml $metrics -}}
{{- end -}}

{{/*
True when the apiserver endpoint is served by an internally-rendered
Deployment (either run.all or run.apiserver). Used by Service and
Ingress to decide whether to emit.
*/}}
{{- define "inventario.apiserverRendered" -}}
{{- if or .Values.run.all.enabled .Values.run.apiserver.enabled -}}true{{- end -}}
{{- end -}}

{{/*
Selector labels used by the apiserver Service/Ingress. Matches
whichever Deployment (all or apiserver) is currently enabled.
*/}}
{{- define "inventario.apiserverServiceSelectorLabels" -}}
{{- if .Values.run.all.enabled -}}
{{ include "inventario.allSelectorLabels" . }}
{{- else -}}
{{ include "inventario.apiserverSelectorLabels" . }}
{{- end -}}
{{- end -}}

{{- define "inventario.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "inventario.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "inventario.secretName" -}}
{{- default (printf "%s-secret" (include "inventario.fullname" .)) .Values.secrets.existingSecret -}}
{{- end -}}

{{- define "inventario.demoPostgresName" -}}
{{- printf "%s-demo-postgres" (include "inventario.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoPostgresSecretName" -}}
{{- printf "%s-credentials" (include "inventario.demoPostgresName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoPostgresInitName" -}}
{{- printf "%s-init" (include "inventario.demoPostgresName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoPostgresPvcName" -}}
{{- printf "%s-data" (include "inventario.demoPostgresName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoRedisName" -}}
{{- printf "%s-demo-redis" (include "inventario.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoMinioName" -}}
{{- printf "%s-demo-minio" (include "inventario.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoMinioSecretName" -}}
{{- printf "%s-credentials" (include "inventario.demoMinioName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoMinioPvcName" -}}
{{- printf "%s-data" (include "inventario.demoMinioName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.demoMinioBucketJobName" -}}
{{- printf "%s-create-bucket" (include "inventario.demoMinioName" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "inventario.dbDsn" -}}
{{- if .Values.demo.postgresql.enabled -}}
{{- printf "postgres://%s:%s@%s:5432/%s?sslmode=disable" .Values.demo.postgresql.username .Values.demo.postgresql.password (include "inventario.demoPostgresName" .) .Values.demo.postgresql.database -}}
{{- else -}}
{{- required "secrets.dbDsn must be set when demo.postgresql.enabled=false and secrets.existingSecret is empty" (.Values.secrets.dbDsn | trim) -}}
{{- end -}}
{{- end -}}

{{- define "inventario.migratorDsn" -}}
{{- if .Values.secrets.migratorDbDsn -}}
{{- .Values.secrets.migratorDbDsn -}}
{{- else if .Values.demo.postgresql.enabled -}}
{{- printf "postgres://%s:%s@%s:5432/%s?sslmode=disable" .Values.demo.postgresql.migratorUser .Values.demo.postgresql.migratorPassword (include "inventario.demoPostgresName" .) .Values.demo.postgresql.database -}}
{{- else -}}
{{- include "inventario.dbDsn" . -}}
{{- end -}}
{{- end -}}

{{- define "inventario.superuserDsn" -}}
{{- if .Values.setupJob.bootstrap.superuserDsn -}}
{{- .Values.setupJob.bootstrap.superuserDsn -}}
{{- else if .Values.demo.postgresql.enabled -}}
{{- printf "postgres://%s:%s@%s:5432/%s?sslmode=disable" .Values.demo.postgresql.username .Values.demo.postgresql.password (include "inventario.demoPostgresName" .) .Values.demo.postgresql.database -}}
{{- else -}}
{{- include "inventario.dbDsn" . -}}
{{- end -}}
{{- end -}}

{{- define "inventario.redisUrl" -}}
{{- if .Values.demo.redis.enabled -}}
{{- printf "redis://%s:6379/0" (include "inventario.demoRedisName" .) -}}
{{- end -}}
{{- end -}}

{{/*
Migration init container, used by the app Deployment(s) in argocdMode so the
schema is brought up to date by the same pod revision that runs the new image
(approach A from #1884). Runs `inventario db migrate up` with MIGRATOR_DB_DSN
(falling back to INVENTARIO_DB_DSN); identical retry-loop semantics to the
setup Job's migrate step. Idempotent — re-runs on every pod start, no-ops
when schema is already at the embedded max version.

The container's name is hard-coded `migrate` so callers don't pick the wrong
name; rolling-update / advisory-lock semantics make concurrent runs safe
across replicas.

Usage:
  {{ include "inventario.migrateInitContainer" . | nindent 8 }}
The caller is expected to render this list item under `initContainers:`.
*/}}
{{- define "inventario.migrateInitContainer" -}}
{{- $secretName := include "inventario.secretName" . -}}
{{- $imageTag := default .Chart.AppVersion .Values.image.tag -}}
- name: migrate
  image: {{ printf "%s:%s" .Values.image.repository $imageTag | quote }}
  imagePullPolicy: {{ .Values.image.pullPolicy }}
  command: ["sh", "-c"]
  args:
    - |
      set -eu
      export INVENTARIO_DB_DSN="${MIGRATOR_DB_DSN:-$APP_DB_DSN}"
      i=1
      while [ "$i" -le 15 ]; do
        if inventario db migrate up; then
          echo "Schema migrations completed successfully"
          exit 0
        fi
        echo "Migration attempt $i failed, retrying in 2 seconds..."
        i=$((i + 1))
        sleep 2
      done
      echo "Schema migrations failed after 15 attempts"
      exit 1
  env:
    - name: APP_DB_DSN
      {{- if .Values.demo.postgresql.enabled }}
      value: {{ include "inventario.dbDsn" . | quote }}
      {{- else }}
      valueFrom:
        secretKeyRef:
          name: {{ $secretName }}
          key: INVENTARIO_DB_DSN
      {{- end }}
    - name: MIGRATOR_DB_DSN
      {{- if .Values.demo.postgresql.enabled }}
      value: {{ include "inventario.migratorDsn" . | quote }}
      {{- else }}
      valueFrom:
        secretKeyRef:
          name: {{ $secretName }}
          key: MIGRATOR_DB_DSN
          optional: true
      {{- end }}
  {{- with .Values.containerSecurityContext }}
  securityContext:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  volumeMounts:
    - name: tmp
      mountPath: /tmp
{{- end -}}

{{- define "inventario.uploadLocation" -}}
{{- if .Values.demo.minio.enabled -}}
{{- printf "s3://%s?prefix=%s&region=us-east-1&endpoint=%s&s3ForcePathStyle=true" .Values.demo.minio.bucket .Values.demo.minio.prefix (include "inventario.minioEndpointUrl" .) -}}
{{- else -}}
{{- .Values.app.uploadLocation -}}
{{- end -}}
{{- end -}}

{{- define "inventario.minioEndpoint" -}}
{{- if .Values.demo.minio.enabled -}}
{{- printf "%s:9000" (include "inventario.demoMinioName" .) -}}
{{- end -}}
{{- end -}}

{{- define "inventario.minioEndpointUrl" -}}
{{- if .Values.demo.minio.enabled -}}
{{- printf "http://%s" (include "inventario.minioEndpoint" .) -}}
{{- end -}}
{{- end -}}