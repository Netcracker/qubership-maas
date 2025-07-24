package org.qubership.it.maas.entity;

import lombok.Builder;
import lombok.Value;

import java.util.Map;

@Value
@Builder
public class SearchCriteria {
    String vhost;
    String user;
    String namespace;
    String instance;
    Map<String, Object> classifier;
}
