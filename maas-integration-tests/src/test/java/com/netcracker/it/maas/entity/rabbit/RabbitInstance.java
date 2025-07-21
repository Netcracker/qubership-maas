package com.netcracker.it.maas.entity.rabbit;

import com.fasterxml.jackson.annotation.JsonProperty;
import com.google.gson.annotations.SerializedName;
import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;

@Data
@NoArgsConstructor
@AllArgsConstructor
public class RabbitInstance {
    private String id;
    private String ApiUrl;
    private String AmqpUrl;
    private String User;
    private String Password;

    @SerializedName("default")
    @JsonProperty("default")
    private Boolean isDefault;
}
