package org.qubership.it.maas.entity.rabbit;

import lombok.Data;

import java.util.Map;

@Data
public class VhostConfigResponse {
    private VirtualHostResponse vhost;
    private Map<String, Object> entities;
}
