package com.netcracker.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

@Data
@Builder
public class KafkaTenantTopicConfigResponse {
    private SingleReply[] replies;

    public KafkaTenantTopicConfigResponse() {
    }

    public KafkaTenantTopicConfigResponse(SingleReply[] replies) {
        this.replies = replies;
    }

    @Data
    public static class SingleReply {
        KafkaTenantTopicConfigRequest request;
        KafkaTenantTopicConfigReplyResult result;

        public SingleReply(KafkaTenantTopicConfigRequest request, KafkaTenantTopicConfigReplyResult result) {
            this.request = request;
            this.result = result;
        }

        public SingleReply() {
        }

    }

    @Data
    public static class KafkaTenantTopicConfigReplyResult {
        String status;
        String error;
        SyncTenantsResponse[] data;
    }

}
