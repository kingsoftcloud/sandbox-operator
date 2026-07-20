{{- define "sandbox-operator.namespace" -}}
{{- default .Release.Namespace .Values.namespace -}}
{{- end -}}
