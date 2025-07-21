package com.netcracker.it.maas.entity.rabbit;

import com.netcracker.it.maas.entity.ConfigV1;
import lombok.Builder;
import lombok.Data;

import java.util.Map;

@Data
@Builder
public class RabbitConfigReq implements ConfigV1 {

    private String apiVersion;
    private String kind;
    private Spec spec;

    @Data
    @Builder
    public static class Spec {
        Map<String, Object> classifier;
        String instanceId;
        RabbitEntities entities;
        //@JsonProperty("versioned-entities")
        RabbitEntities versionedEntities;
        Map<String, Object>[] policies;
        RabbitDeletions deletions;

        public Spec() {
        }

        public Spec(Map<String, Object> classifier, String instanceId, RabbitEntities entities,  RabbitEntities versionedEntities, Map<String, Object>[] policies, RabbitDeletions deletions) {
            this.classifier = classifier;
            this.instanceId = instanceId;
            this.entities = entities;
            this.versionedEntities = versionedEntities;
            this.policies = policies;
            this.deletions = deletions;
        }

        @Override
        public boolean equals(Object o) {
            if (this == o) return true;
            if (!(o instanceof Spec)) return false;
            Spec spec = (Spec) o;
            return classifier.equals(spec.classifier) &&
                    instanceId.equals(spec.instanceId) &&
                    entities.equals(spec.entities);
        }

    }

    @Data
    @Builder
    public static class RabbitDeletions {
        Map<String, Object>[] exchanges;
        Map<String, Object>[] queues;
        Map<String, Object>[] bindings;
        Map<String, Object>[] policies;

        public RabbitDeletions() {
        }

        public RabbitDeletions(Map<String, Object>[] exchanges, Map<String, Object>[] queues, Map<String, Object>[] bindings, Map<String, Object>[] policies) {
            this.exchanges = exchanges;
            this.queues = queues;
            this.bindings = bindings;
            this.policies = policies;
        }
    }

    @Data
    @Builder
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

    public RabbitConfigReq(String apiVersion, String kind, Spec spec) {
        this.apiVersion = apiVersion;
        this.kind = kind;
        this.spec = spec;
    }

    public RabbitConfigReq() {

    }

    @Override
    public boolean equals(Object o) {
        if (this == o) return true;
        if (!(o instanceof RabbitConfigReq)) return false;
        RabbitConfigReq that = (RabbitConfigReq) o;
        return apiVersion.equals(that.apiVersion) &&
                kind.equals(that.kind) &&
                spec.equals(that.spec);
    }


}
