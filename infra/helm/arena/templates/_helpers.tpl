{{/* Standard labels applied to every object. */}}
{{- define "arena.labels" -}}
app.kubernetes.io/part-of: arena
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{/* The DSN the gateway and seed job use. */}}
{{- define "arena.databaseUrl" -}}
{{- if .Values.postgres.externalDatabaseUrl -}}
{{ .Values.postgres.externalDatabaseUrl }}
{{- else -}}
postgres://{{ .Values.postgres.user }}:{{ .Values.postgres.password }}@{{ .Release.Name }}-postgres:5432/{{ .Values.postgres.database }}?sslmode=disable
{{- end -}}
{{- end -}}
