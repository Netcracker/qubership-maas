package com.netcracker.it.maas.entity.rabbit;

import lombok.Data;

@Data
public class CpMessage {
    private String version;
    private String stage;


    public CpMessage() {
    }

    public CpMessage(String version, String stage) {
        this.version = version;
        this.stage = stage;
    }
}
