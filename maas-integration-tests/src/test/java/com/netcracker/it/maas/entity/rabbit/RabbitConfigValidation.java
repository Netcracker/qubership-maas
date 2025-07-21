package com.netcracker.it.maas.entity.rabbit;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.Map;

@Data
@AllArgsConstructor
@NoArgsConstructor
public class RabbitConfigValidation {

    private LazyBinding[] bindings;

    @Data
    @Builder
    @AllArgsConstructor
    @NoArgsConstructor
    public static class LazyBinding {
        private String vhost;
        private Map<String, Object> entity;
        private String exchangeVersion;
        private String queueVersion;
    }

}
