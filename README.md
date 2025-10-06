[![Go build](https://github.com/Netcracker/qubership-maas/actions/workflows/maas---build-on-push.yaml/badge.svg)](https://github.com/Netcracker/qubership-maas/actions/workflows/go-build.yml)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?metric=coverage&project=Netcracker_qubership-maas)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-maas)
[![duplicated_lines_density](https://sonarcloud.io/api/project_badges/measure?metric=duplicated_lines_density&project=Netcracker_qubership-maas)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-maas)
[![vulnerabilities](https://sonarcloud.io/api/project_badges/measure?metric=vulnerabilities&project=Netcracker_qubership-maas)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-maas)
[![bugs](https://sonarcloud.io/api/project_badges/measure?metric=bugs&project=Netcracker_qubership-maas)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-maas)
[![code_smells](https://sonarcloud.io/api/project_badges/measure?metric=code_smells&project=Netcracker_qubership-maas)](https://sonarcloud.io/summary/overall?id=Netcracker_qubership-maas)


# MaaS
MaaS stands for Messaging as a Service. It offers to create and manage entities on RabbitMQ and Kafka brokers: 
* RabbitMQ - vhosts, exchanges, queues and bindings
* Kafka - topics  

Entities can be added using declarative configuration or via direct REST API calls. Also MaaS intended to help support Blue/Green application deployments.

# Architecture
MaaS should be installed in cloud in its own namespace. MaaS requires PostgresSQL database to persist its data. After installation, operations team should register RabbitMQ and Kafka instances in MaaS either using REST API or ENV variables during installation. Application that planning to use MaaS is need to enable MaaS integration in CMDB for its namespace. Application microservices shouldn't interact with MaaS service directly, it must use maas-agent as security proxy and perform all request with M2M token. 

MaaS service DOES NOT proxies or tunneling connections from microcervice to message brokers. Application microservices receives from MaaS only metainformation about vhost/topic with host address and credentials to target broker. Then miscroservice establish direct connection to message broker using provided parameters from MaaS.   

# Security model
There are two roles in MaaS: 
* `manager`
    * grants ability to create/delete user accounts in MaaS
    * register RabbitMQ and Kafka instances
* `agent`
    * role grants all API operations except listed above for `manager` role, like create/delete VHost objects in RabbitMQ, create/delete topics in Kafka, etc 

Deployer during MaaS installation creates one mandatory user account - `admin` that responsible for user management. Its account only has `manager` role. Its account intended for use only by operations team.

Also in typical MaaS installation we recommend creating account for deployer with both roles: `manager` and `agent` assigned to it. We need both because:
* `manager` role is needed to create dedicated user for maas-agent attached to specific namespace. Secondly deployer use this role to create RabbitMQ VHost as way to isolate entities between namespaces. 
* `agent` role is needed to create low level entities in RabbitMQ and Kafka such as: exchanges, queue and topics. Its role will be used by deployer if you intend to use `declarative approach` to create your entities in deploy time via config files in `deployment/maas-configuration.yaml`.

# Classifier
Classifier is an identifier for your entities like topics, vhosts, etc. It should be unique for your entity. It consists of 3 fields: 
```json
    {
        "name": "<your any custom name>",
        "tenantId": "(Optional) <external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
    }
```
Fields "name" and "namespace" are mandatory. "tenantId" is optional. "namespace" field should be equal to namespace where request to MaaS comes from (or it could be from another namespace if they have same base-namespace in composite platform).
In particular cases API provide explicit way to specify exact name for Kafka topics.
For example, for Kafka topic in case if you don't put "name" field in request (outside of classifier), then classifier is used to generate Kafka topic name by next contract: 
In case if you have "tenantId" field:
```
"maas.{{namespace}}.{{tenantId}}.{{name}}" 
```

In case if you don't have "tenantId" field:
```
"maas.{{namespace}}.{{name}}" 
```

You can put your own template for topic name (it includes lazy and tenant topics). You can use parameters {{namespace}}, {{tenantId}}, {{name}} and they will be replaced with real values from your classifier. Note, that yaml recognize curly braces as special character, so we need to wrap your value in string literal (surround with `"`). Example of topic config with custom template: 
```yaml
apiVersion: nc.maas.kafka/v1
kind: topic
spec:
  name: "{{namespace}}-{{tenantId}}-{{name}}"
  classifier:
    name: topic-name
    namespace: topic-namespace
    tenantId: tenant-id
  numPartitions: 1
```

# RabbitMQ/Kafka entities creation
There are two approaches to create and manipulate entities in RabbitMQ and Kafka using MaaS.You can choose better suited for your case or even use both at the same time: 
* declarative approach - suitable if you have static set of entities well-known on application deployment time. Big advantage is that it's much simpler to declare entities in config file that write management code in Java/Go/Other lang via REST API.
* dynamic/lazy - it's the only way for you if you don't know your entities set on deploy time. One of use cases is multitenant solutions. At the moment, use MaaS Rest API to manipulate your entities in runtime. Later core team plan to release MaaS client lib for Java and Go to simplify your life.

## Manual entities management restriction 
If you use MaaS for your Kafka or RabbitMQ instance, then all actions like create\delete\update entities (topics, exchanges, etc.) should be done via MaaS and `NOT manually`. If you do it without MaaS, then database will be in inconsistent state which can lead to malfunction of whole system.

## Kafka templates, lazy topics, tenant topics
You can create your `templates` with necessary properties and then just mention it during topic or lazy-topic creation instead of describing properties. Adding a new template to MaaS doesn't trigger any topic creations. The main purpose of this entity is to reduce configuration repetitions during topic definitions. The link to the template is stored to derived topics. So, when you change original template parameters, it triggers new value propagation to all linked topics. Template parameters have higher priority over parameters declared on target entities. The api is described in api doc.

Another feature is `lazy-topic` - postponed creation of topic. You can think of it like a request of topic creation, which is stored in MaaS DB and could be triggered by corresponding api endpoint only by classifier. It could be useful if you want to describe topic beforehand using declarative approach and then just create it by name in your code. 
Lazy topic is an entity stored in register of maas, defining lazy topic does not create anything in Kafka  
Main feature of lazy topic is that you can define topic with partially-filled classifier (e.g. `{ "name" : "my-topic", "namespace" : "my-namespace" }` ) with all proper configurations  
Then you can create topic by lazy topic (see api below) only with fully-filled classifier, adding for example tenantId (e.g. `{ "name" : "my-topic",  "namespace" : "my-namespace", "tenantId" : "123" }` )  
And topic will be created with fully-filled classifier but with configs from lazy topics.
The api is also described in api doc.

`tenant-topic` - MaaS's entity, which is linked to tenants of a particular namespace. So you can declare tenant-topic (it is similar with lazy topic, only request of topic creation is stored) and for every tenant in your namespace (new or which were already activated) topic will be created with parameters and classifier you declared in tenant-topic, but classifier will have one more field - `tenantId` with externalId of concrete tenant. So every active tenant will have a topic defined by tenant-topic.
Note! If tenant is deactivated or deleted, topics assigned to this tenant will stay intact. No topic deletion will be performed.

To use `tenant-topic` feature in your namespace you should have Cloud-Core version `6.32` or later.
Otherwise, tenant-topics could be created in maas, but no tenants would be synchronised and therefore no real topic will be created for any tenant (until you update Cloud-Core, during rolling update for all existing activated tenants for every tenant-topic topic will be created)
Since Cloud-Core `6.32` tenant-manager have a task that sends ALL active tenants to maas-agent for every new active tenant. If maas-agent is configured to work with maas (Cloud-Core was deployed with necessary parameters) then maas-agent will synchronise all new tenants with tenant topics

## Instance designators

Instance designator is a feature that allows you to choose instance for your kafka topic/rabbit vhost more selectively. For example, you can designate the specific instance for topics of special tenant or exclusive instance for the particular topic. 

```yaml
apiVersion: nc.maas.kafka/v2
kind: instance-designator
spec:
  namespace: namespace
  defaultInstance: default-instance-name
  selectors:
  - classifierMatch:
      name: topic-in-internal-kafka
    instance: cpq-internal-kafka
  - classifierMatch:
      name: production-topic
    instance: customer-external-kafka
```
Instance designator configuration can be set via CMDB parameter with name `MAAS_CONFIG` on cloud or namespace level. 

Let's observe this example of config:  
* `apiVersion`, `kind` - mandatory fields which defines config type  
* `namespace` - your project namespace. Note, that you can have only one instance designator config per namespace
* `defaultInstance` - optional field if you want to declare default instance for all unmatched topics. See the priority of instance choosing described below.
* `selectors` - structure that consists of `classfierMatch` and `instance` fields. Instance would be used if `classifierMatch` matched with particular classifier of the topic.  
Classifier Match structure is similar with approach used in lazy topic - selector will be chosen if all fields from classifier match structure are a subset of classifier fields. For example, if classifier match has field `name: production-topic` and topic has classifier `{ "name" : "production-topic",  "namespace" : "my-namespace"}`, then selector will be chosen for this topic. That's how you can declare instance for all topics of particular tenant.  

Note, that order in selectors list is important and declares matching priority. In our example at first selector with classifier match with name `name: topic-in-internal-kafka` will be tried. And, if it is not compatible with topic classifier, next selector with classifier match with `name: production-topic` will be tried and so on.  

It is also possible to use wildcards as `classifierMatch` value property, e.g. `name: prefixed-*-n-??`. Where `*` means any number of chars and `?` for one any char.

The order of resolving instance for topic:
1. If there is no instance designator:
    1. If there is instance id field in topic it will be chosen.
    2. If there is no instance id - default MaaS instance will be chosen.
2. If there is instance designator:
    1. If there is a compatible selector, instance will be resolved from it.
    2. If there is no compatible selector, then defaultInstance of instance designator is checked, if it is not null, instance will be resolved from it.
    3. If there is no default instance designator instance then default MaaS instance will be chosen.

There is also api that allow you to get or delete instance designator of particular namespace, see REST api docs. 

Same rules are working for RabbitMQ instance designators, but you should use `apiVersion: nc.maas.rabbit/v2`

# Monitoring
There is an GET endpoint, that sends information about registered MaaS entities (topics and vhosts) in Prometheus format, example:
```
maas_rabbit{namespace="maas-it-test", microservice="order-processor", broker_host="amqps://rabbitmq.maas-rabbitmq-2:5671", vhost="maas.maas-it-test.vers-test", broker_status="UP"}
maas_rabbit{namespace="maas-it-test", microservice="", broker_host="amqps://rabbitmq.maas-rabbitmq-2:5671", vhost="maas.maas-it-test.it-test.VirtualHostBasicOperationsIT", broker_status="UP"}
maas_kafka{namespace="maas-it-test", broker_host="{'SASL_PLAINTEXT': ['kafka.maas-kafka-2:9092']}", topic="maas.maas-it-test.it-test.KafkaTopicBasicOperationsIT", broker_status="UP"}
```

# Client Libraries 
## Plain Java Client 
Library contains module with MaaS API Client, Kafka Blue/Green Consumer, Kafka Streams Support and context propagation utilities
https://github.com/Netcracker/qubership-maas-client

## Java Declarative clients
For Spring: https://github.com/Netcracker/qubership-maas-declarative-client-spring
For Quarkus: https://github.com/Netcracker/qubership-maas-declarative-client-quarkus

## Go MaaS Client
https://github.com/Netcracker/qubership-core-lib-go-maas-client
