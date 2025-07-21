package com.netcracker.it.maas.entity;

import com.fasterxml.jackson.annotation.JsonRootName;
import lombok.Data;

@Data
@JsonRootName(value = "error")
public class TmfErrorResponse {
    String id;
    String status;
    String code;
    String message;
    String details;
}
