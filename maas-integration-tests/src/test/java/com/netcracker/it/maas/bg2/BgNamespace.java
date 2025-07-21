package com.netcracker.it.maas.bg2;

import lombok.Value;

@Value
public class BgNamespace {
    private String name;
    private String state;
    private String version;
}