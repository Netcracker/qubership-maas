package com.netcracker.it.maas.entity.rabbit;

import lombok.Data;

@Data
public class VirtualHostResponse {
    private String cnn;
    private String username;
    private String password;
}
