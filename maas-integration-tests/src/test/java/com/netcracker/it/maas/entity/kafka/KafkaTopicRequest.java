package com.netcracker.it.maas.entity.kafka;

import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.Builder;
import lombok.Data;

import java.util.Collection;
import java.util.Map;

@Data
@Builder
@JsonInclude(JsonInclude.Include.NON_NULL)
public class KafkaTopicRequest {
    private String name;
    private Map<String, Object> classifier;
    private String instance;
    private Boolean externallyManaged;
    private Integer numPartitions;
    private String replicationFactor;
    private Map<Integer, Collection<Integer>> replicaAssignment;
    private Map<String, String> configs;
    private String template;
    private boolean versioned;
}
