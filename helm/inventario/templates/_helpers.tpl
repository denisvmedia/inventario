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
Compute whether any split role is enabled. Emits "true" or empty.
*/}}
{{- define "inventario.splitEnabled" -}}
{{- $split := .Values.run.apiserver.enabled -}}
{{- range $role := splitList " " (include "inventario.workerRoles" .) -}}
  {{- $cfg := index $.Values.run.workers $role -}}
  {{- if and $cfg $cfg.enabled -}}{{- $split = true -}}{{- end -}}
{{- end -}}
{{- if $split -}}true{{- end -}}
{{- end -}}

{{/*
Validate the run topology. Fails the render when run.all and any
split role (apiserver or any worker) are both enabled, or when
neither mode is active.
*/}}
{{- define "inventario.validateRunMode" -}}
{{- $all := .Values.run.all.enabled -}}
{{- $split := eq (include "inventario.splitEnabled" .) "true" -}}
{{- if and $all $split -}}
{{- fail "run.all.enabled and split roles (run.apiserver or run.workers.<group>) are mutually exclusive. Set run.all.enabled=false when using split Deployments." -}}
{{- end -}}
{{- if not (or $all $split) -}}
{{- fail "No run topology is active. Enable run.all (combined) or run.apiserver together with at least one run.workers.<group>.enabled=true (split)." -}}
{{- end -}}
{{- end -}}

{{/*
Resource name suffix for the API-server split Deployment.
*/}}
{{- define "inventario.apiserverName" -}}
{{- printf "%s-apiserver" (include "inventario.fullname" .) | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Resource name suffix for a per-worker-group Deployment.
Usage: include "inventario.workerName" (dict "root" . "role" "media")
*/}}
{{- define "inventario.workerName" -}}
{{- $cli := include "inventario.workerCliId" .role -}}
{{- printf "%s-worker-%s" (include "inventario.fullname" .root) $cli | trunc 63 | trimSuffix "-" -}}
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