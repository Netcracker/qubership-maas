package org.qubership.it.maas.entity.kafka;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class SyncTenantsResponse {
    private Map<String, Object> tenant;
    private KafkaTopicResponse[] topics;
}
