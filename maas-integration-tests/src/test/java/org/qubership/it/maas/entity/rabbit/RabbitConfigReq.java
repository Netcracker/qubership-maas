package org.qubership.it.maas.entity.rabbit;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import org.qubership.it.maas.entity.ConfigV1;

import java.util.Map;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class RabbitConfigReq implements ConfigV1 {

    private String apiVersion;
    private String kind;
    private Spec spec;

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class Spec {
        private Map<String, Object> classifier;
        private String instanceId;
        private RabbitEntities entities;
        private RabbitEntities versionedEntities;
        private Map<String, Object>[] policies;
        private RabbitDeletions deletions;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RabbitDeletions {
        private Map<String, Object>[] exchanges;
        private Map<String, Object>[] queues;
        private Map<String, Object>[] bindings;
        private Map<String, Object>[] policies;
    }

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RabbitEntities {
        private Map<String, Object>[] exchanges;
        private Map<String, Object>[] queues;
        private Map<String, Object>[] bindings;
    }
}
