package org.qubership.it.maas.entity.kafka;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class KafkaLazyTopicConfigResponse {
    private SingleReply[] replies;

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class SingleReply {
        private KafkaLazyTopicConfigRequest request;
        private KafkaLazyTopicConfigReplyResult result;
    }

    @Data
    public static class KafkaLazyTopicConfigReplyResult {
        private String status;
        private String error;
        private KafkaLazyTopicConfigRequest.Spec data;
    }

}
