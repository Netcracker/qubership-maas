package com.netcracker.it.maas.entity.rabbit;

import lombok.Builder;
import lombok.Data;

import java.util.Map;

@Data
@Builder
public class VirtualHostRequest {
    private Map<String, Object> classifier;
    private String instance;
}
