package org.qubership.it.maas.entity.rabbit;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class CpMessage {
    private String version;
    private String stage;
}
