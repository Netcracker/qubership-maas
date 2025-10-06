package org.qubership.it.maas.entity.kafka;

import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Map;

@Data
@NoArgsConstructor
public class KafkaInstance {
    private String id;
    private Map<String, List<String>> addresses;
    @JsonProperty("default")
    private Boolean isDefault;
    private String maasProtocol;
    private String caCert;
    private Map<String, List<Map<String, String>>> credentials;

    public KafkaInstance(KafkaInstance orig) {
        this.id = orig.id;
        this.addresses = orig.addresses;
        this.isDefault = orig.isDefault;
        this.maasProtocol = orig.maasProtocol;
        this.caCert = orig.caCert;
        this.credentials = orig.credentials;
    }
}
