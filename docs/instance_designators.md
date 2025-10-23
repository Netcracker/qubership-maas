# Instance designators

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
* `selectors` - structure that consists of `classifierMatch` and `instance` fields. Instance would be used if `classifierMatch` matched with particular classifier of the topic.
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

You can read about instance update using designators in [maintenance.md](./maintenance.md)
