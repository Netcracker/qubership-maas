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
public class KafkaTenantTopicConfigRequest {
    private final String apiVersion = "nc.maas.kafka/v1";
    private final String kind = "tenant-topic";

    private KafkaTenantTopicConfigRequest.Spec spec;

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class Spec {
        private String name;
        private Map<String, Object> classifier;
        private String instance;
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, Collection<Integer>> replicaAssignment;
        private Map<String, String> configs;
        private String template;
    }
}
