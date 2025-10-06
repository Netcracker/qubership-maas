package org.qubership.it.maas.entity.kafka;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Map;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class KafkaTemplateConfigResponse {
    private SingleReply[] replies;

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class SingleReply {
        private KafkaTemplateConfigRequest request;
        private KafkaTemplateConfigReplyResult result;
    }

    @Data
    public static class KafkaTemplateConfigReplyResult {
        private String status;
        private String error;
        private KafkaTemplateConfigReplyData data;
    }

    @Data
    @NoArgsConstructor
    public static class KafkaTemplateConfigReplyData {
        private String name;
        private String namespace;
        private TopicSettings currentSettings;
        private TopicSettings previousSettings;
        private Map<String, Object>[] updatedTopics;
    }

    @Data
    @NoArgsConstructor
    public static class TopicSettings {
        private Integer numPartitions;
        private Integer replicationFactor;
        private Map<Integer, List<Integer>> replicaAssignment;
        private Map<String, Object> configs;
    }
}
