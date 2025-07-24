package org.qubership.it.maas.entity.kafka;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@Builder
@NoArgsConstructor
@AllArgsConstructor
public class KafkaTenantTopicConfigResponse {
    private SingleReply[] replies;

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class SingleReply {
        private KafkaTenantTopicConfigRequest request;
        private KafkaTenantTopicConfigReplyResult result;
    }

    @Data
    public static class KafkaTenantTopicConfigReplyResult {
        private String status;
        private String error;
        private SyncTenantsResponse[] data;
    }
}
