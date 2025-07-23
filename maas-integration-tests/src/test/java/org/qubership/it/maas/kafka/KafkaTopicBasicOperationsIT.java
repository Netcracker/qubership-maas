package org.qubership.it.maas.kafka;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import okhttp3.Response;
import org.apache.commons.lang3.StringUtils;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;
import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.bg2.BGState;
import org.qubership.it.maas.bg2.BgNamespace;
import org.qubership.it.maas.bg2.BgNamespaces;
import org.qubership.it.maas.entity.ConfigV2Resp;
import org.qubership.it.maas.entity.SearchCriteria;
import org.qubership.it.maas.entity.TmfErrorResponse;
import org.qubership.it.maas.entity.kafka.*;

import java.io.IOException;
import java.util.*;
import java.util.concurrent.ExecutionException;

import static org.apache.http.HttpStatus.SC_NOT_FOUND;
import static org.apache.http.HttpStatus.SC_OK;
import static org.junit.jupiter.api.Assertions.*;
import static org.junit.jupiter.api.Assumptions.assumeTrue;
import static org.qubership.it.maas.MaasITHelper.*;


@Slf4j
class KafkaTopicBasicOperationsIT extends AbstractMaasWithInitsIT {

    @Test
    public void checkCreatingTopic() throws IOException {
        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, createSimpleClassifier("create-topic-test-" + UUID.randomUUID()));
        assertTrue(topic.getName().startsWith("maas." + TEST_NAMESPACE));
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertTrue(topic.getName().contains("create-topic-test-"));
        assertTrue(topic.getName().contains(TEST_NAMESPACE));
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());
    }

    @Test
    public void checkCreatingTopic_WithExternalManaged() throws IOException {
        KafkaTopicRequest kafkaTopicRequest = KafkaTopicRequest.builder()
                .classifier(
                        createSimpleClassifier("KafkaTopicBasicOperationsIT", "it-test")
                )
                .externallyManaged(true)
                .name("KafkaTopicBasicOperationsIT-externally-managed")
                .build();
        createKafkaTopic(HttpStatus.SC_GONE, kafkaTopicRequest, TmfErrorResponse.class);
    }

    @Test
    public void checkCreatingTopic_WithExternalManagedAndEmptyName() throws IOException {
        KafkaTopicRequest kafkaTopicRequest = KafkaTopicRequest.builder()
                .classifier(
                        createSimpleClassifier("KafkaTopicBasicOperationsIT", "it-test")
                )
                .externallyManaged(true)
                .build();
        createKafkaTopic(HttpStatus.SC_BAD_REQUEST, kafkaTopicRequest, TmfErrorResponse.class);
    }

    @Test
    public void topicWithReplicationFactorDefaults() throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("KafkaTopicBasicOperationsIT", "it-test");
        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, KafkaTopicRequest.builder().classifier(classifier).replicationFactor("inherit").build());
        assertTrue(topic.getName().startsWith("maas." + TEST_NAMESPACE));
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertTrue(topic.getName().contains("KafkaTopicBasicOperationsIT"));
        assertTrue(topic.getName().contains("it-test"));
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());
    }

    @Test
    public void topicWithTheSpecifiedName() throws IOException {
        final String name = "KafkaTopicBasicOperationsIT." + UUID.randomUUID().toString();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .name(name)
                .classifier(createSimpleClassifier(name))
                .build();

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, request);
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertEquals(name, topic.getName());
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());

        topic = createKafkaTopic(HttpStatus.SC_OK, request);
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertEquals(name, topic.getName());
    }

    @Test
    public void alreadyCreatedTopic() throws IOException {
        KafkaTopicResponse newTopic = createKafkaTopic(HttpStatus.SC_CREATED);

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_OK); // http status should be 200
        assertEquals(newTopic, topic);
        assertTrue(topic.getName().startsWith("maas." + TEST_NAMESPACE));
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertTrue(topic.getName().contains("KafkaTopicBasicOperationsIT"));
        assertTrue(topic.getName().contains("it-test"));
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());
    }

    @Test
    public void getTopicByClassifier() throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("KafkaTopicBasicOperationsIT", "it-test");
        KafkaTopicResponse newTopic = createKafkaTopic(HttpStatus.SC_CREATED, classifier);

        KafkaTopicResponse topic = getKafkaTopicByClassifier(HttpStatus.SC_OK, classifier);
        assertEquals(newTopic, topic);

        classifier = createSimpleClassifier("anotherTestTopic");
        getKafkaTopicByClassifier(HttpStatus.SC_NOT_FOUND, classifier);
    }

    @Test
    public void updateTopicSettings_Configs() throws IOException {
        final String name = "KafkaTopicBasicOperationsIT." + UUID.randomUUID().toString();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .classifier(createSimpleClassifier(name))
                .numPartitions(1)
                .configs(Collections.singletonMap("flush.ms", "1000"))
                .build();

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, request);
        log.info("Result of topic creation: {}", topic);
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());
        assertNotNull(topic.getRequestedSettings());
        assertEquals(Collections.singletonMap("flush.ms", "1000"), topic.getRequestedSettings().getConfigs());
        assertEquals(Integer.valueOf(1), topic.getRequestedSettings().getNumPartitions());
        assertNotNull(topic.getActualSettings());
        assertEquals("1000", topic.getActualSettings().getConfigs().get("flush.ms"));
        assertEquals(Integer.valueOf(1), topic.getActualSettings().getNumPartitions());

        request.setConfigs(topic.getActualSettings().getConfigs());
        request.getConfigs().put("flush.ms", "10000");

        log.info("Checking topic with getKafkaTopicByClassifier");
        KafkaTopicResponse checkTopic = getKafkaTopicByClassifier(HttpStatus.SC_OK, createSimpleClassifier(name));
        assertEquals(topic, checkTopic);
        log.info("Checked topic with getKafkaTopicByClassifier successfully");

        topic = createKafkaTopic(HttpStatus.SC_OK, request);
        log.info("Result of topic update: {}", topic);
        assertNotNull(topic.getRequestedSettings());
        assertEquals("10000", topic.getRequestedSettings().getConfigs().get("flush.ms"));
        assertEquals(topic.getRequestedSettings().getConfigs(), topic.getActualSettings().getConfigs());
        assertNotNull(topic.getActualSettings());
        assertEquals("10000", topic.getActualSettings().getConfigs().get("flush.ms"));

        request.setConfigs(topic.getActualSettings().getConfigs());
        request.getConfigs().put("fake.config.name", "10000");

        createKafkaTopic(HttpStatus.SC_INTERNAL_SERVER_ERROR, request);
    }

    @Test
    public void updateTopicSettings_errorOnInstanceUpdate() throws IOException {
        KafkaInstance[] kafkaInstances = getKafkaInstances();
        assumeTrue(kafkaInstances.length >= 2, "kafkaInstanceDesignator test requires at least two instances. Skip test");

        KafkaInstance firstInstance = kafkaInstances[0];
        KafkaInstance secondInstance = kafkaInstances[1];

        final String name = "KafkaTopicBasicOperationsIT." + UUID.randomUUID();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .classifier(createSimpleClassifier(name))
                .numPartitions(1)
                .instance(firstInstance.getId())
                .build();

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, request);
        log.info("Result of topic creation: {}", topic);
        assertEquals(firstInstance.getId(), topic.getInstance());

        request.setInstance(secondInstance.getId());

        createKafkaTopic(HttpStatus.SC_BAD_REQUEST, request, TmfErrorResponse.class);
    }

    @Test
    public void deleteTopic() throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("KafkaTopicBasicOperationsIT", "it-test");
        SearchCriteria searchForm = SearchCriteria.builder().classifier(classifier).build();
        KafkaTopicResponse createdTopic = createKafkaTopic(HttpStatus.SC_CREATED, classifier);
        log.info("Deleting kafka topics by search criteria: {}", searchForm);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_PATH, getMaasBasicAuth(), searchForm, "DELETE");
        try (Response response = helper.okHttpClient.newCall(request).execute()) {
            log.info("Topics deletion response: {}", response);
            ObjectMapper objectMapper = new ObjectMapper();
            objectMapper.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
            KafkaTopicDeletionResponse topicDeletionResponse = objectMapper.readValue(response.body().string(), KafkaTopicDeletionResponse.class);
            switch (response.code()) {
                case HttpStatus.SC_OK:
                    assertTrue(topicDeletionResponse.getFailedToDelete() == null || topicDeletionResponse.getFailedToDelete().isEmpty());
                    assertEquals(1, topicDeletionResponse.getDeletedSuccessfully().size());
                    assertEquals(createdTopic, topicDeletionResponse.getDeletedSuccessfully().get(0));
                    break;
                case HttpStatus.SC_METHOD_NOT_ALLOWED:
                    assertTrue(topicDeletionResponse.getDeletedSuccessfully() == null || topicDeletionResponse.getDeletedSuccessfully().isEmpty());
                    assertEquals(1, topicDeletionResponse.getFailedToDelete().size());
                    break;
                default:
                    fail("Unexpected code: " + response);
            }
        }

    }

    @Test
    public void deleteTopic_NoTopicsHaveToBeDeleted() throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("KafkaTopicBasicOperationsITNoTopics", "it-test");
        KafkaTopicSearchRequest searchRequest = KafkaTopicSearchRequest.builder()
                .classifier(classifier)
                .build();
        KafkaTopicDeletionResponse response = deleteKafkaTopic(searchRequest, HttpStatus.SC_OK);
        assertTrue(response.getDeletedSuccessfully() == null || response.getDeletedSuccessfully().isEmpty());
        assertTrue(response.getFailedToDelete() == null || response.getFailedToDelete().isEmpty());
    }

    @Test
    public void bulkDeleteTopics() throws IOException {
        Map<String, Object> classifier1 = createSimpleClassifier("KafkaTopicBasicOperationsIT-1", "it-test");
        Map<String, Object> classifier2 = createSimpleClassifier("KafkaTopicBasicOperationsIT-2", "it-test");
        KafkaTopicResponse createdTopic1 = createKafkaTopic(HttpStatus.SC_CREATED, classifier1);
        KafkaTopicResponse createdTopic2 = createKafkaTopic(HttpStatus.SC_CREATED, classifier2);

        Map<String, String> searchForm = Collections.singletonMap("namespace", TEST_NAMESPACE);
        log.info("Deleting kafka topics by search criteria: {}", searchForm);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_PATH, getMaasBasicAuth(), searchForm, "DELETE");
        try (Response response = helper.okHttpClient.newCall(request).execute()) {
            log.info("Topics deletion response: {}", response);
            ObjectMapper objectMapper = new ObjectMapper();
            objectMapper.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
            KafkaTopicDeletionResponse topicDeletionResponse = objectMapper.readValue(response.body().string(), KafkaTopicDeletionResponse.class);
            switch (response.code()) {
                case HttpStatus.SC_OK:
                    assertTrue(topicDeletionResponse.getFailedToDelete() == null || topicDeletionResponse.getFailedToDelete().isEmpty());
                    assertEquals(2, topicDeletionResponse.getDeletedSuccessfully().size());
                    assertTrue(topicDeletionResponse.getDeletedSuccessfully().contains(createdTopic1));
                    assertTrue(topicDeletionResponse.getDeletedSuccessfully().contains(createdTopic2));
                    break;
                case HttpStatus.SC_METHOD_NOT_ALLOWED:
                    assertTrue(topicDeletionResponse.getDeletedSuccessfully() == null || topicDeletionResponse.getDeletedSuccessfully().isEmpty());
                    assertEquals(2, topicDeletionResponse.getFailedToDelete().size());
                    break;
                default:
                    fail("Unexpected result: " + response);
            }
        }
    }

    @Test
    public void bulkDeleteTopics_NoTopicsHaveToBeDeleted() throws IOException {
        KafkaTopicDeletionResponse response = deleteKafkaTopics(TEST_NAMESPACE_WITHOUT_TOPICS, HttpStatus.SC_OK);
        assertTrue(response.getDeletedSuccessfully() == null || response.getDeletedSuccessfully().isEmpty());
        assertTrue(response.getFailedToDelete() == null || response.getFailedToDelete().isEmpty());
    }

    @Test
    public void searchTopics() throws IOException {
        Map<String, Object> byNamespace = Collections.singletonMap("namespace", TEST_NAMESPACE);
        List<KafkaTopicResponse> response = searchKafkaTopics(byNamespace, HttpStatus.SC_OK);
        assertTrue(response == null || response.isEmpty());

        Map<String, Object> classifier1 = createSimpleClassifier("KafkaTopicBasicOperationsIT-1");
        Map<String, Object> classifier2 = createSimpleClassifier("KafkaTopicBasicOperationsIT-2");
        Map<String, Object> classifier3 = createSimpleClassifier("KafkaTopicBasicOperationsIT-3");
        KafkaTopicResponse createdTopic1 = createKafkaTopic(HttpStatus.SC_CREATED, classifier1);
        KafkaTopicResponse createdTopic2 = createKafkaTopic(HttpStatus.SC_CREATED, classifier2);
        KafkaTopicResponse createdTopic3 = createKafkaTopic(HttpStatus.SC_CREATED, classifier3);
        response = searchKafkaTopics(byNamespace, HttpStatus.SC_OK);
        assertEquals(3, response.size());
        assertTrue(response.contains(createdTopic1));
        assertTrue(response.contains(createdTopic2));
        assertTrue(response.contains(createdTopic3));

        Map<String, Object> byClassifier = Collections.singletonMap("classifier", classifier1);
        response = searchKafkaTopics(byClassifier, HttpStatus.SC_OK);
        assertEquals(1, response.size());
        assertTrue(response.contains(createdTopic1));

        byClassifier = Collections.singletonMap("classifier", classifier2);
        response = searchKafkaTopics(byClassifier, HttpStatus.SC_OK);
        assertEquals(1, response.size());
        assertTrue(response.contains(createdTopic2));

        Map<String, Object> byTopic = Collections.singletonMap("topic", createdTopic3.getName());
        response = searchKafkaTopics(byTopic, HttpStatus.SC_OK);
        assertEquals(1, response.size());
        assertTrue(response.contains(createdTopic3));

        Map<String, Object> byInstance = Collections.singletonMap("instance", createdTopic1.getInstance());
        response = searchKafkaTopics(byInstance, HttpStatus.SC_OK);
        assertTrue(response.contains(createdTopic1));
        assertTrue(response.contains(createdTopic2));
        assertTrue(response.contains(createdTopic3));

        Map<String, Object> byTopicAndInstance = new HashMap<>(2);
        byTopicAndInstance.put("topic", createdTopic1.getName());
        byTopicAndInstance.put("instance", createdTopic1.getInstance());
        response = searchKafkaTopics(byTopicAndInstance, HttpStatus.SC_OK);
        assertEquals(1, response.size());
        assertTrue(response.contains(createdTopic1));
        assertNotNull(response.get(0).getActualSettings());
        assertNotNull(response.get(0).getActualSettings().getNumPartitions());
        assertNotNull(response.get(0).getActualSettings().getReplicaAssignment());
        assertFalse(response.get(0).getActualSettings().getReplicaAssignment().isEmpty());
        assertNotNull(response.get(0).getActualSettings().getConfigs());
        assertFalse(response.get(0).getActualSettings().getConfigs().isEmpty());
    }

    @Test
    public void registerExternallyManagedTopic() throws IOException {
        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "There is no default kafka or incorrect auth mechanism. Skip tests");
        createKafkaTopic(kafkaProp);
        KafkaTopicRequest kafkaTopicRequest = KafkaTopicRequest.builder()
                .classifier(
                        createSimpleClassifier(TOPIC_NAME, "it-test")
                )
                .externallyManaged(true)
                .name(TOPIC_NAME)
                .build();

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, kafkaTopicRequest);
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertTrue(topic.getName().contains(TOPIC_NAME));
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());
        deleteKafkaTopic(kafkaProp);
    }

    @Test
    public void registerAndDeleteExternallyManagedTopic() throws IOException {
        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "There is no default kafka or incorrect auth mechanism. Skip tests");
        createKafkaTopic(kafkaProp);
        KafkaTopicRequest kafkaTopicRequest = KafkaTopicRequest.builder()
                .classifier(
                        createSimpleClassifier(TOPIC_NAME, "it-test")
                )
                .externallyManaged(true)
                .name(TOPIC_NAME)
                .build();

        createKafkaTopic(HttpStatus.SC_CREATED, kafkaTopicRequest);
        Map<String, Object> classifier = createSimpleClassifier(TOPIC_NAME, "it-test");
        KafkaTopicSearchRequest searchRequest = KafkaTopicSearchRequest.builder()
                .classifier(classifier)
                .build();
        KafkaTopicDeletionResponse response = deleteKafkaTopic(searchRequest, HttpStatus.SC_OK);
        assertTrue(response.getFailedToDelete() == null || response.getFailedToDelete().isEmpty());
        assertEquals(1, response.getDeletedSuccessfully().size());
        deleteKafkaTopic(kafkaProp);
    }

    @Test
    public void registerExternallyManagedTopic_WithDeletionBeforeRegistration() throws IOException {
        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "There is no default kafka or incorrect auth mechanism. Skip tests");
        createKafkaTopic(kafkaProp);
        KafkaTopicRequest kafkaTopicRequest = KafkaTopicRequest.builder()
                .classifier(
                        createSimpleClassifier(TOPIC_NAME, "it-test")
                )
                .externallyManaged(true)
                .name(TOPIC_NAME)
                .build();

        createKafkaTopic(HttpStatus.SC_CREATED, kafkaTopicRequest);
        deleteKafkaTopic(kafkaProp);
        createKafkaTopic(HttpStatus.SC_GONE, kafkaTopicRequest, TmfErrorResponse.class);
    }

    @Test
    void kafkaInstanceDesignatorWildcard() throws Exception {
        KafkaInstance[] kafkaInstances = getKafkaInstances();
        assumeTrue(kafkaInstances.length >= 2, "kafkaInstanceDesignator test requires at least two instances. Skip test");

        KafkaInstance firstInstance = kafkaInstances[0];
        KafkaInstance secondInstance = kafkaInstances[1];

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec: \n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  shared: |\n" +
                        "    apiVersion: nc.maas.kafka/v2\n" +
                        "    kind: instance-designator\n" +
                        "    spec:\n" +
                        "        namespace: " + TEST_NAMESPACE + "\n" +
                        "        defaultInstance: " + firstInstance.getId() + "\n" +
                        "        selectors:\n" +
                        "        - classifierMatch:\n" +
                        "            name: prefixed-*\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            name: n-?\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "  services: \n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "          apiVersion: nc.maas.kafka/v1\n" +
                        "          kind: topic\n" +
                        "          spec:\n" +
                        "             classifier: { name: orders, namespace: " + TEST_NAMESPACE + " }\n" +
                        "             "
        ;

        ConfigV2Resp reply;
        reply = applyConfigV2(200, cfg);

        assertEquals("ok", reply.getStatus());

        //check Kafka
        Map<String, Object> classifierKafka = createSimpleClassifier("prefixed-first", "", TEST_NAMESPACE);
        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, classifierKafka);
        assertEquals(topic.getClassifier().get("name"), classifierKafka.get("name"));
        assertEquals(secondInstance.getId(), topic.getInstance());

        Map<String, Object> classifierKafkaTenant = createSimpleClassifier("n-1", "", TEST_NAMESPACE);
        KafkaTopicResponse topic2 = createKafkaTopic(HttpStatus.SC_CREATED, classifierKafkaTenant);
        assertEquals(topic2.getClassifier().get("name"), classifierKafkaTenant.get("name"));
        assertEquals(secondInstance.getId(), topic2.getInstance());

        deleteKafkaInstanceDesignator(HttpStatus.SC_OK);
    }

    @Test
    void checkCreatingVersionedTopic_Warmup_Commit() throws IOException, ExecutionException, InterruptedException {
        var kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "There is no default kafka or incorrect auth mechanism. Skip tests");

        deleteNamespace(TEST_NAMESPACE);
        deleteNamespace(TEST_NAMESPACE_2_BG);
        var request = helper.createJsonRequest(BG2_DESTROY_DOMAIN, getMaasBasicAuth(), new BgNamespaces(TEST_NAMESPACE, TEST_NAMESPACE_2_BG, ""), "DELETE");
        helper.doRequest(request, SC_OK, SC_NOT_FOUND);

        var firstTopicName = "{{namespace}}-test-topic-" + UUID.randomUUID();
        var secondTopicName = "{{namespace}}-test-topic-" + UUID.randomUUID();
        var thirdTopicName = "{{namespace}}-test-topic-" + UUID.randomUUID();

        createKafkaTopic(HttpStatus.SC_CREATED, KafkaTopicRequest.builder()
                .name(firstTopicName)
                .classifier(Map.of("name", firstTopicName,
                        "namespace", TEST_NAMESPACE))
                .versioned(false)
                .build()
        );

        createKafkaTopic(HttpStatus.SC_CREATED, KafkaTopicRequest.builder()
                .name(secondTopicName)
                .classifier(Map.of("name", secondTopicName,
                        "namespace", TEST_NAMESPACE))
                .versioned(true)
                .build()
        );

        createKafkaTopic(HttpStatus.SC_CREATED, KafkaTopicRequest.builder()
                .name(thirdTopicName)
                .classifier(Map.of("name", thirdTopicName,
                        "namespace", TEST_NAMESPACE))
                .versioned(true)
                .build()
        );

        Set<String> allKafkaTopicsNames = getAllKafkaTopicsNames(kafkaProp);

        log.info("allKafkaTopicsNames after creation: {}", String.join(", ", allKafkaTopicsNames));

        assertTrue(allKafkaTopicsNames.contains(firstTopicName.replace("{{namespace}}", TEST_NAMESPACE)));
        assertTrue(allKafkaTopicsNames.contains(secondTopicName.replace("{{namespace}}", TEST_NAMESPACE)));
        assertTrue(allKafkaTopicsNames.contains(thirdTopicName.replace("{{namespace}}", TEST_NAMESPACE)));

        assertFalse(allKafkaTopicsNames.contains(firstTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG)));
        assertFalse(allKafkaTopicsNames.contains(secondTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG)));
        assertFalse(allKafkaTopicsNames.contains(thirdTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG)));

        var bgNamespacesPair = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "candidate", "v2"),
                new Date()
        );

        request = helper.createJsonRequest(BG2_INIT_DOMAIN, getMaasBasicAuth(), bgNamespacesPair, "POST");
        helper.doRequest(request, SC_OK);

        request = helper.createJsonRequest(BG2_WARMUP, getMaasBasicAuth(), bgNamespacesPair, "POST");
        helper.doRequest(request, SC_OK);

        allKafkaTopicsNames = getAllKafkaTopicsNames(kafkaProp);

        log.info("allKafkaTopicsNames after warmup: {}", String.join(", ", allKafkaTopicsNames));

        assertTrue(allKafkaTopicsNames.contains(firstTopicName.replace("{{namespace}}", TEST_NAMESPACE)));
        assertTrue(allKafkaTopicsNames.contains(secondTopicName.replace("{{namespace}}", TEST_NAMESPACE)));
        assertTrue(allKafkaTopicsNames.contains(thirdTopicName.replace("{{namespace}}", TEST_NAMESPACE)));

        assertTrue(allKafkaTopicsNames.contains(secondTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG)));
        assertTrue(allKafkaTopicsNames.contains(thirdTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG)));

        bgNamespacesPair = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "idle", null),
                new Date()
        );
        request = helper.createJsonRequest(BG2_COMMIT, getMaasBasicAuth(), bgNamespacesPair, "POST");
        helper.doRequest(request, SC_OK);

        int retries = 10;
        for (int retry = 0; retry <= retries; retry++) {
            allKafkaTopicsNames = getAllKafkaTopicsNames(kafkaProp);

            log.info("allKafkaTopicsNames after commit: {}", String.join(", ", allKafkaTopicsNames));

            if (allKafkaTopicsNames.contains(firstTopicName.replace("{{namespace}}", TEST_NAMESPACE)) &&
                    allKafkaTopicsNames.contains(secondTopicName.replace("{{namespace}}", TEST_NAMESPACE)) &&
                    allKafkaTopicsNames.contains(thirdTopicName.replace("{{namespace}}", TEST_NAMESPACE)) &&
                    !allKafkaTopicsNames.contains(secondTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG)) &&
                    !allKafkaTopicsNames.contains(thirdTopicName.replace("{{namespace}}", TEST_NAMESPACE_2_BG))) {
                break;
            }

            if (retry != retries) {
                log.warn("Retrying to get allKafkaTopicsNames...");
                Thread.sleep(1000);
            } else {
                fail("Assertions failed after retries");
            }
        }
    }
}
