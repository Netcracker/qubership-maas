# MaaS troubleshooting guide

- [MaaS troubleshooting guide](#maas-troubleshooting-guide)
  - [Why MaaS responds with Forbidden (403) error code with correct credentials](#why-maas-responds-with-forbidden-403-error-code-with-correct-credentials)
  - [Why Cloud-Core deployment fails with validation of MaaS](#why-cloud-core-deployment-fails-with-validation-of-maas)
    - [How to check instance is registered:](#how-to-check-instance-is-registered)
    - [How to register instance](#how-to-register-instance)
  - [Why Kafka instance could be unhealthy](#why-kafka-instance-could-be-unhealthy)
    - [Amazon case](#amazon-case)
  - [My instance has crashed and I want to change it to new instance](#change-instance-after-crash)
  - [Kafka fails with timeout](#kafka-broker-operations-timeout)

## Why MaaS responds with Forbidden (403) error code with correct credentials
Case 1: it is MaaS clean install or rolling update error - script failing doesn't fail whole job
Before 3.0.0 version any sh script in deployment (openshift) folder didn't have proper error handling behavior and in case of failing of sh script, e.g. creating account manager, whole Jenkins job wouldn't fail. So if you have some problems with accounts, database or secrets check if there is any error in Jenkins job. Sometimes container could have not been created inside pod and that leads to fail of scripts. In that case just run the job again.

Case 2: inappropriate role usage. Different requests require creds with different roles (agent or manager), they are mentioned in [rest_api.md](rest_api.md) file. See security model of MaaS for more info: ../README.md#security-model

## Why Cloud-Core deployment fails with validation of MaaS

Reason: Kafka or Rabbit instances are not registered

Due to the fact, that MaaS is usually installed by Cloud Ops and then used by different application teams, it could happen that MaaS was installed, but instances were not registered.

There is a validation.sh script for Cloud Core, checking that at least one instance is registered either for Rabbit or Kafka. In that case, error in deployer will look like this:

```json
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
* [rest_api.md#get-rabbit-instances](rest_api.md#get-rabbit-instances)
* [rest_api.md#get-kafka-instances](rest_api.md#get-kafka-instances)

### How to register instance

* [rest_api.md#register-rabbit-instance](rest_api.md#register-rabbit-instance)
* [rest_api.md#register-kafka-instance](rest_api.md#register-kafka-instance)

Parameters of request depend on the broker configuration, see the description below, but in difficult cases it should be checked with Cloud Ops team.

## Why Kafka instance could be unhealthy

Reason: Kafka instance was not registered correctly. The error could look like `Failed to create kafka admin client for [localhost:9092]: kafka: client has run out of available brokers to talk to (Is your cluster reachable?)`
Kafka has many different ways to both authenticate users and use encryption. MaaS supports the most of such configurations, they are described here:

[rest_api.md#kafka-auth-dto](rest_api.md#kafka-auth-dto)

### Amazon case

Amazon's Kafka (Amazon MSK) is fully supported by MaaS, but it could have its special namings and certificates. In case if Kafka is used without authentication, then typical configuration for instance is :

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

## Change instance after crash

Assume, your instance become broken and you want to change it to new instance. You want to make a reconciliation of all maas entities (topics/vhosts/...) - move them to new instance.
Then you need to look at instance update guide - it allows you to either update instance registration and trigger recovery endpoint or to redeploy your application with new maas configuration:
[Instance update](./maintenance.md#instance-update)

## Kafka broker operations timeout

If your Kafka responds too long for some reason for Kafka broker operations like list/update/create/delete, you may want to increase timeout for Kafka broker client. You can do it by changing client timeout property via env variable `KAFKA_CLIENT_TIMEOUT`. Default value is 10s.

## Recover Kafka topics and Rabbit vhosts

If you created topics or vhosts via MaaS then information about these entities is stored in MaaS registry. In case if for some reason topics were deleted in Kafka or vhosts were deleted in RabbitMQ, then you can use special APIs to recover them:

### Single topic recovery
[Kafka Single Topic Recovery](./rest_api.md#Kafka-Single-Topic-Recovery)

### Multiple topics recovery (namespace level)
[Kafka Topics Reconciliation](./rest_api.md#Kafka-Topics-Reconciliation)

### Rabbit vhost recovery (namespace level)
[Rabbit namespace recovery](./rest_api.md#Rabbit-namespace-recovery)

