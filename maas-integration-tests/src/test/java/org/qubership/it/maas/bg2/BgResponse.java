package org.qubership.it.maas.bg2;

import lombok.Data;

@Data
public class BgResponse {

    private String status;
    private String message;
    private String operationDetails;

    public BgResponse(String status, String message, String operationDetails) {
        this.status = status;
        this.message = message;
        this.operationDetails = operationDetails;
    }

    public BgResponse() {
    }
}
