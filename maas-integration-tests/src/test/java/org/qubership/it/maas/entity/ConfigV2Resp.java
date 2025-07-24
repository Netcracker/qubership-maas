package org.qubership.it.maas.entity;

import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;
import org.qubership.it.maas.entity.rabbit.RabbitConfigReq;
import org.qubership.it.maas.entity.rabbit.RabbitConfigResp;
import lombok.Data;

import java.util.List;

@Data
public class ConfigV2Resp {
    private List<MsResponses> msResponses;
    private String status;
    private String error;

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class MsResponses {
        private Request request;
        //todo think of how to add here kafka too and how deserialize it manually
        private RabbitConfigResp.RabbitConfigReplyResult result;
    }

    @Data
    @NoArgsConstructor
    @AllArgsConstructor
    public static class Request {
        private String serviceName;
        private RabbitConfigReq config;
    }
}
