package com.netcracker.it.maas.rabbit;

import com.netcracker.it.maas.entity.TmfErrorResponse;
import com.netcracker.it.maas.entity.rabbit.RabbitInstance;
import okhttp3.Request;
import org.junit.jupiter.api.Test;

import java.io.IOException;

import static com.netcracker.it.maas.MaasITHelper.*;
import static org.apache.http.HttpStatus.SC_BAD_REQUEST;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

class InstanceOperationsIT extends RabbitTest {

    @Test
    void addWrongRabbitInstance() throws IOException {
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assertNotEquals(0, rabbitInstances.length);

        RabbitInstance rabbitInstance = rabbitInstances[0];
        rabbitInstance.setId("wrong-rabbit-instance");
        rabbitInstance.setApiUrl("http://wrong:15432/api");
        rabbitInstance.setAmqpUrl("amqp://wrong:5432");
        Request request = helper.createJsonRequest(RABBIT_INSTANCE_PATH, getMaasBasicAuth(), rabbitInstance, POST);
        helper.doRequest(request, TmfErrorResponse.class, SC_BAD_REQUEST);
    }

    @Test
    void updateWrongRabbitInstance() throws IOException {
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assertNotEquals(0, rabbitInstances.length);

        RabbitInstance rabbitInstance = rabbitInstances[0];
        rabbitInstance.setId("wrong-rabbit-instance");
        rabbitInstance.setApiUrl("http://wrong:15432/api");
        rabbitInstance.setAmqpUrl("amqp://wrong:5432");
        Request request = helper.createJsonRequest(RABBIT_INSTANCE_PATH, getMaasBasicAuth(), rabbitInstance, PUT);
        helper.doRequest(request, TmfErrorResponse.class, SC_BAD_REQUEST);
    }
}
