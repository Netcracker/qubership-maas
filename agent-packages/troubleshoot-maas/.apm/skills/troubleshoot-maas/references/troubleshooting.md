# MaaS troubleshooting guide

<!-- markdownlint-disable MD013 -->
- [MaaS troubleshooting guide](#maas-troubleshooting-guide)
  - [Why MaaS responds with Forbidden (403) error code with correct credentials](#why-maas-responds-with-forbidden-403-error-code-with-correct-credentials)
  - [Why Cloud-Core deployment fails with validation of MaaS](#why-cloud-core-deployment-fails-with-validation-of-maas)
    - [How to check instance is registered](#how-to-check-instance-is-registered)
    - [How to register instance](#how-to-register-instance)
  - [Why Kafka instance could be unhealthy](#why-kafka-instance-could-be-unhealthy)
    - [Amazon case](#amazon-case)
  - [Why maas-agent responds with Bad Gateway (502) status](#why-maas-agent-responds-with-bad-gateway-502-status)
  - [Where to find MaaS libs examples](#where-to-find-maas-libs-examples)
  - [Change instance after crash](#change-instance-after-crash)
  - [Kafka broker operations timeout](#kafka-broker-operations-timeout)
  - [Recover Kafka topics and Rabbit vhosts](#recover-kafka-topics-and-rabbit-vhosts)
    - [Single topic recovery](#single-topic-recovery)
    - [Multiple topics recovery (namespace level)](#multiple-topics-recovery-namespace-level)
    - [Rabbit vhost recovery (namespace level)](#rabbit-vhost-recovery-namespace-level)
  - [Entity exists in MaaS registry but missing in Kafka or RabbitMQ](#entity-exists-in-maas-registry-but-missing-in-kafka-or-rabbitmq)
<!-- markdownlint-enable MD013 -->

## Why MaaS responds with Forbidden (403) error code with correct credentials

Case 1: it is MaaS clean install or rolling update error - script failing
doesn't fail whole job. Before 3.0.0 version any sh script in deployment
(openshift) folder didn't have proper error handling behavior and in case of
failing of sh script, e.g. creating account manager, whole Jenkins job
wouldn't fail.

So if you have some problems with accounts, database or secrets check if
there is any error in Jenkins job. Sometimes container could have not been
created inside pod and that leads to fail of scripts. In that case just run
the job again.

Case 2: inappropriate role usage. Different requests require creds with
different roles (agent or manager), they are mentioned in
[rest_api.md](rest_api.md) file. See security model of MaaS for more info:
[README.md#security-model](../README.md#security-model)

## Why Cloud-Core deployment fails with validation of MaaS

Reason: Kafka or Rabbit instances are not registered.

Due to the fact, that MaaS is usually installed by Cloud Ops and then used
by different application teams, it could happen that MaaS was installed,
but instances were not registered.

There is a validation.sh script for Cloud Core, checking that at least one
instance is registered either for Rabbit or Kafka. In that case, error in
deployer will look like this:

```text
MAAS_ROUTE_HEALTH={"postgres":{"status":"UP"},"status":"UP"}
There are no one registered broker!
```

### How to check instance is registered

The easiest way is to send GET request to `/health` endpoint of MaaS.
In case if only Kafka is registered the response should look like:

```json
{"kafka":{"status":"UP"},"postgres":{"status":"UP"},"status":"UP"}
```

Another way is to use REST api to get instances:

- [rest_api.md#get-rabbit-instances](rest_api.md#get-rabbit-instances)
- [rest_api.md#get-kafka-instances](rest_api.md#get-kafka-instances)

### How to register instance

- [rest_api.md#register-rabbit-instance](rest_api.md#register-rabbit-instance)
- [rest_api.md#register-kafka-instance](rest_api.md#register-kafka-instance)

Parameters of request depend on the broker configuration, see the
description below, but in difficult cases it should be checked with Cloud
Ops team.

## Why Kafka instance could be unhealthy

Reason: Kafka instance was not registered correctly. The error could look
like `Failed to create kafka admin client for [localhost:9092]: kafka:
client has run out of available brokers to talk to (Is your cluster
reachable?)`. Kafka has many different ways to both authenticate users and
use encryption. MaaS supports the most of such configurations, they are
described here:

[rest_api.md#kafka-auth-dto](rest_api.md#kafka-auth-dto)

### Amazon case

Amazon's Kafka (Amazon MSK) is fully supported by MaaS, but it could have
its special namings and certificates. In case if Kafka is used without
authentication, then typical configuration for instance is:

```json
{
  "id": "kafka-maas-test",
  "addresses": {
    "SSL": [
      "***.amazonaws.com:9094"
    ]
  },
  "maasProtocol": "SSL",
  "caCert": "<put your CA cert here>"
}
```

where CA cert is an Amazon root certificate

More info in this ticket:
[PDSDNREQ-5206](https://psup.netcracker.com/browse/PDSDNREQ-5206)

## Why maas-agent responds with Bad Gateway (502) status

Reason: maas-agent (Cloud-Core) was installed without MaaS checkbox in
CMDB, only then checkbox was turned on.

Applications that talk to MaaS must enable the MaaS integration checkbox in
CMDB for the tenant/namespace. When it is on:

1. Deployer looks for `maas-configuration.yaml` in application
   microservices, aggregates them, and sends them to MaaS.
2. External Route and Internal Address properties are set so maas-agent
   can create secrets and reach MaaS.

If Cloud-Core was already installed and the checkbox is turned on later,
maas-agent stays broken until Cloud-Core is rolling-updated with the
checkbox enabled so secrets are created.

Applications must not call MaaS directly — they go through maas-agent with
an M2M token. See [Architecture](../README.md#architecture).

More about maas-agent:
[qubership-core-maas-agent](https://github.com/Netcracker/qubership-core-maas-agent)

## Where to find MaaS libs examples

Java clients live in the
[qubership-core-java-libs](https://github.com/Netcracker/qubership-core-java-libs)
monorepo:

- [maas-client](https://github.com/Netcracker/qubership-core-java-libs/tree/main/maas-client)
- [maas-client-spring](https://github.com/Netcracker/qubership-core-java-libs/tree/main/maas-client-spring)
- [maas-client-quarkus](https://github.com/Netcracker/qubership-core-java-libs/tree/main/maas-client-quarkus)

Go clients:

- [qubership-core-lib-go-maas-client](https://github.com/Netcracker/qubership-core-lib-go-maas-client)
- [qubership-core-lib-go-maas-core](https://github.com/Netcracker/qubership-core-lib-go-maas-core)
- [qubership-core-lib-go-maas-segmentio](https://github.com/Netcracker/qubership-core-lib-go-maas-segmentio)
- [qubership-core-lib-go-maas-bg-segmentio](https://github.com/Netcracker/qubership-core-lib-go-maas-bg-segmentio)

## Change instance after crash

Assume, your instance become broken and you want to change it to new
instance. You want to make a reconciliation of all maas entities
(topics/vhosts/...) - move them to new instance.
Then you need to look at instance update guide - it allows you to either
update instance registration and trigger recovery endpoint or to redeploy
your application with new maas configuration:
[Instance update](./maintenance.md#instance-update)

## Kafka broker operations timeout

If your Kafka responds too long for some reason for Kafka broker operations
like list/update/create/delete, you may want to increase timeout for Kafka
broker client. You can do it by changing client timeout property via env
variable `KAFKA_CLIENT_TIMEOUT`. Default value is 10s.

Source: [PSUPCLFRM-5218](https://psup.netcracker.com/browse/PSUPCLFRM-5218)

## Recover Kafka topics and Rabbit vhosts

If you created topics or vhosts via MaaS then information about these
entities is stored in MaaS registry. In case if for some reason topics were
deleted in Kafka or vhosts were deleted in RabbitMQ, then you can use
special APIs to recover them:

### Single topic recovery

[Kafka Single Topic Recovery](./rest_api.md#Kafka-Single-Topic-Recovery)

### Multiple topics recovery (namespace level)

[Kafka Topics Reconciliation](./rest_api.md#Kafka-Topics-Reconciliation)

### Rabbit vhost recovery (namespace level)

[Rabbit namespace recovery](./rest_api.md#Rabbit-namespace-recovery)

## Entity exists in MaaS registry but missing in Kafka or RabbitMQ

Scenario: topic or vhost is present in MaaS registry, but this entity is
absent in the real broker (Kafka/RabbitMQ).

You have 2 options:

1. Recover entities from MaaS registry to broker:
   - for one topic use [Single topic recovery](#single-topic-recovery);
   - for all namespace topics use
     [Multiple topics recovery (namespace level)](#multiple-topics-recovery-namespace-level);
   - for Rabbit namespace vhosts use
     [Rabbit vhost recovery (namespace level)](#rabbit-vhost-recovery-namespace-level).

2. If these entities are no longer needed or were created by mistake:
   - delete namespace declarations:
     - Kafka (delete all topics in namespace):
       [Delete Kafka topic](./rest_api.md#delete-kafka-topic)
     - RabbitMQ (delete all vhosts in namespace):
       [Delete virtual host](./rest_api.md#delete-virtual-host)
   - then remove instance registration:
     - Kafka:
       [Remove Kafka instance registration](./rest_api.md#remove-kafka-instance-registration)
     - RabbitMQ:
       [Remove Rabbit instance registration](./rest_api.md#remove-rabbit-instance-registration)
