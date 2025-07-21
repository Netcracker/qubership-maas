package com.netcracker.it.maas.entity.rabbit;

import lombok.Builder;
import lombok.Data;

import java.util.Map;

@Builder
@Data
public class MsConfig {
    String msName;
    Map<String, Object>[] exchanges;
    Map<String, Object>[] queues;
    Map<String, Object>[] bindings;

    public MsConfig() {
    }

    public MsConfig(String msName, Map<String, Object>[] exchanges, Map<String, Object>[] queues, Map<String, Object>[] bindings) {
        this.msName = msName;
        this.exchanges = exchanges;
        this.queues = queues;
        this.bindings = bindings;
    }
}
