package com.netcracker.it.maas.entity;

import com.netcracker.it.maas.entity.rabbit.RabbitConfigReq;
import lombok.Builder;
import lombok.Data;

import java.util.Map;

@Data
public class ConfigV2Req {
    private String apiVersion = "nc.maas.config/v2";
    private String kind = "config";
    //private ConfigV1[] spec;
    private Spec spec;

    @Data
    @Builder
    public static class Spec {
        String version;
        String namespace;
        Service[] services;

        public Spec() {
        }

        public Spec(String version, String namespace, Service[] services) {
            this.version = version;
            this.namespace = namespace;
            this.services = services;
        }
    }


    @Data
    @Builder
    public static class Service {
        String serviceName;
        String config;
    }


    public ConfigV2Req() {
    }

    public ConfigV2Req(Spec spec) {
        this.spec = spec;
    }
}
