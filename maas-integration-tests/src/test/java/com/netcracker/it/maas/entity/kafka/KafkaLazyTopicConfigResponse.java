package com.netcracker.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class KafkaLazyTopicConfigResponse {
    private SingleReply[] replies;

    public KafkaLazyTopicConfigResponse() {
    }

    public KafkaLazyTopicConfigResponse(SingleReply[] replies) {
        this.replies = replies;
    }

    @Data
    public static class SingleReply {
        KafkaLazyTopicConfigRequest request;
        KafkaLazyTopicConfigReplyResult result;

        public SingleReply(KafkaLazyTopicConfigRequest request, KafkaLazyTopicConfigReplyResult result) {
            this.request = request;
            this.result = result;
        }

        public SingleReply() {
        }

    }

    @Data
    public static class KafkaLazyTopicConfigReplyResult {
        String status;
        String error;
        KafkaLazyTopicConfigRequest.Spec data;
    }

}
