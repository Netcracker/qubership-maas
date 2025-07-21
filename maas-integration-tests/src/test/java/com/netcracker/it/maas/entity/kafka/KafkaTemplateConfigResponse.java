package com.netcracker.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

import java.util.List;
import java.util.Map;

@Data
@Builder
public class KafkaTemplateConfigResponse {
    private SingleReply[] replies;

    public KafkaTemplateConfigResponse() {
    }

    public KafkaTemplateConfigResponse(SingleReply[] replies) {
        this.replies = replies;
    }

    @Data
    public static class SingleReply {
        KafkaTemplateConfigRequest request;
        KafkaTemplateConfigReplyResult result;

        public SingleReply(KafkaTemplateConfigRequest request, KafkaTemplateConfigReplyResult result) {
            this.request = request;
            this.result = result;
        }

        public SingleReply() {
        }

    }

    @Data
    public static class KafkaTemplateConfigReplyResult {
        String status;
        String error;
        KafkaTemplateConfigReplyData data;
    }

    @Data
    public static class KafkaTemplateConfigReplyData {
        String name;
        String namespace;
        TopicSettings currentSettings;
        TopicSettings previousSettings;
        Map<String, Object>[] updatedTopics;

        public KafkaTemplateConfigReplyData() {
        }
    }

    @Data
    public static class TopicSettings {
        Integer numPartitions;
        Integer replicationFactor;
        Map<Integer, List<Integer>> replicaAssignment;
        Map<String, Object> configs;

        public TopicSettings() {
        }
    }
}
