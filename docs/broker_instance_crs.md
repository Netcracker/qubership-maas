# KafkaInstance and RabbitInstance CRs

Register Kafka and RabbitMQ brokers with MaaS using `KafkaInstance` and `RabbitInstance`
(`maas.netcracker.com/v1`). maas-service reconciles them in-process (Watch + ProcessCR). This is separate from
[custom_resources(CR).md](custom_resources(CR).md) (`kind: MaaS` topic and vhost declarations processed by
core-operator).

Architecture and reconcile behavior: [operator_design.md](operator_design.md).

## Prerequisites

- Kubernetes **1.32+** (CEL on CRDs, selectable fields on `spec.operatorNamespace`).
- Install CRDs from `crds/` in the platform **zero wave** before the MaaS Application Syncs.
- Per broker namespace: Role + RoleBinding so the MaaS ServiceAccount can `get`/`list`/`watch` Secrets referenced by
  the CR.

## Install order

1. Zero wave: apply CRDs from `crds/`.
2. MaaS Application: Sync `maas-service` with operator values (below).
3. Broker namespace Application: apply `KafkaInstance` / `RabbitInstance` (and Secrets).

Brokers may already be running. MaaS does not proxy traffic; it stores connection metadata in PostgreSQL.

## MaaS Application values

| Value | Default | Purpose |
| --- | --- | --- |
| `OPERATOR_ENABLED` | `false` | Start Watch + ProcessCR (Lease in `CLOUD_NAMESPACE`). ClusterRole when `true` (unless restricted). |
| `DEFAULT_KAFKA_INSTANCE` | `""` | Broker namespace of the default `KafkaInstance` CR. Empty: first Register is default. |
| `DEFAULT_RABBIT_INSTANCE` | `""` | Same for `RabbitInstance`. |
| `K8S_EVENTS_ENABLED` | `true` | Kubernetes Events on instance CRs. `false` omits `events` from ClusterRole. |
| `restrictedEnvironment` | `false` | Skip in-chart ClusterRole; apply out of band. CRDs still from `crds/`. |

Example (enable operator and name a default broker namespace):

```yaml
OPERATOR_ENABLED: true
DEFAULT_KAFKA_INSTANCE: kafka-infra
DEFAULT_RABBIT_INSTANCE: rabbit-infra
K8S_EVENTS_ENABLED: true
restrictedEnvironment: false
```

With `OPERATOR_ENABLED: false`, manager REST still works; instance CRs are not reconciled and the chart does not
render operator ClusterRole.

## Sample CRs

Minimal shapes; field tables and CEL rules are in [operator_design.md](operator_design.md#crd-sketch) and in the
generated CRD YAML under `crds/`.

```yaml
apiVersion: maas.netcracker.com/v1
kind: KafkaInstance
metadata:
  name: kafka-infra
  namespace: kafka-infra
spec:
  operatorNamespace: maas
  address:
    protocol: SASL_SSL
    saslMechanism: SCRAM-SHA-512
    bootstrapServers:
      - kafka.kafka-infra.svc.cluster.local:9093
    caCertSecretRef:
      name: kafka-ca
  credentials:
    admin:
      credentialsSecretRef:
        name: kafka-admin
    client:
      credentialsSecretRef:
        name: kafka-client
```

```yaml
apiVersion: maas.netcracker.com/v1
kind: RabbitInstance
metadata:
  name: rabbit-infra
  namespace: rabbit-infra
spec:
  operatorNamespace: maas
  apiURL: http://rabbit.rabbit-infra.svc.cluster.local:15672/api
  amqpURL: amqp://rabbit.rabbit-infra.svc.cluster.local:5672
  credentialsSecretRef:
    name: rabbit-admin
```

## Status and troubleshooting

- Use `kubectl get kafkainstances` / `kubectl get rabbitinstances` (`mkafi` / `mrabi`). Columns include `READY`,
  `DEFAULT`, `ALIAS`, and `PHASE`. `ALIAS` is `status.alias`, the PostgreSQL instance id.
- While the operator is enabled, manager REST cannot Update or Unregister a row whose `cr_namespace` is set (`409`).
  Change the CR, or delete it. Delete does not remove the PostgreSQL registration: ProcessCR clears `cr_namespace` and
  `cr_name`, then REST can change that id again. See [rest_api.md](rest_api.md) and
  [troubleshooting.md](troubleshooting.md).
