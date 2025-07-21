package com.netcracker.it.maas.entity;

import lombok.AccessLevel;
import lombok.AllArgsConstructor;
import lombok.NoArgsConstructor;
import lombok.Value;

@Value
@NoArgsConstructor(force = true, access = AccessLevel.PRIVATE)
@AllArgsConstructor
public class HealthResponse {
    HeathItem postgres;
    HeathItem rabbit;
    HeathItem status;
}