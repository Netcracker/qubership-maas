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