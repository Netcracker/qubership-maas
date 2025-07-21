package com.netcracker.it.maas.entity.kafka;

import com.netcracker.it.maas.entity.kafka.KafkaTopicResponse;
import lombok.Data;

import java.util.Map;

@Data
public class SyncTenantsResponse {
    Map<String, Object> tenant;
    KafkaTopicResponse[] topics;

    public SyncTenantsResponse(Map<String, Object> tenant, KafkaTopicResponse[] topics) {
        this.tenant = tenant;
        this.topics = topics;
    }

    public SyncTenantsResponse() {
    }

}
