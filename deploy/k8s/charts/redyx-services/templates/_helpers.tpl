{{/*
Expand the name of the chart.
*/}}
{{- define "redyx-services.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "redyx-services.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "redyx-services.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "redyx-services.labels" -}}
helm.sh/chart: {{ include "redyx-services.chart" . }}
{{ include "redyx-services.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "redyx-services.selectorLabels" -}}
app.kubernetes.io/name: {{ include "redyx-services.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Service-specific labels
*/}}
{{- define "redyx-services.serviceLabels" -}}
helm.sh/chart: {{ include "redyx-services.chart" . }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/part-of: redyx
{{- end }}

{{/*
Service-specific selector labels
*/}}
{{- define "redyx-services.serviceSelectorLabels" -}}
app.kubernetes.io/name: {{ .serviceName }}-service
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the image name for a service
*/}}
{{- define "redyx-services.image" -}}
{{- if .Values.global.image.registry }}
{{- printf "%s/redyx/%s-service:%s" .Values.global.image.registry .serviceName .Values.global.image.tag }}
{{- else }}
{{- printf "redyx/%s-service:%s" .serviceName .Values.global.image.tag }}
{{- end }}
{{- end }}

{{/*
Generate environment variable from template string
Handles {{ .Values.xxx }} references in env values
*/}}
{{- define "redyx-services.resolveEnvValue" -}}
{{- $value := . -}}
{{- if contains "{{" $value -}}
{{- $value -}}
{{- else -}}
{{- $value -}}
{{- end -}}
{{- end }}
