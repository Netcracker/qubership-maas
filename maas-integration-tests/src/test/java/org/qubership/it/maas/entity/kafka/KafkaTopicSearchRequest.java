package org.qubership.it.maas.entity.kafka;

import lombok.Builder;
import lombok.Data;

import java.util.Map;

@Data
@Builder
public class KafkaTopicSearchRequest {
    private Map<String, Object> classifier;
    private String topic;
    private String namespace;
    private String instance;
    private int template;
}
