# Declaring Kafka topics and RabbitMQ VHosts using Custom Resource syntax

[[_TOC_]]

Create file with appropriate name and `.yaml` extensions in you helm charts folder. Contents of this file may contain one
or more topic or vhost declarations separated by triple dash `---` separator.

After deploying those CRs on cloud you can watch its processing status by monitoring kubernetes events on your namespace.
Processing details also will be published in CR itself in `status` section.

Example CRs for all available entities is represented below:

## Kafka Topic
```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: Topic
metadata:
  name: orders
  namespace: '{{ .Values.NAMESPACE }}'
  labels:
    app.kubernetes.io/name: '{{ .Values.SERVICE_NAME }}'
    deployer.cleanup/allow: 'true'
    deployment.netcracker.com/sessionId: '{{ .Values.DEPLOYMENT_SESSION_ID }}'
    app.kubernetes.io/part-of: Cloud-Core
    app.kubernetes.io/processed-by-operator: core-operator
spec:
  # all properties below are optional
  pragma:
    onTopicExists: merge
  replicationFactor: inherit
  minNumPartitions: 20
  topicNameTemplate: '%namespace%.%name%'
  config:
    retention.ms: '1000'
    flush.ms: '1000'

  # this classifier section is optional and only needed to demonstrate one specific case: metadata.name value should
  # conform to DNS spec and prohibits chars like underscore and colon. To provide backward compatibility with old configuration
  # maas-configuration.xml that permits such chars, we introduce this classifier name override section
  # Resulting classifier.name will be overridden by defined value below
  classifier:
    name: my_orders
```
## Kafka Tenant Topic
```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: TenantTopic
metadata:
  name: orders
  namespace: '{{ .Values.NAMESPACE }}'
  labels:
    app.kubernetes.io/name: '{{ .Values.SERVICE_NAME }}'
    deployer.cleanup/allow: 'true'
    deployment.netcracker.com/sessionId: '{{ .Values.DEPLOYMENT_SESSION_ID }}'
    app.kubernetes.io/part-of: Cloud-Core
    app.kubernetes.io/processed-by-operator: core-operator
spec:
  # all properties below are optional
  replicationFactor: inherit
  minNumPartitions: 10
  topicNameTemplate: '%namespace%.%name%.%tenantId%'
  config:
    retention.ms: "1000"
    compression.type: gzip
```

## Kafka Lazy Topic
```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: LazyTopic
metadata:
  name: orders
  namespace: '{{ .Values.NAMESPACE }}'
  labels:
    app.kubernetes.io/name: '{{ .Values.SERVICE_NAME }}'
    deployer.cleanup/allow: 'true'
    deployment.netcracker.com/sessionId: '{{ .Values.DEPLOYMENT_SESSION_ID }}'
    app.kubernetes.io/part-of: Cloud-Core
    app.kubernetes.io/processed-by-operator: core-operator
spec:
  replicationFactor: inherit
  minNumPartitions: 10
  template: OrdersTopicTemplate
  config:
    cleanup.policy: delete
    index.interval.bytes: '2048'
```

## Kafka Topic Template
```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: TopicTemplate
metadata:
  name: MyTemplate
  namespace: '{{ .Values.NAMESPACE }}'
  labels:
    app.kubernetes.io/name: '{{ .Values.SERVICE_NAME }}'
    deployer.cleanup/allow: 'true'
    deployment.netcracker.com/sessionId: '{{ .Values.DEPLOYMENT_SESSION_ID }}'
    app.kubernetes.io/part-of: Cloud-Core
    app.kubernetes.io/processed-by-operator: core-operator
spec:
  replicationFactor: inherit
  minNumPartitions: 10
  config:
    retention.ms: "1000"
```

## RabbitMQ VHost

In `entities` section you can put 3 types of entities: exchanges, queues, bindings. Parameters that you set there are defined by RabbitMq itself - MaaS just send them as is. The mandatory parameter for queue and exchange is "name". For binding - source and destination.
`deletions` section allows declarative deletion. It is applied before other sections.
`policies` section allows creation of policies of RabbitMQ.


```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: VHost
metadata:
  name: public
  namespace: '{{ .Values.NAMESPACE }}'
  labels:
    app.kubernetes.io/name: '{{ .Values.SERVICE_NAME }}'
    deployer.cleanup/allow: 'true'
    deployment.netcracker.com/sessionId: '{{ .Values.DEPLOYMENT_SESSION_ID }}'
    app.kubernetes.io/part-of: Cloud-Core
    app.kubernetes.io/processed-by-operator: core-operator
spec:
  classifier:
    name: core_public
  entities:
    exchanges:
      - name: test-exchange
        type: topic
        durable: false
        auto_delete: false
        arguments:
          some-argument: smth
          second-arg: second
      - name: cpq-quote-modify-dlx1
        type: fanout
    queues:
      - name: test-queue
        durable: false
        auto_delete: false
        arguments:
          x-dead-letter-exchange: cpq-quote-modify-dlx1
      - name: dlx-queue
        durable: true
    bindings:
      - source: test-exchange
        destination: test-queue
        routing_key: one-more
        arguments:
          x-some-header: smth2
      - source: cpq-quote-modify-dlx1
        destination: dlx-queue
   deletions:
      exchanges:
        - name: test-exchange-to-be-deleted
      queues:
        - name: test-queue-to-be-deleted
      bindings:
        - source: test-exchange-to-be-deleted
          destination: test-queue-to-be-deleted
          properties_key: routing~7XFWLA
      policies:
        - name: old-policy-to-be-deleted
          pattern: ".*"
          definition:
            max-length: 1000
  policies:
    - name: "<name of your policy>"
      pattern: "<a regular expression that matches one or more queue (exchange) names>"
      apply-to: "<match policy to only queues, only exchanges, or both>"
      priority: "<priority of policy>"
      definition:
        "<first parameter>": "<first value>"
        "<second parameter>": "<second value>"
```

