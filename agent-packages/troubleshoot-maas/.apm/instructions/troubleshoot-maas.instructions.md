---
description: >-
  Diagnostic playbook for MaaS deployment, broker registration, and runtime
  failures. Never invoke automatically — only when explicitly requested by
  the user.
applyTo: "**/*"
---

When diagnosing a MaaS problem — 403/502, missing broker registration,
unhealthy Kafka/Rabbit, topic/vhost drift, instance recovery, Cloud-Core
validation, or DR standby — apply the `troubleshoot-maas` skill.
