package org.qubership.it.maas.entity.rabbit;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Data
@NoArgsConstructor
@AllArgsConstructor
public class MsConfig {
    private String msName;
    private Map<String, Object>[] exchanges;
    private Map<String, Object>[] queues;
    private Map<String, Object>[] bindings;
}
