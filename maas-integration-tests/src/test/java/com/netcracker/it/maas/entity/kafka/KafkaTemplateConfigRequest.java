package com.netcracker.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

import java.util.Collection;
import java.util.Map;

@Data
@Builder
public class KafkaTemplateConfigRequest {
    private String apiVersion;
    private String kind;
    private KafkaTemplateConfigRequest.Spec spec;

    public KafkaTemplateConfigRequest() {
    }

    public KafkaTemplateConfigRequest(String apiVersion, String kind, Spec spec) {
        this.apiVersion = apiVersion;
        this.kind = kind;
        this.spec = spec;
    }

    @Data
    @Builder
    public static class Spec {
        private String name;
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, Collection<Integer>> replicaAssignment;
        private Map<String, String> configs;

        public Spec() {
        }

        public Spec(String name, Integer numPartitions, Integer replicationFactor, Map<Integer, Collection<Integer>> replicaAssignment, Map<String, String> configs) {
            this.name = name;
            this.numPartitions = numPartitions;
            this.replicationFactor = replicationFactor;
            this.replicaAssignment = replicaAssignment;
            this.configs = configs;
        }
    }
}
