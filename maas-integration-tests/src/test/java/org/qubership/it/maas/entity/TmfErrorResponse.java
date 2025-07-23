package org.qubership.it.maas.entity;

import com.fasterxml.jackson.annotation.JsonRootName;
import lombok.Data;

@Data
@JsonRootName(value = "error")
public class TmfErrorResponse {
    private String id;
    private String status;
    private String code;
    private String message;
    private String details;
}
