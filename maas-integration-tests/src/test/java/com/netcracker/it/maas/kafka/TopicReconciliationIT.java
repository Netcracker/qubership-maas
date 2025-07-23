package com.netcracker.it.maas.kafka;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.netcracker.it.maas.AbstractMaasWithInitsIT;
import com.netcracker.it.maas.MaasITHelper;
import com.netcracker.it.maas.entity.kafka.KafkaInstance;
import com.netcracker.it.maas.entity.kafka.KafkaTopicRequest;
import com.netcracker.it.maas.entity.kafka.KafkaTopicSearchRequest;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Arrays;
import java.util.Comparator;
import java.util.List;
import java.util.Map;

import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

class TopicReconciliationIT extends AbstractMaasWithInitsIT {

    private static final String FIRST_TOPIC_NAME = "first-test-topic";
    private static final String SECOND_TOPIC_NAME = "second-test-topic";

    @Test
    void topicReconciliation() throws Exception {
        KafkaInstance[] kafkaInstances = getKafkaInstances();
        assumeTrue(kafkaInstances.length >= 2, "TopicReconciliation test requires at least two instances. Skip test");

        deleteKafkaTopic(KafkaTopicSearchRequest.builder()
                .classifier(createSimpleClassifier(FIRST_TOPIC_NAME, null))
                .build(), HttpStatus.SC_OK);

        deleteKafkaTopic(KafkaTopicSearchRequest.builder()
                .classifier(createSimpleClassifier(SECOND_TOPIC_NAME, null))
                .build(), HttpStatus.SC_OK);
        Arrays.sort(kafkaInstances, Comparator.comparing(KafkaInstance::getIsDefault));

        KafkaInstance firstKafkaInstance = kafkaInstances[0];
        KafkaInstance secondKafkaInstance = kafkaInstances[1];
        deleteKafkaInstance(firstKafkaInstance, HttpStatus.SC_OK);

        Map<String, Object> firstTopicClassifier = createSimpleClassifier(FIRST_TOPIC_NAME, null);
        KafkaTopicRequest firstTopicKafkaTopicRequest = KafkaTopicRequest.builder()
                .name(FIRST_TOPIC_NAME)
                .instance(secondKafkaInstance.getId())
                .classifier(firstTopicClassifier)
                .build();
        createKafkaTopic(HttpStatus.SC_CREATED, firstTopicKafkaTopicRequest);

        Map<String, Object> secondTopicClassifier = createSimpleClassifier(SECOND_TOPIC_NAME, null);
        KafkaTopicRequest secondTopicKafkaTopicRequest = KafkaTopicRequest.builder()
                .name(SECOND_TOPIC_NAME)
                .instance(secondKafkaInstance.getId())
                .classifier(secondTopicClassifier)
                .build();
        createKafkaTopic(HttpStatus.SC_CREATED, secondTopicKafkaTopicRequest);

        String firstKafkaAddr = firstKafkaInstance.getAddresses().get("PLAINTEXT").get(0);
        String secondKafkaAddr = secondKafkaInstance.getAddresses().get("PLAINTEXT").get(0);
        secondKafkaInstance.getAddresses().get("PLAINTEXT").set(0, firstKafkaAddr);
        updateKafkaInstance(secondKafkaInstance);

        List<Map<String, Object>> result = startReconciliation();
        assertEquals(2, result.size());

        Map<String, Object> first = getByName(result, FIRST_TOPIC_NAME);
        assertEquals(FIRST_TOPIC_NAME, first.get("name"));
        assertEquals("added", first.get("status"));

        Map<String, Object> second = getByName(result, SECOND_TOPIC_NAME);
        assertEquals(SECOND_TOPIC_NAME, second.get("name"));
        assertEquals("added", second.get("status"));

        // TODO ADD KAFKA TOPIC CHECKS ON BOTH INSTANCES
        firstTopicKafkaTopicRequest.setConfigs(Map.of("flush.ms", "10000"));
        createKafkaTopic(HttpStatus.SC_OK, firstTopicKafkaTopicRequest);

        // rollback
        deleteKafkaTopic(KafkaTopicSearchRequest.builder()
                .classifier(firstTopicClassifier)
                .build(), HttpStatus.SC_OK);
        deleteKafkaTopic(KafkaTopicSearchRequest.builder()
                .classifier(secondTopicClassifier)
                .build(), HttpStatus.SC_OK);
        secondKafkaInstance.getAddresses().get("PLAINTEXT").set(0, secondKafkaAddr);
        updateKafkaInstance(secondKafkaInstance);
        createKafkaInstance(firstKafkaInstance);
        deleteKafkaTopic(preparePropertiesAndPortForwardKafka(secondKafkaInstance), FIRST_TOPIC_NAME);
        deleteKafkaTopic(preparePropertiesAndPortForwardKafka(secondKafkaInstance), SECOND_TOPIC_NAME);
    }

    private List<Map<String, Object>> startReconciliation() throws IOException {
        Request request = helper.createJsonRequest(MaasITHelper.KAFKA_RECOVERY_PATH + "/" + TEST_NAMESPACE, getMaasBasicAuth(), "dummy", MaasITHelper.POST);
        String resultJson = helper.doRequestWithStringResponse(request, 200);
        ObjectMapper objectMapper = new ObjectMapper();
        return objectMapper.readValue(resultJson, new TypeReference<>() {
        });
    }

    private Map<String, Object> getByName(List<Map<String, Object>> list, String name) {
        return list.stream()
                .filter(v -> name.equals(v.get("name")))
                .findFirst()
                .orElse(null);
    }
}
