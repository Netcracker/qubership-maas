package com.netcracker.it.maas.kafka;

import com.netcracker.it.maas.AbstractMaasWithInitsIT;
import com.netcracker.it.maas.entity.TmfErrorResponse;
import com.netcracker.it.maas.entity.kafka.KafkaInstance;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.List;
import java.util.Map;

import static com.netcracker.it.maas.MaasITHelper.*;
import static org.junit.jupiter.api.Assertions.assertNotEquals;

class InstanceOperationsIT extends AbstractMaasWithInitsIT {

    @Test
    void addWrongKafkaInstance() throws IOException {
        KafkaInstance[] kafkaInstances = getKafkaInstances();
        assertNotEquals(0, kafkaInstances.length);

        KafkaInstance kafkaInstance = kafkaInstances[0];
        kafkaInstance.setId("wrong-kafka-instance");
        kafkaInstance.setAddresses(Map.of("PLAINTEXT", List.of("wrong:9092")));
        Request request = helper.createJsonRequest(KAFKA_INSTANCE_PATH, getMaasBasicAuth(), kafkaInstance, POST);
        helper.doRequest(request, TmfErrorResponse.class, HttpStatus.SC_BAD_REQUEST);
    }

    @Test
    void updateWrongKafkaInstance() throws IOException {
        KafkaInstance[] kafkaInstances = getKafkaInstances();
        assertNotEquals(0, kafkaInstances.length);

        KafkaInstance kafkaInstance = kafkaInstances[0];
        kafkaInstance.setId("wrong-kafka-instance");
        kafkaInstance.setAddresses(Map.of("PLAINTEXT", List.of("wrong:9092")));
        Request request = helper.createJsonRequest(KAFKA_INSTANCE_PATH, getMaasBasicAuth(), kafkaInstance, PUT);
        helper.doRequest(request, TmfErrorResponse.class, HttpStatus.SC_BAD_REQUEST);
    }
}
