{{- define "to_millicores" -}}
  {{- $value := toString . -}}
  {{- if hasSuffix "m" $value -}}
    {{ trimSuffix "m" $value }}
  {{- else -}}
    {{ mulf $value 1000 }}
  {{- end -}}
{{- end -}}

{{- define "to_MiB" -}}
  {{- $value := toString . -}}
  {{- if hasSuffix "Gi" $value -}}
   {{ trimSuffix "Gi"  $value |  mulf 1024 | floor }}
  {{- else if hasSuffix "Mi" $value -}}
    {{ trimSuffix "Mi" $value | floor }}
  {{- else if hasSuffix "Ki" $value -}}
    {{ trimSuffix "Ki" $value | divf 1024 | floor }}
  {{- else if hasSuffix "G" $value -}}
    {{ trimSuffix "G" $value | mulf 953.67431640625 | floor }}
  {{- else if hasSuffix "M" $value -}}
    {{ trimSuffix "M" $value | mulf 0.953674316 | floor }}
  {{- else if hasSuffix "k" $value -}}
    {{ trimSuffix "k" $value | mulf 0.00095367431640625 | floor }}
  {{- else }}
    {{ fail (printf "%s MUST have one of suffix: Gi, Mi, Ki, G, M, k." $value) }}
  {{- end -}}
{{- end -}}

{{- define "maas.globalPodSecurityContext" -}}
runAsNonRoot: true
seccompProfile:
  type: RuntimeDefault
{{- if eq .Values.PAAS_PLATFORM "KUBERNETES" }}
fsGroup: 10001
runAsUser: 10001
runAsGroup: 10001
{{- end }}
{{- end -}}

{{- define "maas.globalContainerSecurityContext" -}}
allowPrivilegeEscalation: false
readOnlyRootFilesystem: {{ and (default true .Values.READONLY_CONTAINER_FILE_SYSTEM_ENABLED) (eq .Values.PAAS_PLATFORM "KUBERNETES") }}
capabilities:
  drop:
    - ALL
{{- end -}}

{{- define "maas.tmpVolume" -}}
- name: tmp
  emptyDir:
    sizeLimit: 100Mi
{{- end -}}

{{- define "maas.tmpVolumeMount" -}}
- name: tmp
  mountPath: /tmp
{{- end -}}

{{- define "maas.certVolumes" -}}
{{- if eq .Values.PAAS_PLATFORM "KUBERNETES" }}
- name: sslcerts
  emptyDir:
    sizeLimit: 16Mi
- name: cacerts
  emptyDir:
    sizeLimit: 8Mi
{{- end }}
{{- end -}}

{{- define "maas.certVolumeMounts" -}}
{{- if eq .Values.PAAS_PLATFORM "KUBERNETES" }}
- name: sslcerts
  mountPath: /etc/ssl/certs
- name: cacerts
  mountPath: /usr/local/share/ca-certificates
{{- end }}
{{- end -}}