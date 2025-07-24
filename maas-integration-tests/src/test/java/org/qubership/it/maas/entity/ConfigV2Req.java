package org.qubership.it.maas.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
public class ConfigV2Req {
    private String apiVersion = "nc.maas.config/v2";
    private String kind = "config";
    private Spec spec;

    @Data
    @Builder
    @NoArgsConstructor
    @AllArgsConstructor
    public static class Spec {
        private String version;
        private String namespace;
        private Service[] services;
    }

    @Data
    @Builder
    public static class Service {
        private String serviceName;
        private String config;
    }

    public ConfigV2Req(Spec spec) {
        this.spec = spec;
    }
}
