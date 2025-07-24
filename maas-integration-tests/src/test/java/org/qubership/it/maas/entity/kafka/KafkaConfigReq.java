package org.qubership.it.maas.entity.kafka;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Collection;
import java.util.Map;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class KafkaConfigReq {

    private String apiVersion;
    private String kind;
    private Spec spec;

    @Data
    @NoArgsConstructor
    public static class Spec {
        private String name;
        private Map<String, Object> classifier;
        private String instance;
        private Boolean externallyManaged;
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, Collection<Integer>> replicaAssignment;
        private Map<String, String> configs;

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
}
