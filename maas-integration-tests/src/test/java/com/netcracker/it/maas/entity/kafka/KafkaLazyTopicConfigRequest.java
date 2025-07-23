package com.netcracker.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

import java.util.Collection;
import java.util.Map;

@Data
@Builder
public class KafkaLazyTopicConfigRequest {
    private String apiVersion = "nc.maas.kafka/v1";
    private String kind = "lazy-topic";
    private KafkaLazyTopicConfigRequest.Spec spec;

    public KafkaLazyTopicConfigRequest() {
    }

    public KafkaLazyTopicConfigRequest(String apiVersion, String kind, Spec spec) {
        this.spec = spec;
    }

    @Data
    @Builder
    public static class Spec {
        private String name;
        private Map<String, Object> classifier;
        private String instance;
        private Integer numPartitions;
        private String replicationFactor;
        private Map<Integer, Collection<Integer>> replicaAssignment;
        private Map<String, String> configs;
        private String template;



        public Spec(String name, Map<String, Object> classifier, String instance, Integer numPartitions,
                    String replicationFactor, Map<Integer, Collection<Integer>> replicaAssignment,
                    Map<String, String> configs, String template) {
            this.name = name;
            this.classifier = classifier;
            this.instance = instance;
            this.numPartitions = numPartitions;
            this.replicationFactor = replicationFactor;
            this.replicaAssignment = replicaAssignment;
            this.configs = configs;
            this.template = template;
        }

        public Spec() {
        }
    }
}
