package com.netcracker.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

import java.util.Collection;
import java.util.Map;

@Data
@Builder
public class KafkaConfigReq {

    private String apiVersion;
    private String kind;
    private Spec spec;

    @Data
    public static class Spec {
        private String name;
        private Map<String, Object> classifier;
        private String instance;
        private Boolean externallyManaged;
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, Collection<Integer>> replicaAssignment;
        private Map<String, String> configs;

        public Spec() {
        }

        public Spec(String name, Map<String, Object> classifier) {
            this.name = name;
            this.classifier = classifier;
        }

        public Spec(String name, Map<String, Object> classifier, Boolean externallyManaged) {
            this.name = name;
            this.classifier = classifier;
            this.externallyManaged = externallyManaged;
        }
    }


    public KafkaConfigReq(String apiVersion, String kind, Spec spec) {
        this.apiVersion = apiVersion;
        this.kind = kind;
        this.spec = spec;
    }

    public KafkaConfigReq() {

    }



}
