# Kafka blue-green design

In a typical scenario, after the warmup operation for each topic in the origin namespace, an alias is created using the peer namespace in the classifier.

For example, suppose your topic is declared in the Maas configuration. During application deployment, this configuration will simply create a regular topic.

```yaml
apiVersion: nc.maas.kafka/v1
kind: topic
pragma:
  on-entity-exists: merge
spec:
  name: my-topic
  classifier:
    name: my-topic
    namespace: ${ORIGIN_NAMESPACE}
  numPartitions: 3
  replicationFactor: inherit
```

Important note: Please note that the `${ORIGIN_NAMESPACE}` variable is used instead of `${NAMESPACE}`. This ensures that the topic remains linked to the origin namespace, so if you delete the peer namespace, the topic will still be available in the origin namespace.

## BG operations behavior
* Warmup – During warmup, no additional topics are created. Kafka will continue to have a single topic with the pattern `maas.my-namespace.my-topic`. However, this topic is now also accessible from the peer namespace using a peer classifier:
```yaml
classifier:
    name: my-topic
    namespace: ${PEER_NAMESPACE}
```
* Promote and Rollback - these operations don't affect Kafka topics directly.
* Commit - operation removes legacy version and returns system to initial state, so correctly declared topic will still exist, but won't be accessed from peer namespace anymore.


## Versioned topics
In rare cases, you may want to clone topics from the origin to the peer namespace. Consider this carefully, as it comes with several downsides:
* topics are cloned to peer namespace without stored messages during warmup procedure
* they would be deleted if you make commit operation
* real topic name must contain namespace in its name: Example: {{namespace}}-mytopic.
  `Warning!` Must not include statically defined namespace value. Example of wrong definition: ${NAMESPACE}-mytopic
  Also you can omit such declaration and rely on predefined naming template by MaaS.
* offsets for cloned topics are not stored, so you need to manage them manually

If you still choose to use this approach, declare the topic with the `versioned` parameter set to `true`. Use the `${NAMESPACE}` variable, as it will differ for each cloned topic. This setting only affects the warmup operation:
```yaml
apiVersion: nc.maas.kafka/v1
kind: topic
pragma:
  on-entity-exists: merge
spec:
  name: my-topic-{{namespace}}
  classifier:
    name: my-topic
    namespace: ${NAMESPACE}
  numPartitions: 3
  replicationFactor: inherit
  versioned: true
```
