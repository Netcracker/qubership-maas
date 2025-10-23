# Maintenance guide

## Instance update
Instance can be updated both for RabbitMQ vhost and Kafka topic.
In some cases, you can use instance-designators, you can read about them in [README.md](../README.md)

You have 2 options how to move existing topics/vhosts to another instance:

1. First method could be used if you want to use rolling update in deployer. You can change maas-configuration.yaml file and put new instance name in request. If you used instance-designator and changed it - it will also work.
   For example, you had rabbit config using default instance:

```yamlyaml
      serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: core-dev
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
                  durable: false
```yaml

You decided to create new instance with name `rabbit-ci` and put your entities to this new instance. Then you can change your config and make rolling update for your application:
```yamlyaml
      serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            instanceId: rabbit-ci
            classifier:
              name: test
              namespace: core-dev
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
                  durable: false
```yaml

In that case, old topic/vhost WILL NOT BE deleted in broker, but removed from MaaS registry and then created in new broker instance and saved in MaaS with new instance.
As we described, for that you need to either put new instance id in vhost/kafka during getOrCreate request/maas yaml config. Or if you have instance designator and instance is resolved by it and it differs from existing instance id in topic/vhost, then topic/vhost will be moved to new instance broker. Note, that you CANNOT use both designators and instance id in request.
If you're using default RabbitMQ/Kafka instance (not putting any instance in request and have no match in instance designator), then instance of vhost/topic WILL NOT be changed if default has changed for MaaS.

2. Second method does not demand you to change instance in config, therefore no need to do rolling update for either your application or instance designator. For that, you first change the parameters of your instance using REST api and then use reconciliation endpoint to run recovery of namespace.
   For example, you had your default rabbit instance with these properties:

```yamljson
{
    "id": "rabbit-ci",
    "apiUrl": "https://rabbitmq.maas-rabbitmq-2:15671/api",
    "amqpUrl": "amqps://rabbitmq.maas-rabbitmq-2:5671",
    "user": "admin",
    "password": "admin",
    "default": true
}
```yaml

You already have vhosts with entities in this instance, but you want to copy them to another instance. Then you need to UPDATE your rabbit properties with new url-s (and credentials if they differs in new instance):
```yaml
curl --location --request PUT 'http://localhost:8080/api/v1/rabbit/instance' \
--header 'Authorization: Basic bWFuYWdlcjoyNDAwMmVhZGJj' \
--header 'Content-Type: application/json' \
--data-raw '{
    "id": "rabbit-ci",
    "apiUrl": "http://rabbitmq.maas-rabbitmq:15672/api",
    "amqpUrl": "amqp://rabbitmq.maas-rabbitmq:5672",
    "user": "admin",
    "password": "admin",
    "default": true
}'
```yaml

Now you need to run reconciliation endpoint for rabbit for your namespace:
```yaml
curl --location --request POST 'localhost:8080/api/v2/rabbit/recovery/my-namespace' \
--header 'X-Origin-Namespace: my-namespace' \
--header 'Authorization: Basic Y2xpZW50OmNsaWVudA=='
```yaml

If you received success response, then you have copied all vhosts with entities to new instance of rabbit. Note, that vhosts on previous instance WON'T be deleted, you should now do it manually if you need it.

