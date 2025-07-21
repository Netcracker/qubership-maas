package com.netcracker.it.maas.entity.rabbit;

import lombok.Data;

import java.util.Map;

@Data
public class RabbitConfigResp {
    private SingleReply[] replies;

    @Data
    public static class SingleReply {
        RabbitConfigReq request;
        RabbitConfigReplyResult result;

        public SingleReply() {
        }

        public SingleReply(RabbitConfigReq request, RabbitConfigReplyResult result) {
            this.request = request;
            this.result = result;
        }
    }

    @Data
    public static class RabbitConfigReplyResult {
        String status;
        String error;
        RabbitConfigReplyData data;
    }

    @Data
    public static class RabbitConfigReplyData {
        VirtualHostResponse vhost;
        RabbitEntities entities;
        //@JsonProperty("versioned-entities")
        RabbitEntities versionedEntities;
        Map<String, Object>[] policies;
        Map<String, Object>[] updateStatus;
        RabbitDeletedEntities deletions;
    }

    @Data
    public static class RabbitEntities {
        Map<String, Object>[] exchanges;
        Map<String, Object>[] queues;
        Map<String, Object>[] bindings;

        public RabbitEntities() {
        }

        public RabbitEntities(Map<String, Object>[] exchanges, Map<String, Object>[] queues, Map<String, Object>[] bindings) {
            this.exchanges = exchanges;
            this.queues = queues;
            this.bindings = bindings;
        }
    }


    @Data
    public static class RabbitDeletedEntities {
        Map<String, Object>[] exchanges;
        Map<String, Object>[] queues;
        Map<String, Object>[] bindings;
        Map<String, Object>[] policies;

        public RabbitDeletedEntities() {
        }

        public RabbitDeletedEntities(Map<String, Object>[] exchanges, Map<String, Object>[] queues, Map<String, Object>[] bindings, Map<String, Object>[] policies) {
            this.exchanges = exchanges;
            this.queues = queues;
            this.bindings = bindings;
            this.policies = policies;
        }
    }
}
