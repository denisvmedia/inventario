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
{{- printf "s3://%s?prefix=%s&region=us-east-1&endpoint=%s&disableSSL=true&s3ForcePathStyle=true" .Values.demo.minio.bucket .Values.demo.minio.prefix (include "inventario.minioEndpoint" .) -}}
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