# Declarative configuration to CR migration

MaaS now supports configuring Kafka/RabbitMQ entities via CRs. This guide describes how to migrate `maas-configuration.yaml` config to
new CRs.

## Migration Template

All MaaS supported `maas-configuration.yaml` entities had a common declaration structure and new CRs have such commonality.
All CRs structure consists of:
```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: <maas-entity-kind>
metadata:
  name: <classifier-name>
  namespace: <classifier-namespace>
  labels:
    app.kubernetes.io/name: "{{ .Values.SERVICE_NAME }}"
    deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}
    app.kubernetes.io/part-of: "Cloud-Core"
    deployer.cleanup/allow: "true"
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  ...
  <the-rest-of-declaration>
  ...
```
Diagram below describes properties migrations between declarations:
![declarative-to-cr.png](img%2Fdeclarative-to-cr.png)

Note: It is also possible to override _topic/vhost_ name from `metadata.name` if it contains prohibited characters by
putting wanted _topic/vhost_ name to `spec.classifier.name` property.

## Kafka

Perform these steps to convert declarative entity to CR:
* use template declared above for CRs skeleton as starting point
* copy `spec.classifier.name` to `metadata.name`
* copy `spec.classifier.namespace` to `metadata.namespace`
* take value from `kind`, remove hyphens and capitalize then put it as value to `subKind` key
* copy `spec.pragma` to `spec.pragma` section
* all the rest properties from `spec` should be copied under `spec` section
* `spec.name` field was removed to `spec.topicNameTemplate`
* replace substitution(namespace, name, topicId) editing from `{{` `}}` to `%` in value of `spec.topicNameTemplate`

Conversion example from:
```yaml
apiVersion: nc.maas.kafka/v1
kind: topic
pragma:
  on-entity-exists: merge
spec:
  classifier:
    name: orders
    namespace: ${ENV_NAMESPACE}
  name: "{{namespace}}-orders-{{tenantId}}"
  minNumPartitions: 20
  replicationFactor: inherit
```

become to:
```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: Topic
metadata:
  name: orders
  namespace: {{ .Values.NAMESPACE }}
  labels:
    app.kubernetes.io/name: "{{ .Values.SERVICE_NAME }}"
    deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}
    app.kubernetes.io/part-of: "Cloud-Core"
    deployer.cleanup/allow: "true"
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  replicationFactor: inherit
  pragma:
    onEntityExists: merge
  minNumPartitions: 20
  topicNameTemplate: '%namespace%-orders-%name%'
```

## Rabbit

The old way how you could declare your configuration for maas was to make `maas-configuration.yaml` file like this:

```yaml
apiVersion: nc.maas.rabbit/v2
kind: vhost
spec:
    classifier:
      name: demo-vhost
      namespace: ${ENV_NAMESPACE}
    entities:
        exchanges:
        - name: e1
        queues:
        - name: q1
        bindings:
        - source: e1
        - destination: q1
    deletions:
      exchanges:
        - name: test-exchange
      queues:
        - name: test-queue
      bindings:
        - source: test-exchange
          destination: test-queue
          properties_key: routing~7XFWLA
      policies:
        - name: old-policy-to-be-deleted
          pattern: ".*"
          definition:
            max-length: 1000
    policies:
      - name: "<name of your policy>"
        pattern: "<a regular expression that matches one or more queue (exchange) names>"
        apply-to: "<match policy to only queues, only exchanges, or both>"
        priority: "<priority of policy>"
        definition:
          "<first parameter>": "<first value>"
          "<second parameter>": "<second value>"
```

New way is to put a new yaml file named like your vhost in `deployments` folder.
Your classifier fields 'name' and 'namespace' moved under 'metadata' section.

Old config would be transformed like this:

```yaml
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: VHost
metadata:
  name: demo-vhost
  namespace: "{{ .Values.NAMESPACE }}"
  labels:
    app.kubernetes.io/name: "{{ .Values.SERVICE_NAME }}"
    deployment.netcracker.com/sessionId: {{ .Values.DEPLOYMENT_SESSION_ID }}
    app.kubernetes.io/part-of: "Cloud-Core"
    deployer.cleanup/allow: "true"
    app.kubernetes.io/processed-by-operator: "core-operator"
spec:
  entities:
    exchanges:
      - name: e1
    queues:
      - name: q1
    bindings:
      - source: e1
      - destination: q1
  deletions:
    exchanges:
      - name: test-exchange
    queues:
      - name: test-queue
    bindings:
      - source: test-exchange
        destination: test-queue
        properties_key: routing~7XFWLA
    policies:
      - name: old-policy-to-be-deleted
        pattern: ".*"
        definition:
          max-length: 1000
  policies:
    - name: "<name of your policy>"
      pattern: "<a regular expression that matches one or more queue (exchange) names>"
      apply-to: "<match policy to only queues, only exchanges, or both>"
      priority: "<priority of policy>"
      definition:
        "<first parameter>": "<first value>"
        "<second parameter>": "<second value>"
```
