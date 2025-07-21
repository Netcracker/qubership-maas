package com.netcracker.it.maas.entity.kafka;

import lombok.Data;

import java.util.List;
import java.util.Map;
import java.util.Objects;

@Data
public class KafkaTopicResponse {
    private Map<String, List<String>> addresses;
    private String name;

    private Map<String, Object> classifier;
    private String namespace;
    private String instance;
    private String template;

    private Boolean externallyManaged;

    private TopicSettings requestedSettings;
    private TopicSettings actualSettings;

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (o == null || getClass() != o.getClass()) return false;
        KafkaTopicResponse that = (KafkaTopicResponse) o;
        return addresses.equals(that.addresses) &&
                name.equals(that.name) &&
                classifier.equals(that.classifier) &&
                namespace.equals(that.namespace) &&
                instance.equals(that.instance) &&
                externallyManaged.equals(that.externallyManaged) &&
                Objects.equals(requestedSettings, that.requestedSettings);
    }

    @Override
    public int hashCode() {
        return Objects.hash(addresses, name, classifier, namespace, instance, requestedSettings);
    }

    @Data
    public static class TopicSettings {
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, List<Integer>> replicaAssignment;
        private Map<String, String> configs;
    }
}
