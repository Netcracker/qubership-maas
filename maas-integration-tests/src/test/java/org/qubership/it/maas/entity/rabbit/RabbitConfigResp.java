package org.qubership.it.maas.entity.rabbit;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Data
public class RabbitConfigResp {
    private SingleReply[] replies;

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class SingleReply {
        private RabbitConfigReq request;
        private RabbitConfigReplyResult result;
    }

    @Data
    public static class RabbitConfigReplyResult {
        private String status;
        private String error;
        private RabbitConfigReplyData data;
    }

    @Data
    public static class RabbitConfigReplyData {
        private VirtualHostResponse vhost;
        private RabbitEntities entities;
        private RabbitEntities versionedEntities;
        private Map<String, Object>[] policies;
        private Map<String, Object>[] updateStatus;
        private RabbitDeletedEntities deletions;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RabbitEntities {
        private Map<String, Object>[] exchanges;
        private Map<String, Object>[] queues;
        private Map<String, Object>[] bindings;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class RabbitDeletedEntities {
        private Map<String, Object>[] exchanges;
        private Map<String, Object>[] queues;
        private Map<String, Object>[] bindings;
        private Map<String, Object>[] policies;
    }
}
