package org.qubership.it.maas;

import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.google.common.collect.Lists;
import com.google.common.net.MediaType;
import com.google.gson.GsonBuilder;
import com.rabbitmq.client.Connection;
import com.rabbitmq.client.ConnectionFactory;
import lombok.extern.slf4j.Slf4j;
import net.jodah.failsafe.Failsafe;
import net.jodah.failsafe.RetryPolicy;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.apache.commons.lang3.StringUtils;
import org.apache.http.HttpStatus;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.DescribeTopicsResult;
import org.apache.kafka.clients.admin.NewTopic;
import org.apache.kafka.clients.admin.TopicDescription;
import org.qubership.it.maas.entity.Account;
import org.qubership.it.maas.entity.ConfigV2Req;
import org.qubership.it.maas.entity.ConfigV2Resp;
import org.qubership.it.maas.entity.SearchCriteria;
import org.qubership.it.maas.entity.kafka.*;
import org.qubership.it.maas.entity.rabbit.*;
import org.yaml.snakeyaml.Yaml;

import java.io.IOException;
import java.net.URI;
import java.time.Duration;
import java.time.temporal.ChronoUnit;
import java.util.*;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.regex.Pattern;
import java.util.stream.Collectors;

import static org.junit.jupiter.api.Assertions.*;
import static org.qubership.it.maas.AbstractMaasWithInitsIT.*;
import static org.qubership.it.maas.MaasITHelper.*;

@Slf4j
public abstract class AbstractMaasIT {

    public static OkHttpClient okHttpClient = new OkHttpClient.Builder()
            .readTimeout(2, TimeUnit.MINUTES)
            .build();
    public static final MediaType JSON = MediaType.parse("application/json; charset=utf-8");
    private static final String NAME = "name";
    private static final String TENANT_ID = "tenantId";
    protected static final String TOPIC_NAME = "KafkaTopicBasicOperationsIT";
    protected static final String MAAS_SERVICE_NAME = "maas-service";
    public static String NAMESPACE = "namespace";
    public static String MAAS_MANAGER_ACCOUNT_AUTH_STR = getBasicAuthStr("manager", "manager");
    public static Account agentAccount = new Account("MaasAgentAccountIT", "MaasAgentIT_password",
            new ArrayList<>() {{
                add("agent");
                add("manager");
                add("bgoperator");
            }});

    public static final String API_RABBIT_VERSION = "nc.maas.rabbit/v1";
    public static final String API_RABBIT_VERSION_V2 = "nc.maas.rabbit/v2";
    public static final String KIND_VHOST = "vhost";
    public static final String DEFAULT_RABBIT_INSTANCE = "";

    public static final String API_KAFKA_VERSION = "nc.maas.kafka/v1";
    public static final String KIND_KAFKA = "topic";

    //todo remove in future
    public static final String KAFKA_FAKE_INSTANCE_ID = "fake-kafka-service";
    public static final String KAFKA_FAKE_SERVICE_NAME = "kafka-fake-service";

    private Pattern kafkaServicePattern = Pattern.compile("^kafka-\\d");

    private static final String KIND_KAFKA_TEMPLATE = "topic-template";

    private static final String STATUS_ERROR = "error";
    private static final String STATUS_OK = "ok";

    public static final RetryPolicy<Object> DEFAULT_RETRY_POLICY = new RetryPolicy<>()
            .handle(AssertionError.class, Exception.class)
            .withMaxRetries(5)
            .withDelay(Duration.ofSeconds(3))
            .onRetry(e -> {
                Throwable lastFailure = e.getLastFailure();
                log.info("Retry #{} after: {} ms, lastFailure: {}",
                        e.getAttemptCount(), e.getElapsedAttemptTime().toMillis(),
                        lastFailure.getClass().getSimpleName() + " - " + lastFailure.getMessage());
            });

    protected static MaasITHelper helper;

    protected enum ApplyConfigOperation {
        CREATE,
        DELETE,
        CREATE_AND_DELETE,
        VERSIONED
    }

    protected static void initHelper() {
        if (helper == null) {
            helper = new MaasITHelper(getMaasContainerAddress());
        }
    }

    protected static void resetHelper() {
        helper = new MaasITHelper(getMaasContainerAddress());
    }

    public static String getBasicAuthStr(String user, String password) {
        return Base64.getEncoder().encodeToString((user + ":" + password).getBytes());
    }

    public void initAccount(Account account, String namespace) {
        account.setNamespace(namespace);
        String initAccountRequest = new GsonBuilder().create().toJson(account);
        try (Response response = okHttpClient.newCall(
                new Request.Builder()
                        .url(getMaasContainerAddress() + "api/v1/auth/account/client")
                        .addHeader("Authorization", "Basic " + MAAS_MANAGER_ACCOUNT_AUTH_STR)
                        .addHeader("X-Origin-Namespace", account.getNamespace())
                        .post(RequestBody.create(initAccountRequest.getBytes()))
                        .build()).execute()) {
            log.info("Response: {}", response);
            assertTrue(response.code() == 200 || response.code() == 201);
        } catch (Exception e) {
            log.error("Failed to create client account", e);
            fail("Failed to create client account", e);
        }
    }

    public Response createAccount(Account account) throws IOException {
        String initAccountRequest = new GsonBuilder().create().toJson(account);
        return okHttpClient.newCall(
                new Request.Builder()
                        .url(getMaasContainerAddress() + "api/v1/auth/account/client")
                        .addHeader("Authorization", "Basic " + MAAS_MANAGER_ACCOUNT_AUTH_STR)
                        .post(RequestBody.create(initAccountRequest.getBytes()))
                        .build()).execute();
    }

