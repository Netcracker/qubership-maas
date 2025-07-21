package com.netcracker.it.maas.entity;

import com.netcracker.it.maas.entity.rabbit.RabbitConfigReq;
import com.netcracker.it.maas.entity.rabbit.RabbitConfigResp;
import lombok.Data;

import java.util.List;

@Data
public class ConfigV2Resp {
    private List<MsResponses> msResponses;
    private String status;
    private String error;

    @Data
    public static class MsResponses {
        Request request;
        //todo think of how to add here kafka too and how deserialize it manually
        RabbitConfigResp.RabbitConfigReplyResult result;

        public MsResponses() {
        }

        public MsResponses(Request request, RabbitConfigResp.RabbitConfigReplyResult result) {
            this.request = request;
            this.result = result;
        }
    }

    @Data
    public static class Request {
        private String serviceName;
        private RabbitConfigReq config;

        public Request() {
        }

        public Request(String serviceName, RabbitConfigReq config) {
            this.serviceName = serviceName;
            this.config = config;
        }
    }
}