## Passwords rotation
Diagram below shows different type of credentials used in applications built on MaaS. What each arrow number means is described in table below. Each number corresponds to top level bullet in list.
```yamlplantuml
@startuml
node "Application" {
  [Microservice] --> [maas-agent] : M2M
}
component MaaS
component RabbitMQ {
  component "VHost 1" as VHost1
  component "VHost 2" as VHost2
}
component Kafka {
  component "Topic 1" as Topic1
}
database "PostgreSQL"

[maas-agent] --> MaaS: 1
MaaS --> Kafka : 2
[Microservice] --> Kafka : 3
MaaS --> RabbitMQ : 4
[Microservice] --> VHost1 : 5
MaaS --> PostgreSQL : 6
@enduml
```yaml
1. `maas-agent` use Basic authorization mechanism proxying microservice calls to MaaS. MaaS REST API requires Basic authorization.
   1. Login and password for basic auth stored in secret `cluster-maas-agent-credentials-secret` in application namespace.
   2. Password change:
      1. Delete maas-agent account using: [rest_api.md#delete-client-account](rest_api.md#delete-client-account)
      2. Update credentials in secret `cluster-maas-agent-credentials-secret`.
      3. Register maas-agent again with new credentials by using: [rest_api.md#create-client-account](rest_api.md#create-client-account)
2. MaaS stores two types of credentials to access Kafka: Admin and Client. Admin credentials used only by MaaS itself for all topic management operations. The only purpose of storing Client credentials is to be sent to end user microservice.
   1. Admin and Client credentials is stored in postgresql database belonging to MaaS (represented in diagram above).
   2. Password change:
      1. Change password on Kafka cluster as it defined in documentation on your installation.
      2. Update changed credentials on your instance registration in MaaS. It's easier to request instance registration first, by using API call: [rest_api.md#get-kafka-instances](rest_api.md#get-kafka-instances), then edit changed credentials and send update using API call: [rest_api.md#update-kafka-instance-registration](rest_api.md#update-kafka-instance-registration)
      3. Restart your application microservices to force obtain new Kafka Client credentials from MaaS
3. Microservices uses Client credentials from MaaS To access Kafka topics in Kafka cluster. How to change it described in section #2.

4. MaaS uses RabbitMQ management plugin to perform all management operations on vhosts, exchanges, binding, queues and etc. This plugin hosted on separate url and requires login credentials too.
   1. Login and password to access RabbitMQ management plugin stored in postgresql database belonging to MaaS.
   2. Password change:
      1. Change password on RabbitMQ cluster as it defined in documentation on your installation.
      2. Update changed credentials on your instance registration in MaaS. It's easier to request instance registration first, by using API call: [rest_api.md#get-rabbit-instances](rest_api.md#get-rabbit-instances), then edit changed credentials and send update using API call: [rest_api.md#update-rabbit-instance-registration](rest_api.md#update-rabbit-instance-registration)
5. MaaS uses RabbitMQ vhost entities to separate microservice access from different namespaces. Each vhost entity has bound RabbitMQ user having full access grants to this vhost.
   1. User credentials for vhosts are stored in postgresql database belonging to MaaS
   2. To rotate password you can use this api [rest_api.md](rest_api.md#rotate-vhosts-passwords)

6. All MaaS data is stored in postgresql database. Access to postgresql database requires login and password.
   1. Login and password for database is stored in secret `maas-db-postgresql-credentials-secret` located in MaaS namespace.
      1. Change password on database in Postgresql
      2. Update secret `maas-db-postgresql-credentials-secret`
      3. Restart MaaS pod


## How to Check if There Are No Unprocessed Messages in RabbitMQ

After retrieving a RabbitMQ virtual host using the get-by-classifier API, you can inspect the state of queues to determine if there are any unprocessed messages.

### Step-by-Step Guide
Call the API Use the [/api/v2/rabbit/vhost/get-by-classifier](rest_api.md#get-rabbit-virtual-host-with-config) endpoint according to documentation.

Inspect the Response In the response JSON, locate the `entities.queues` array. Each object in this array represents a queue. You need to examine the following fields inside each queue object:

* `messages` – Total number of messages in the queue.
* `messages_ready` – Messages ready to be delivered to consumers.
* `messages_unacknowledged` – Messages that were delivered but not yet acknowledged by consumers.

### Example

Here’s a snippet from a successful response that indicates no unprocessed messages:
```yamljson
{
  "messages": 0,
  "messages_ready": 0,
  "messages_unacknowledged": 0
}
```yaml

This means:
* No messages are waiting in the queue.
* No messages are being processed by consumers.
* The queue is idle and empty.

### Noteworthy properties

`state` - Queue State. Ensure the queue is in "running" state. This confirms that the queue is operational.
`consumers` - Number of running consumers on specific queue. If queue is not empty and no consumers, then re-check application configuration or consumer program logic


# TLS setup

## PostgreSQL
MaaS can connect to PostgreSQL using TLS transport. Set `DB_POSTGRESQL_TLS` to `true` property in
properties and install or rolling update MaaS installation. TLS certificates should be attached to MaaS
deployment using platform deployer.

## Kafka
MaaS supports Kafka instances with enabled TLS transport. To register Kafka instance with TLS:
* broker host `addresses` should be specified under key `SSL` or `SASL_SSL`
* attach TLS CA certificate using field: `caCert`. Value should be encoded in base64 format.

Example:
```yamljson
{
  "addresses": {
    "SSL": ["localkafka.kafka-cluster:9094"]
  },
  "caCert": "MIID7j...6038=",
  ...
}
```yaml

## RabbitMQ
MaaS supports RabbitMQ instances with TLS support. First of all CA certificate should be attached to
MaaS server deployment before RabbitMQ instance registration. It can be done using platform deployer.
all that you need to register RabbitMQ instance with TLS is to change protocols to its secured versions:

```yamljson
{
  "apiUrl": "https://rabbitmq.maas-rabbitmq-2:15671/api",
  "amqpUrl": "amqps://rabbitmq.maas-rabbitmq-2:5671",
  ...
}
```yaml
Client microservices also should attach CA certificate to its cert storage using platform deployer.
