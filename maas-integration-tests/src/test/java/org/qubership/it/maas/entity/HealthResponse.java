package org.qubership.it.maas.entity;

import lombok.AccessLevel;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;
import lombok.Value;

@Value
@NoArgsConstructor(force = true, access = AccessLevel.PRIVATE)
@AllArgsConstructor
public class HealthResponse {
    HealthItem postgres;
    HealthItem rabbit;
    HealthItem status;
}