    public Response getAccounts() throws IOException {
        return okHttpClient.newCall(
                new Request.Builder()
                        .url(getMaasContainerAddress() + "api/v1/auth/accounts")
                        .addHeader("Authorization", "Basic " + MAAS_MANAGER_ACCOUNT_AUTH_STR)
                        .get()
                        .build()).execute();
    }

    public void deleteAccount(Account account, String namespace) {
        account.setNamespace(namespace);
        String deleteAccountRequest = new GsonBuilder().create().toJson(account);
        try (Response response = okHttpClient.newCall(
                new Request.Builder()
                        .url(getMaasContainerAddress() + "api/v1/auth/account/client")
                        .addHeader("Authorization", "Basic " + MAAS_MANAGER_ACCOUNT_AUTH_STR)
                        .delete(RequestBody.create(deleteAccountRequest.getBytes()))
                        .build()).execute()) {
            log.info("Response: {}", response);
        } catch (Exception e) {
            log.error("Failed to remove client account", e);
        }
    }

    public void deleteAccount(Account account) {
        deleteAccount(account, ((account.getNamespace() == null) ? "" : account.getNamespace()));
    }

    protected KafkaTopicResponse getKafkaTopicByClassifier(int expectStatus, Map<String, Object> classifier) throws IOException {
        log.info("Getting kafka topic with classifier {}", classifier);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_GET_BY_CLASSIFIER_PATH, getMaasBasicAuth(), classifier, MaasITHelper.POST);
        return helper.doRequest(request, KafkaTopicResponse.class, expectStatus);
    }

    protected void deleteTenantTopic(int expectStatus, Map<String, Object> classifier) throws IOException {
        log.info("Delete tenant topic by classifier {}", classifier);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_DELETE_TENANT_TOPICS_PATH, getMaasBasicAuth(), classifier, DELETE);
        helper.doRequest(request, null, expectStatus);
    }

    protected KafkaTopicResponse getKafkaTopicByClassifierWithNamespace(int expectStatus, Map<String, Object> classifier, String namespace) throws IOException {
        log.info("Getting kafka topic with classifier {}", classifier);
        Request request = helper.createJsonRequestWithNamespace(KAFKA_TOPIC_GET_BY_CLASSIFIER_PATH, getMaasBasicAuth(), classifier, MaasITHelper.POST, namespace);
        return helper.doRequest(request, KafkaTopicResponse.class, expectStatus);
    }

    protected KafkaTopicResponse getKafkaTopicByClassifierWithAccount(int expectStatus, Map<String, Object> classifier, Account account, String namespace) throws IOException {
        log.info("Getting kafka topic with classifier {}", classifier);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_GET_BY_CLASSIFIER_PATH, getMaasBasicAuth(account), namespace, classifier, MaasITHelper.POST);
        return helper.doRequest(request, KafkaTopicResponse.class, expectStatus);
    }

    protected KafkaTopicResponse createKafkaTopic(int expectStatus) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("KafkaTopicBasicOperationsIT", "it-test");
        return createKafkaTopic(expectStatus, classifier);
    }

    protected KafkaTopicResponse createKafkaTopic(int expectStatus, Map<String, Object> classifier) throws IOException {
        return createKafkaTopic(expectStatus, KafkaTopicRequest.builder().classifier(classifier).build(), TEST_NAMESPACE);
    }

    protected KafkaTopicResponse createKafkaTopicWithK8sToken(int expectStatus, Map<String, Object> classifier) throws IOException {
        var topicRequest = KafkaTopicRequest.builder().classifier(classifier).build();
        log.info("Create kafka topic with classifier {}", topicRequest.getClassifier());
        Request request = helper.createJsonRequestWithNamespaceAndK8sToken(KAFKA_TOPIC_PATH, k8sAuthHelper.getMaasToken(), topicRequest, POST, TEST_NAMESPACE);
        return helper.doRequest(request, KafkaTopicResponse.class, expectStatus);
    }

    protected KafkaTopicResponse createKafkaTopic(int expectStatus, Map<String, Object> classifier, String namespace) throws IOException {
        return createKafkaTopic(expectStatus, KafkaTopicRequest.builder().classifier(classifier).build(), namespace);
    }

    protected KafkaTopicResponse createKafkaTopic(int expectStatus, KafkaTopicRequest topicRequest) throws IOException {
        return createKafkaTopic(expectStatus, topicRequest, TEST_NAMESPACE);
    }

    protected KafkaTopicResponse createKafkaTopic(int expectStatus, KafkaTopicRequest topicRequest, String namespace) throws IOException {
        return createKafkaTopic(expectStatus, topicRequest, namespace, KafkaTopicResponse.class);
    }

    protected <T> T createKafkaTopic(int expectStatus, KafkaTopicRequest topicRequest, String namespace, Class<T> clazz) throws IOException {
        log.info("Create kafka topic with classifier {}", topicRequest.getClassifier());
        Request request = helper.createJsonRequestWithNamespace(KAFKA_TOPIC_PATH, getMaasBasicAuth(), topicRequest, POST, namespace);
        return helper.doRequest(request, clazz, expectStatus);
    }

    protected <T> T createKafkaTopic(int expectStatus, KafkaTopicRequest topicRequest, Class<T> clazz) throws IOException {
        log.info("Create kafka topic with classifier {}", topicRequest.getClassifier());
        Request request = helper.createJsonRequestWithNamespace(KAFKA_TOPIC_PATH, getMaasBasicAuth(), topicRequest, MaasITHelper.POST, TEST_NAMESPACE);
        return helper.doRequest(request, clazz, expectStatus);
    }

    protected KafkaTemplateConfigResponse.SingleReply createKafkaTemplate(int expectStatus, KafkaTemplateConfigRequest.Spec spec) throws IOException {
        KafkaTemplateConfigRequest topicTemplateRequest = KafkaTemplateConfigRequest.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA_TEMPLATE)
                .spec(spec)
                .build();
        Request request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), topicTemplateRequest, MaasITHelper.POST);
        KafkaTemplateConfigResponse response = new KafkaTemplateConfigResponse();
        response.setReplies(helper.doRequest(request, KafkaTemplateConfigResponse.SingleReply[].class, expectStatus));
        return response.getReplies()[0];
    }

    protected void assertKafkaTemplates(int expectedCount) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_TOPIC_GET_TOPIC_TEMPLATES_PATH, getMaasBasicAuth(), null, "GET");
        var templates = helper.doRequest(request, Object[].class, HttpStatus.SC_OK);
        assertEquals(expectedCount, templates.length);
    }

    protected void assertKafkaLazyTopics(int expectedCount) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_TOPIC_GET_LAZY_TOPICS_PATH, getMaasBasicAuth(), null, "GET");
        var topics = helper.doRequest(request, Object[].class, HttpStatus.SC_OK);
        assertEquals(expectedCount, topics.length);
    }

    protected void assertKafkaTenantTopics(int expectedCount) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_TOPIC_GET_TENANT_TOPICS_PATH, getMaasBasicAuth(), null, "GET");
        var topics = helper.doRequest(request, Object[].class, HttpStatus.SC_OK);
        assertEquals(expectedCount, topics.length);
    }

    protected void assertKafkaTenants(int expectedCount) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_TOPIC_GET_TENANTS_PATH, getMaasBasicAuth(), null, "GET");
        var tenants = helper.doRequest(request, Object[].class, HttpStatus.SC_OK);
        assertEquals(expectedCount, tenants.length);
    }

    protected VirtualHostResponse createVirtualHost(int expectStatus) throws IOException {
        Map<String, Object> vHostClassifier = createSimpleClassifier("VirtualHostBasicOperationsIT", "it-test");
        return createVirtualHost(expectStatus, vHostClassifier);
    }

    protected VirtualHostResponse createVirtualHost(int expectStatus, Map<String, Object> classifier) throws IOException {
        return createVirtualHost(expectStatus, VirtualHostRequest.builder().classifier(classifier).build(), TEST_NAMESPACE);
    }

    protected VirtualHostResponse createVirtualHost(int expectStatus, Map<String, Object> classifier, String instanceId) throws IOException {
        return createVirtualHost(expectStatus, VirtualHostRequest.builder().classifier(classifier).instance(instanceId).build(), TEST_NAMESPACE);
    }

    protected VirtualHostResponse createVirtualHost(int expectStatus, VirtualHostRequest vhRequest, String namespace) throws IOException {
        log.info("Create virtual host with classifier {}", vhRequest.getClassifier());
        Request request = helper.createJsonRequestWithNamespace(RABBIT_VIRTUAL_HOST_PATH, getMaasBasicAuth(), vhRequest, MaasITHelper.POST, namespace);
        return helper.doRequest(request, VirtualHostResponse.class, expectStatus);
    }

    protected VirtualHostResponse createVirtualHostWithK8sToken(int expectStatus) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("VirtualHostBasicOperationsIT", "it-test");
        var vhRequest = VirtualHostRequest.builder().classifier(classifier).build();
        log.info("Create virtual host with classifier {}", vhRequest.getClassifier());
        Request request = helper.createJsonRequestWithNamespaceAndK8sToken(RABBIT_VIRTUAL_HOST_PATH, k8sAuthHelper.getMaasToken(), vhRequest, MaasITHelper.POST, TEST_NAMESPACE);
        return helper.doRequest(request, VirtualHostResponse.class, expectStatus);
    }

    protected String extractVirtualHost(String conn) {
        return conn.split("\\d/")[1];
    }

    protected VhostConfigResponse getRabbitVhostByClassifier(int expectStatus, Map<String, Object> classifier) throws IOException {
        log.info("Getting rabbit vhost with classifier {}", classifier);
        Request request = helper.createJsonRequest(RABBIT_VIRTUAL_HOST_GET_BY_CLASSIFIER_PATH, getMaasBasicAuth(), classifier, MaasITHelper.POST);
        return helper.doRequest(request, VhostConfigResponse.class, expectStatus);
    }

    protected VhostConfigResponse getRabbitVhostByClassifierWithNamespace(int expectStatus, Map<String, Object> classifier, String namespace) throws IOException {
        log.info("Getting rabbit vhost with classifier {}", classifier);
        Request request = helper.createJsonRequestWithNamespace(RABBIT_VIRTUAL_HOST_GET_BY_CLASSIFIER_PATH, getMaasBasicAuth(), classifier, MaasITHelper.POST, namespace);
        return helper.doRequest(request, VhostConfigResponse.class, expectStatus);
    }

    protected List<KafkaTopicResponse> searchKafkaTopics(Map<String, Object> searchCriteria, int expectStatus) throws IOException {
        log.info("Searching kafka topics by search criteria: {}", searchCriteria);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_SEARCH_PATH, getMaasBasicAuth(), searchCriteria, "POST");
        try (Response response = helper.okHttpClient.newCall(request).execute()) {
            log.info("Topics deletion response: {}", response);
            assertEquals(expectStatus, response.code());
            ObjectMapper objectMapper = new ObjectMapper();
            objectMapper.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
            return objectMapper.readValue(response.body().string(), new TypeReference<List<KafkaTopicResponse>>() {
            });
        }
    }

    protected KafkaTopicDeletionResponse deleteKafkaTopics(String namespace, int expectStatus) throws IOException {
        KafkaTopicSearchRequest searchRequest = KafkaTopicSearchRequest.builder().namespace(namespace).build();
        return deleteKafkaTopic(searchRequest, expectStatus);
    }

    protected KafkaTopicDeletionResponse deleteKafkaTopic(KafkaTopicSearchRequest searchRequest, int expectStatus) throws IOException {
        log.info("Deleting kafka topics by search criteria: {}", searchRequest);
        Request request = helper.createJsonRequest(KAFKA_TOPIC_PATH, getMaasBasicAuth(), searchRequest, "DELETE");
        try (Response response = helper.okHttpClient.newCall(request).execute()) {
            log.info("Topics deletion response: {}", response);
            assertEquals(expectStatus, response.code());
            ObjectMapper objectMapper = new ObjectMapper();
            objectMapper.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
            return objectMapper.readValue(response.body().string(), KafkaTopicDeletionResponse.class);
        }
    }

    protected void deleteVhost(SearchCriteria searchForm, int... expectStatus) throws IOException {
        Request request = helper.createJsonRequest(RABBIT_VIRTUAL_HOST_PATH, getMaasBasicAuth(), searchForm, "DELETE");
        try (Response response = helper.okHttpClient.newCall(request).execute()) {
            log.info("Response: {}", response);
            var statusesList = Arrays.stream(expectStatus).boxed().toList();
            var errorMsg = String.format("Unexpected status code received: %d, expected: %s", response.code(), statusesList);
            assertTrue(statusesList.contains(response.code()), errorMsg);
        }
    }

    protected Map<String, Object> createSimpleClassifier(String name) {
        return createSimpleClassifier(name, null);
    }

    protected Map<String, Object> createSimpleClassifier(String name, String tenantId) {
        return createSimpleClassifier(name, tenantId, TEST_NAMESPACE);
    }

    protected Map<String, Object> createSimpleClassifier(String name, String tenantId, String namespace) {
        Map<String, Object> classifier = new HashMap<>();
        if (StringUtils.isNotEmpty(name)) {
            classifier.put(NAME, name);
        }
        if (StringUtils.isNotEmpty(tenantId)) {
            classifier.put(TENANT_ID, tenantId);
        }
        if (StringUtils.isNotEmpty(namespace)) {
            classifier.put(NAMESPACE, namespace);
        }
        return classifier;
    }

    public void deleteVirtualHost(Map<String, Object> classifier) throws IOException {
        VirtualHostRequest vHostRequest = VirtualHostRequest.builder().classifier(classifier).build();
        Request request = helper.createJsonRequest(RABBIT_VIRTUAL_HOST_PATH, getMaasBasicAuth(), vHostRequest, DELETE);
        try (Response response = helper.okHttpClient.newCall(request).execute()) {
            log.info("Response: {}", response);
            String body = response.body().string();
            log.debug("Response body: {}", body);
        }
    }

    public String getMaasBasicAuth() {
        return helper.getBasicAuthorization(agentAccount.getUsername(), agentAccount.getPassword());
    }

    public String getMaasBasicAuth(Account account) {
        return helper.getBasicAuthorization(account.getUsername(), account.getPassword());
    }

    public RabbitConfigResp.SingleReply applySingleConfigWithEntities(Map<String, Object> exchange,
                                                                      Map<String, Object> queue, Map<String, Object> binding) throws IOException {

        return applySingleConfigWithEntities(200, exchange, queue, binding, null, ApplyConfigOperation.CREATE);
    }

    public RabbitConfigResp.SingleReply applySingleConfigWithEntities(Map<String, Object> exchange,
                                                                      Map<String, Object> queue, Map<String, Object> binding, Map<String, Object> policy) throws IOException {

        return applySingleConfigWithEntities(200, exchange, queue, binding, policy, ApplyConfigOperation.CREATE);
    }

    public RabbitConfigResp.SingleReply applySingleConfigWithEntities(int expectHttpCode, Map<String, Object> exchange,
                                                                      Map<String, Object> queue, Map<String, Object> binding, Map<String, Object> policy,
                                                                      ApplyConfigOperation operation) throws IOException {
        return applySingleConfigWithEntities(expectHttpCode, exchange, queue, binding, policy, operation, null);
    }

    public RabbitConfigResp.SingleReply applySingleConfigWithEntities(int expectHttpCode, Map<String, Object> exchange,
                                                                      Map<String, Object> queue, Map<String, Object> binding, Map<String, Object> policy,
                                                                      ApplyConfigOperation operation, String version) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("ApplyRabbitConfigIT", "it-test");

        Map<String, Object>[] exchanges = null;
        if (exchange != null) {
            exchanges = new HashMap[1];
            exchanges[0] = exchange;
        }

        Map<String, Object>[] queues = null;
        if (queue != null) {
            queues = new HashMap[1];
            queues[0] = queue;
        }

        Map<String, Object>[] bindings = null;
        if (binding != null) {
            bindings = new HashMap[1];
            bindings[0] = binding;
        }

        Map<String, Object>[] policies = null;
        if (policy != null) {
            policies = new HashMap[1];
            policies[0] = policy;
        }

        RabbitConfigReq.RabbitEntities entities = RabbitConfigReq.RabbitEntities.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .build();

        RabbitConfigReq.RabbitDeletions deletions = RabbitConfigReq.RabbitDeletions.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .policies(policies)
                .build();

        RabbitConfigReq.Spec spec = null;

        switch (operation) {
            case CREATE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .policies(policies)
                        .build();
                break;
            case DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .deletions(deletions)
                        .build();
                break;
            case CREATE_AND_DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .deletions(deletions)
                        .build();
                break;
            case VERSIONED:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .versionedEntities(entities)
                        .build();
                break;
        }

        RabbitConfigReq rabbitConfigReq = RabbitConfigReq.builder()
                .apiVersion(API_RABBIT_VERSION)
                .kind(KIND_VHOST)
                .spec(spec)
                .build();

        Request request = helper.createYamlRequestWithVersion(APPLY_CONFIG_PATH, getMaasBasicAuth(), rabbitConfigReq, "POST", version);
        RabbitConfigResp rabbitConfigResp = new RabbitConfigResp();
        rabbitConfigResp.setReplies(helper.doRequest(request, RabbitConfigResp.SingleReply[].class, expectHttpCode));
        return rabbitConfigResp.getReplies()[0];
    }

    public ConfigV2Resp.MsResponses applyConfigV2WithEntities(int expectHttpCode, Map<String, Object> exchange,
                                                              Map<String, Object> queue, Map<String, Object> binding, Map<String, Object> policy,
                                                              ApplyConfigOperation operation, String version) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("ApplyRabbitConfigIT", "it-test");

        Map<String, Object>[] exchanges = null;
        if (exchange != null) {
            exchanges = new HashMap[1];
            exchanges[0] = exchange;
        }

        Map<String, Object>[] queues = null;
        if (queue != null) {
            queues = new HashMap[1];
            queues[0] = queue;
        }

        Map<String, Object>[] bindings = null;
        if (binding != null) {
            bindings = new HashMap[1];
            bindings[0] = binding;
        }

        Map<String, Object>[] policies = null;
        if (policy != null) {
            policies = new HashMap[1];
            policies[0] = policy;
        }

        RabbitConfigReq.RabbitEntities entities = RabbitConfigReq.RabbitEntities.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .build();

        RabbitConfigReq.RabbitDeletions deletions = RabbitConfigReq.RabbitDeletions.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .policies(policies)
                .build();

        RabbitConfigReq.Spec spec = null;

        switch (operation) {
            case CREATE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .policies(policies)
                        .build();
                break;
            case DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .deletions(deletions)
                        .build();
                break;
            case CREATE_AND_DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .deletions(deletions)
                        .build();
                break;
            case VERSIONED:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .versionedEntities(entities)
                        .build();
                break;
        }

        RabbitConfigReq rabbitConfigReq = RabbitConfigReq.builder()
                .apiVersion(API_RABBIT_VERSION_V2)
                .kind(KIND_VHOST)
                .spec(spec)
                .build();


        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version(version)
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("service1").
                                config(new Yaml().dump(rabbitConfigReq)).
                                build()
                        }
                )
                .build();


        ConfigV2Req config = new ConfigV2Req(outerSpec);

        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");

        ConfigV2Resp.MsResponses[] resp;
        resp = helper.doRequest(request, ConfigV2Resp.MsResponses[].class, expectHttpCode);

        return resp[0];
    }

    public ConfigV2Resp.MsResponses applyConfigV2WithEntities(int expectHttpCode, String msName, Map<String, Object> exchange,
                                                              Map<String, Object> queue, Map<String, Object> binding, Map<String, Object> policy,
                                                              ApplyConfigOperation operation, String version) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("ApplyRabbitConfigIT", "it-test");

        Map<String, Object>[] exchanges = null;
        if (exchange != null) {
            exchanges = new HashMap[1];
            exchanges[0] = exchange;
        }

        Map<String, Object>[] queues = null;
        if (queue != null) {
            queues = new HashMap[1];
            queues[0] = queue;
        }

        Map<String, Object>[] bindings = null;
        if (binding != null) {
            bindings = new HashMap[1];
            bindings[0] = binding;
        }

        Map<String, Object>[] policies = null;
        if (policy != null) {
            policies = new HashMap[1];
            policies[0] = policy;
        }

        RabbitConfigReq.RabbitEntities entities = RabbitConfigReq.RabbitEntities.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .build();

        RabbitConfigReq.RabbitDeletions deletions = RabbitConfigReq.RabbitDeletions.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .policies(policies)
                .build();

        RabbitConfigReq.Spec spec = null;

        switch (operation) {
            case CREATE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .policies(policies)
                        .build();
                break;
            case DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .deletions(deletions)
                        .build();
                break;
            case CREATE_AND_DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .deletions(deletions)
                        .build();
                break;
            case VERSIONED:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .versionedEntities(entities)
                        .build();
                break;
        }

        RabbitConfigReq rabbitConfigReq = RabbitConfigReq.builder()
                .apiVersion(API_RABBIT_VERSION_V2)
                .kind(KIND_VHOST)
                .spec(spec)
                .build();


        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version(version)
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName(msName).
                                config(new Yaml().dump(rabbitConfigReq)).
                                build()
                        }
                )
                .build();


        ConfigV2Req config = new ConfigV2Req(outerSpec);

        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");

        ConfigV2Resp.MsResponses[] resp;
        resp = helper.doRequest(request, ConfigV2Resp.MsResponses[].class, expectHttpCode);

        return resp[0];
    }

    public ConfigV2Resp.MsResponses applyConfigV2WithMultipleEntities(int expectHttpCode, Map<String, Object>[] exchanges,
                                                                      Map<String, Object>[] queues, Map<String, Object>[] bindings, Map<String, Object> policy,
                                                                      ApplyConfigOperation operation, String version) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("ApplyRabbitConfigIT", "it-test");

        Map<String, Object>[] policies = null;
        if (policy != null) {
            policies = new HashMap[1];
            policies[0] = policy;
        }

        RabbitConfigReq.RabbitEntities entities = RabbitConfigReq.RabbitEntities.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .build();

        RabbitConfigReq.RabbitDeletions deletions = RabbitConfigReq.RabbitDeletions.builder()
                .exchanges(exchanges)
                .queues(queues)
                .bindings(bindings)
                .policies(policies)
                .build();

        RabbitConfigReq.Spec spec = null;

        switch (operation) {
            case CREATE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .policies(policies)
                        .build();
                break;
            case DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .deletions(deletions)
                        .build();
                break;
            case CREATE_AND_DELETE:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(entities)
                        .deletions(deletions)
                        .build();
                break;
            case VERSIONED:
                spec = RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .versionedEntities(entities)
                        .build();
                break;
        }

        RabbitConfigReq rabbitConfigReq = RabbitConfigReq.builder()
                .apiVersion(API_RABBIT_VERSION_V2)
                .kind(KIND_VHOST)
                .spec(spec)
                .build();


        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version(version)
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("service1").
                                config(new Yaml().dump(rabbitConfigReq)).
                                build()
                        }
                )
                .build();


        ConfigV2Req config = new ConfigV2Req(outerSpec);

        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");

        ConfigV2Resp.MsResponses[] resp;
        resp = helper.doRequest(request, ConfigV2Resp.MsResponses[].class, expectHttpCode);

        return resp[0];
    }


    public ConfigV2Resp applyConfigV2(int expectHttpCode, String config) throws IOException {
        Request request = helper.createRequestV2ByYaml(APPLY_CONFIG_V2_PATH, "Basic", getMaasBasicAuth(), config, "POST");
        ConfigV2Resp resp;
        resp = helper.doRequest(request, ConfigV2Resp.class, expectHttpCode);

        if (expectHttpCode == 200) {
            assertEquals(STATUS_OK, resp.getStatus());
        } else {
            assertEquals(STATUS_ERROR, resp.getStatus());
        }
        return resp;
    }

    public ConfigV2Resp applyMsConfigs(int expectHttpCode, MsConfig[] msConfigs, String version) throws IOException {
        Map<String, Object> classifier = createSimpleClassifier("ApplyRabbitConfigIT", "it-test");

        ArrayList<ConfigV2Req.Service> services = new ArrayList<>();

        for (MsConfig msConfig : msConfigs) {
            RabbitConfigReq.RabbitEntities entities = RabbitConfigReq.RabbitEntities.builder()
                    .exchanges(msConfig.getExchanges())
                    .queues(msConfig.getQueues())
                    .bindings(msConfig.getBindings())
                    .build();


            RabbitConfigReq.Spec spec = RabbitConfigReq.Spec.builder()
                    .classifier(classifier)
                    .instanceId(DEFAULT_RABBIT_INSTANCE)
                    .versionedEntities(entities)
                    .build();

            RabbitConfigReq rabbitConfigReq = RabbitConfigReq.builder()
                    .apiVersion(API_RABBIT_VERSION_V2)
                    .kind(KIND_VHOST)
                    .spec(spec)
                    .build();

            services.add(ConfigV2Req.Service.builder().
                    serviceName(msConfig.getMsName()).
                    config(new Yaml().dump(rabbitConfigReq)).
                    build()
            );
        }


        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version(version)
                .services(services.stream().toArray(ConfigV2Req.Service[]::new))
                .build();


        ConfigV2Req config = new ConfigV2Req(outerSpec);

        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");

        ConfigV2Resp resp;
        resp = helper.doRequest(request, ConfigV2Resp.class, expectHttpCode);

        return resp;
    }

    public RabbitConfigValidation validateRabbitConfigs() throws IOException {
        Request request = helper.createJsonRequest(VALIDATE_RABBIT_CONFIGS, getMaasBasicAuth(), null, "GET");

        RabbitConfigValidation resp;
        resp = helper.doRequest(request, RabbitConfigValidation.class, HttpStatus.SC_OK);

        return resp;
    }

    public void changeActiveVersion(CpMessage[] cpMessages) throws IOException {
        Request request = helper.createJsonRequest(SEND_CP_MESSAGE, getMaasBasicAuth(), cpMessages, "POST");
        helper.doRequest(request, String.class, HttpStatus.SC_OK);
    }

    public String convertMapToStream(Map<?, ?> map) {
        String mapAsString = map.keySet().stream()
                .map(key -> key + "=" + map.get(key))
                .collect(Collectors.joining(", ", "{", "}"));
        return mapAsString;
    }

    protected KafkaLazyTopicConfigResponse.SingleReply createKafkaLazyTopic(int expectStatus, KafkaLazyTopicConfigRequest.Spec spec) throws IOException {
        KafkaLazyTopicConfigRequest lazyTopicConfigRequestRequest = KafkaLazyTopicConfigRequest.builder()
                .spec(spec)
                .build();
        Request request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), lazyTopicConfigRequestRequest, MaasITHelper.POST);
        KafkaLazyTopicConfigResponse response = new KafkaLazyTopicConfigResponse();
        response.setReplies(helper.doRequest(request, KafkaLazyTopicConfigResponse.SingleReply[].class, expectStatus));
        return response.getReplies()[0];
    }

    protected KafkaTopicResponse applyLazyTopic(int expectStatus, Map<String, Object> classifier) throws IOException {
        return applyLazyTopic(expectStatus, classifier, KafkaTopicResponse.class);
    }

    protected <T> T applyLazyTopic(int expectStatus, Map<String, Object> classifier, Class<T> clazz) throws IOException {
        log.info("apply kafka lazy topic with classifier {}", classifier);
        Request request = helper.createJsonRequest(KAFKA_APPLY_LAZY_TOPIC_PATH, getMaasBasicAuth(), classifier, MaasITHelper.POST);
        return helper.doRequest(request, clazz, expectStatus);
    }

    protected KafkaTenantTopicConfigResponse.SingleReply createKafkaTenantTopic(int expectStatus, KafkaTenantTopicConfigRequest.Spec spec) throws IOException {
        KafkaTenantTopicConfigRequest lazyTopicConfigRequestRequest = KafkaTenantTopicConfigRequest.builder()
                .spec(spec)
                .build();
        Request request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), lazyTopicConfigRequestRequest, MaasITHelper.POST);
        KafkaTenantTopicConfigResponse response = new KafkaTenantTopicConfigResponse();
        response.setReplies(helper.doRequest(request, KafkaTenantTopicConfigResponse.SingleReply[].class, expectStatus));
        return response.getReplies()[0];
    }

    protected SyncTenantsResponse[] applyTenants(int expectStatus, List<Map<String, Object>> tenants) throws IOException {
        log.info("apply tenants {}", tenants);
        Request request = helper.createJsonRequest(APPLY_TENANTS_PATH, getMaasBasicAuth(), tenants, MaasITHelper.POST);
        return helper.doRequest(request, SyncTenantsResponse[].class, expectStatus);
    }

    public Properties preparePropertiesAndPortForwardKafka(KafkaInstance kafkaInstance) {
        List<String> kafkaUrls = Collections.singletonList(KAFKA_CONTAINER_1.getBootstrapServers());
        log.info("final kafkaUrls {}", kafkaUrls);

        return prepareKafkaProperties(kafkaInstance, kafkaUrls);
    }

    public Properties preparePropertiesAndPortForwardKafka() throws IOException {
        KafkaInstance defaultKafkaInstance = getDefaultKafkaInstance();
        if (defaultKafkaInstance == null) {
            log.warn("There is no default kafka");
            return null;
        }
        log.info("default kafka instance {}", defaultKafkaInstance);
        return preparePropertiesAndPortForwardKafka(defaultKafkaInstance);
    }

    public void createKafkaTopic(Properties props) throws IOException {
        createKafkaTopic(props, TOPIC_NAME);
    }

    public void createKafkaTopic(Properties props, String topicName) throws IOException {
        AdminClient adminClient = AdminClient.create(props);
        NewTopic newTopic = new NewTopic(topicName, 1, (short) 1); //new NewTopic(topicName, numPartitions, replicationFactor)

        adminClient.createTopics(Lists.newArrayList(newTopic));
        adminClient.close();
    }

    public Set<String> getAllKafkaTopicsNames(Properties props) throws ExecutionException, InterruptedException {
        AdminClient adminClient = AdminClient.create(props);
        Set<String> names = adminClient.listTopics().names().get();
        adminClient.close();
        return names;
    }

    public Map<String, TopicDescription> describeKafkaTopic(Properties props, List<String> topicNames) throws ExecutionException, InterruptedException {
        AdminClient adminClient = AdminClient.create(props);
        DescribeTopicsResult describeTopics = adminClient.describeTopics(topicNames);
        Map<String, TopicDescription> topicDescriptionMap = describeTopics.allTopicNames().get();
        adminClient.close();
        return topicDescriptionMap;
    }

    public void deleteKafkaTopic(Properties props) throws IOException {
        deleteKafkaTopic(props, TOPIC_NAME);
    }

    public void deleteKafkaTopic(Properties props, String topicName) throws IOException {
        AdminClient adminClient = AdminClient.create(props);

        adminClient.deleteTopics(Lists.newArrayList(topicName));
        adminClient.close();
    }

    protected Properties prepareKafkaProperties(KafkaInstance defaultKafkaInstance, List<String> kafkaUrls) {
        String maasProtocol = defaultKafkaInstance.getMaasProtocol();

        Properties props = new Properties();
        props.put("bootstrap.servers", kafkaUrls);
        props.put("security.protocol", maasProtocol);

        String jaasTemplate;

        if (defaultKafkaInstance.getCredentials() == null) {
            return props;
        }

        Map<String, String> adminCreds = defaultKafkaInstance.getCredentials().get("admin").get(0);
        switch (adminCreds.get("type")) {
            case "SCRAM":
                jaasTemplate = "org.apache.kafka.common.security.scram.ScramLoginModule required username=\"%s\" password=\"%s\";";
                props.put("sasl.mechanism", "SCRAM-SHA-512");
                break;
            case "plain":
                jaasTemplate = "org.apache.kafka.common.security.plain.PlainLoginModule required username=\"%s\" password=\"%s\";";
                props.put("sasl.mechanism", "PLAIN");
                break;
            default:
                return null;
        }
        String jaasCfg = String.format(jaasTemplate, adminCreds.get("username"), adminCreds.get("password").split(":")[1]);
        props.put("sasl.jaas.config", jaasCfg);

        log.info("kafka properties to connect: {}", props);
        return props;
    }

    protected KafkaInstance getDefaultKafkaInstance() throws IOException {
        Request request = helper.createJsonRequest(KAFKA_INSTANCES_PATH, getMaasBasicAuth(), null, GET);
        KafkaInstance[] kafkaInstances = helper.doRequest(request, KafkaInstance[].class, HttpStatus.SC_OK);
        return Arrays.stream(kafkaInstances).filter(kafkaInstance -> kafkaInstance.getIsDefault().equals(true)).findFirst().get();
    }

    protected KafkaInstance[] getKafkaInstances() throws IOException {
        Request request = helper.createJsonRequest(KAFKA_INSTANCES_PATH, getMaasBasicAuth(), null, GET);
        return helper.doRequest(request, KafkaInstance[].class, HttpStatus.SC_OK);
    }

    protected RabbitInstance[] getRabbitInstances() throws IOException {
        Request request = helper.createJsonRequest(RABBIT_INSTANCES_PATH, getMaasBasicAuth(), null, GET);
        return helper.doRequest(request, RabbitInstance[].class, HttpStatus.SC_OK);
    }

    protected RabbitInstance getDefaultRabbitInstance() throws IOException {
        Request request = helper.createJsonRequest(RABBIT_INSTANCES_PATH, getMaasBasicAuth(), null, GET);
        RabbitInstance[] rabbitInstances = helper.doRequest(request, RabbitInstance[].class, HttpStatus.SC_OK);
        return Arrays.stream(rabbitInstances).filter(rabbitInstance -> rabbitInstance.getIsDefault().equals(true)).findFirst().get();
    }

    public Connection createRabbitConnect(VirtualHostResponse virtualHost) throws Exception {
        return createRabbitConnect(virtualHost, true);
    }

    public Connection createRabbitConnect(VirtualHostResponse virtualHost, boolean failsafe) throws Exception {
        URI uri = URI.create(virtualHost.getCnn());
        ConnectionFactory factory = new ConnectionFactory();
        String amqpUrl = RABBITMQ_CONTAINER_1.getAmqpUrl();
        if (uri.getHost().equals(RABBITMQ_CONTAINER_2.getNetworkAliases().get(0))) {
            amqpUrl = RABBITMQ_CONTAINER_2.getAmqpUrl();
        }
        factory.setUri(amqpUrl + uri.getPath());
        factory.setUsername(virtualHost.getUsername());
        factory.setPassword(getPassword(virtualHost.getPassword()));

        if (failsafe) {
            return Failsafe.with(new RetryPolicy<Connection>()
                    .withMaxRetries(10)
                    .withBackoff(1, 5, ChronoUnit.SECONDS)
            ).get(() -> factory.newConnection());
        } else {
            return factory.newConnection();
        }
    }

    private String getPassword(String password) {
        return password.split(":")[1];
    }

    protected void deleteNamespace(String namespace) throws IOException {
        Request request = helper.createJsonRequest(DELETE_NAMESPACE_PATH, getMaasBasicAuth(), Collections.singletonMap("namespace", namespace), MaasITHelper.DELETE);
        helper.doRequest(request, Object.class, HttpStatus.SC_OK);
    }

    protected void deleteKafkaInstanceDesignator(int... expectStatuses) throws IOException {
        log.info("deleteKafkaInstanceDesignator in test namespace");
        Request request = helper.createJsonRequest(KAFKA_INSTANCE_DESIGNATOR_PATH, getMaasBasicAuth(), "", MaasITHelper.DELETE);
        helper.doRequest(request, expectStatuses);
    }

    protected void deleteRabbitInstanceDesignator(int... expectStatuses) throws IOException {
        log.info("deleteRabbitInstanceDesignator in test namespace");
        Request request = helper.createJsonRequest(RABBIT_INSTANCE_DESIGNATOR_PATH, getMaasBasicAuth(), "", MaasITHelper.DELETE);
        helper.doRequest(request, expectStatuses);
    }

    protected KafkaInstance createKafkaInstance(KafkaInstance createRequest) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_INSTANCE_PATH, getMaasBasicAuth(), createRequest, POST);
        return helper.doRequest(request, KafkaInstance.class, HttpStatus.SC_OK);
    }

    protected KafkaInstance updateKafkaInstance(KafkaInstance updateRequest) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_INSTANCE_PATH, getMaasBasicAuth(), updateRequest, PUT);
        return helper.doRequest(request, KafkaInstance.class, HttpStatus.SC_OK);
    }

    protected void deleteKafkaInstance(KafkaInstance deleteRequest, int... expectStatuses) throws IOException {
        Request request = helper.createJsonRequest(KAFKA_INSTANCE_PATH, getMaasBasicAuth(), deleteRequest, DELETE);
        helper.doRequest(request, expectStatuses);
    }

    protected RabbitInstance createRabbitInstance(RabbitInstance createRequest) throws IOException {
        Request request = helper.createJsonRequest(RABBIT_INSTANCE_PATH, getMaasBasicAuth(), createRequest, POST);
        return helper.doRequest(request, RabbitInstance.class, HttpStatus.SC_OK);
    }

    protected RabbitInstance updateRabbitInstance(RabbitInstance createRequest) throws IOException {
        Request request = helper.createJsonRequest(RABBIT_INSTANCE_PATH, getMaasBasicAuth(), createRequest, PUT);
        return helper.doRequest(request, RabbitInstance.class, HttpStatus.SC_OK);
    }

    protected void deleteRabbitInstance(RabbitInstance deleteRequest, int... expectStatuses) throws IOException {
        Request request = helper.createJsonRequest(RABBIT_INSTANCE_PATH, getMaasBasicAuth(), deleteRequest, DELETE);
        helper.doRequest(request, expectStatuses);
    }

}
