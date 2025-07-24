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
public class KafkaTemplateConfigRequest {
    private String apiVersion;
    private String kind;
    private KafkaTemplateConfigRequest.Spec spec;

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class Spec {
        private String name;
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, Collection<Integer>> replicaAssignment;
        private Map<String, String> configs;
    }
}
