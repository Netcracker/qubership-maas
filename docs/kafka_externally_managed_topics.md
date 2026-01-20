# Using Externally Managed Topics

MaaS provides feature called externally managed topics. Typically an externally managed topics are used for integration purposes. These topics reside in Kafka outside of the cloud and are managed by the client or other third parties. Moreover, we may not even have create/update/delete topic permissions on this Kafka broker.

Basically this feature allows you to create named alias in MaaS for external topic. This allows microservices to use existing libraries and tools to access such topics in a familiar and convenient way. 

# How to use

## Create topic on Kafka broker

Use command line tools provided by Kafka distribution to create topic in Kafka broker. For example:
```bash 
bin/kafka-topics.sh --create --bootstrap-server localhost:9092 --topic orders
```   

## Register Kafka broker instance in MaaS

Register Kafka cluster in MaaS as Kafka instance by using api: [rest_api.md#register-kafka-instance](rest_api.md#register-kafka-instance)

Please note that the instance creation API describes a case in which both the admin and client roles are used. In the case of an external Kafka with read-only access, configure the admin credentials to be the same as the client credentials.

## Declare externally managed topic

Write down topic declaration in `maas-configuration.yaml` file. Declaration of externally managed topic requires:
* configuration property `externallyManaged` set to `true` 
* specified real topic name in Kafka broker via `name` property
* topic should already exist in Kafka

Example config:
```yaml
apiVersion: nc.maas.kafka/v1
kind: topic
spec:
  # bind external topic with this classifier 
  classifier:
    name: orders
    namespace: ${NAMESPACE}

  # real topic name in Kafka, mandatory for externally managed topics
  name: orders
  # do not perform any modification operations on kafka broker for this topic,
  # just expect that this topic already exists 
  externallyManaged: true
```

Note, that MaaS does not perform any operation on topic in Kafka broker for topics marked as -externally managed-. Topic should exist in Kafka prior declaration comes to MaaS, otherwise operation will fail. Topic deletion from MaaS just performs topic registration removal from MaaS registry and real topic stay intact in Kafka.

## Specify instance for topic declaration

As you can see, topic declaration above doesn't specify `instance` property for topic. This property is still supported but deprecated, so we will show other way to bind topic declaration to instances. For those purposes MaaS introduce instance designators: instance_designators.md

Create instance designator config: 
```yaml 
apiVersion: nc.maas.kafka/v2
kind: instance-designator
spec:
  namespace: namespace
  defaultInstance: product-kafka
  selectors:
  - classifierMatch:
      name: orders
    instance: external-kafka
```

and specify it as CMDB property `MAAS_CONFIG` on namespace level. It will be something like:
```
MAAS_CONFIG=apiVersion: nc.maas.kafka/v2
kind: instance-designator
spec:
  namespace: namespace
  defaultInstance: product-kafka
  selectors:
  - classifierMatch:
      name: orders
    instance: external-kafka
```

## Deploy application 

Build and deploy your application. Topic configuration from `maas-configuration.yaml` will be merged with value from `MAAS_CONFIG` property and sent to MaaS as aggregated config during deploy phase.

Voila. 
