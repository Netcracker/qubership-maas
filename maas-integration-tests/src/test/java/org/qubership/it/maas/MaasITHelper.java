package org.qubership.it.maas;

import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.dataformat.yaml.YAMLFactory;
import lombok.NonNull;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import net.jodah.failsafe.Failsafe;
import net.jodah.failsafe.RetryPolicy;
import okhttp3.*;
import org.apache.commons.codec.binary.Base64;
import org.apache.commons.codec.binary.StringUtils;
import org.yaml.snakeyaml.Yaml;

import java.io.IOException;
import java.net.URL;
import java.time.Duration;
import java.util.Arrays;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.concurrent.TimeUnit;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static org.apache.http.HttpStatus.SC_OK;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;


@Slf4j
@RequiredArgsConstructor
public class MaasITHelper {
    private ObjectMapper objectMapper = new ObjectMapper();
    /**
     * Regex to search for service name and namespace in the database connection string.
     * <p>Explanation:
     * <p>Example string: 'http://elasticsearch.dbaas.svc.cluster.local:9200/dbaas_autotests-69c0c8ab-0a87-4fbc-ac3b-5b57b2d97bc3'
     * <p><b>group 1</b>: line beginning and protocol, if exists, e.g. 'http://'
     * <p><b>group 2</b>: service name we are looking for, e.g. 'elasticsearch'
     * <p><b>group 3</b>: namespace we are looking for, e.g. 'dbaas'
     * <p><b>group 5</b>: container port
     * <p><b>group 6</b>: the rest of the line, separated by comma or colon before the port, if exists, and line ending,
     * e.g. '.svc.cluster.local:9200/dbaas_autotests-69c0c8ab-0a87-4fbc-ac3b-5b57b2d97bc3'
     */
    private static final Pattern SERVICE_NAME_WITH_NAMESPACE_PATTERN = Pattern.compile("(^.*?://|^)(.*?)\\.(.*?)([.:])(\\d*|^)/?(.*?$|$)");
    private static final Pattern RABBIT_CNN_PATTERN = Pattern.compile("(^.*?://)(.*?)/(.*?$)");

    protected static final MediaType JSON
            = MediaType.parse("application/json; charset=utf-8");
    protected static final MediaType YAML
            = MediaType.parse("text/yaml; charset=utf-8");
    public final OkHttpClient okHttpClient = new OkHttpClient.Builder()
            .readTimeout(60, TimeUnit.SECONDS)
            .writeTimeout(60, TimeUnit.SECONDS)
            .build();
    public static final String TEST_NAMESPACE = "maas-it-test";
    public static final String TEST_NAMESPACE_2_BG = "maas-it-test-2";
    public static final String TEST_MICROSERVICE = "maas-it-test-microservice";
    public static final String TEST_NAMESPACE_WITHOUT_TOPICS = "maas-it-test-no-topics";
    public static final String PUT = "PUT";
    public static final String POST = "POST";
    public static final String DELETE = "DELETE";
    public static final String GET = "GET";
    public static final String MONITORING_PATH = "api/v2/monitoring/entity-distribution";
    public static final String RABBIT_VIRTUAL_HOST_PATH = "api/v1/rabbit/vhost";
    public static final String RABBIT_VIRTUAL_HOST_GET_BY_CLASSIFIER_PATH = "api/v1/rabbit/vhost/get-by-classifier";
    public static final String SEND_CP_MESSAGE = "api/v1/bg-status";
    public static final String APPLY_CONFIG_PATH = "api/v1/config";
    public static final String APPLY_CONFIG_V2_PATH = "api/v2/config";
    public static final String KAFKA_TOPIC_PATH = "api/v1/kafka/topic";
    public static final String KAFKA_TOPIC_GET_BY_CLASSIFIER_PATH = "api/v1/kafka/topic/get-by-classifier";
    public static final String KAFKA_TOPIC_SEARCH_PATH = KAFKA_TOPIC_PATH + "/search";
    public static final String KAFKA_TOPIC_GET_TOPIC_TEMPLATES_PATH = "api/v1/kafka/topic-templates";
    public static final String KAFKA_TOPIC_GET_LAZY_TOPICS_PATH = "api/v1/kafka/lazy-topics/definitions";
    public static final String KAFKA_TOPIC_GET_TENANT_TOPICS_PATH = "api/v1/kafka/tenant-topics";
    public static final String KAFKA_TOPIC_DELETE_TENANT_TOPICS_PATH = "api/v2/kafka/tenant-topic";
    public static final String KAFKA_TOPIC_GET_TENANTS_PATH = "api/v1/tenants";
    public static final String KAFKA_APPLY_LAZY_TOPIC_PATH = "api/v1/kafka/lazy-topic";
    public static final String APPLY_TENANTS_PATH = "api/v1/synchronize-tenants";
    public static final String KAFKA_INSTANCES_PATH = "api/v1/kafka/instances";
    public static final String KAFKA_INSTANCE_PATH = "api/v1/kafka/instance";
    public static final String RABBIT_INSTANCES_PATH = "api/v1/rabbit/instances";
    public static final String RABBIT_INSTANCE_PATH = "api/v1/rabbit/instance";
    public static final String KAFKA_INSTANCE_DESIGNATOR_PATH = "api/v1/kafka/instance-designator";
    public static final String RABBIT_INSTANCE_DESIGNATOR_PATH = "api/v1/rabbit/instance-designator";
    public static final String DELETE_NAMESPACE_PATH = "api/v1/namespace";
    public static final String VALIDATE_RABBIT_CONFIGS = "api/v2/rabbit/validations";
    public static final String DISCREPANCY_REPORT_PATH = "api/v2/kafka/discrepancy-report";
    public static final String RABBIT_RECOVERY_PATH = "api/v2/rabbit/recovery/" + TEST_NAMESPACE;
    public static final String KAFKA_RECOVERY_PATH = "api/v2/kafka/recovery";
    public static final String MONITORING_ENTITY_REQUEST_AUDIT_PATH = "api/v2/monitoring/entity-request-audit";
    public static final String CR_APPLY = "api/declarations/v1/apply";

    public static final String BG2_INIT_DOMAIN = "api/bluegreen/v1/operation/init-domain";
    public static final String BG2_WARMUP = "api/bluegreen/v1/operation/warmup";
    public static final String BG2_PROMOTE = "api/bluegreen/v1/operation/promote";
    public static final String BG2_COMMIT = "api/bluegreen/v1/operation/commit";
    public static final String BG2_DESTROY_DOMAIN = "api/bluegreen/v1/operation/destroy-domain";


    @NonNull
    private String maasAddress;

    private final ObjectMapper mapper = new ObjectMapper();

    public Request createJsonRequest(String url, String authorization, Object body, String method) throws JsonProcessingException {
        String content = mapper.writeValueAsString(body);
        log.info("Request body {}, url {}, method {}, auth {}", content, url, method, authorization);
        return new Request.Builder()
                .url(maasAddress + url)
                .addHeader("Authorization", "Basic " + authorization)
                .addHeader("X-Origin-Namespace", TEST_NAMESPACE)
                .addHeader("X-Origin-Microservice", TEST_MICROSERVICE)
                .addHeader("Content-Type", "application/json")
                .method(method, body != null ? RequestBody.create(content, JSON) : null)
                .build();
    }

    public Request createJsonRequestWithNamespace(String url, String authorization, Object body, String method, String namespace) throws JsonProcessingException {
        String content = mapper.writeValueAsString(body);
        log.info("Request body {}, url {}, method {}, auth {}", content, url, method, authorization);
        return new Request.Builder()
                .url(maasAddress + url)
                .addHeader("Authorization", "Basic " + authorization)
                .addHeader("X-Origin-Namespace", namespace)
                .addHeader("X-Origin-Microservice", TEST_MICROSERVICE)
                .addHeader("Content-Type", "application/json")
                .method(method, body != null ? RequestBody.create(content, JSON) : null)
                .build();
    }

    public Request createJsonRequest(String url, String authorization, String namespace, Object body, String method) throws JsonProcessingException {
        String content = mapper.writeValueAsString(body);
        log.info("Request body {}, url {}, method {}, auth {}", content, url, method, authorization);
        return new Request.Builder()
                .url(maasAddress + url)
                .addHeader("Authorization", "Basic " + authorization)
                .addHeader("X-Origin-Namespace", namespace)
                .addHeader("X-Origin-Microservice", TEST_MICROSERVICE)
                .addHeader("Content-Type", "application/json")
                .method(method, body != null ? RequestBody.create(content, JSON) : null)
                .build();
    }

    public Request createYamlRequest(String url, String authorization, Object body, String method) throws JsonProcessingException {
        ObjectMapper mapper = new ObjectMapper(new YAMLFactory());
        return createYamlRequest(url, authorization, body != null ? mapper.writeValueAsString(body) : null, method, null);
    }

    public Request createYamlRequest(String url, String authorization, List<Object> body, String method) {
        String content = new Yaml().dumpAll(body.listIterator());
        return createYamlRequest(url, authorization, content, method, null);
    }

    public Request createYamlRequestWithVersion(String url, String authorization, Object body, String method, String version) {
        String content = new Yaml().dump(body);
        return createYamlRequest(url, authorization, content, method, version);
    }

    public Request createYamlRequest(String url, String authorization, String content, String method, String version) {
        log.info("Request body {}, url {}, method {}, auth {}", content, url, method, authorization);
        Request.Builder builder = new Request.Builder()
                .url(maasAddress + url)
                .addHeader("Authorization", "Basic " + authorization)
                .addHeader("X-Origin-Namespace", TEST_NAMESPACE)
                .addHeader("X-Origin-Microservice", TEST_MICROSERVICE)
                .addHeader("Content-Type", "text/yaml")
                .method(method, content != null ? RequestBody.create(content, YAML) : null);

        if (version != null) {
            builder.addHeader("X-Version", version);
        }

        return builder.build();
    }

    public Request createYamlRequestV2(String url, String authorization, Object body, String method) {
        String content = new Yaml().dump(body);

        log.info("Request body {}, url {}, method {}, auth {}", content, url, method, authorization);
        Request.Builder builder = new Request.Builder()
                .url(maasAddress + url)
                .addHeader("Authorization", "Basic " + authorization)
                .addHeader("Content-Type", "text/yaml")
                .method(method, content != null ? RequestBody.create(content, YAML) : null);

        return builder.build();
    }

    public Request createRequestV2ByYaml(String url, String authorization, String content, String method) {
        log.info("Request body {}, url {}, method {}, auth {}", content, url, method, authorization);
        Request.Builder builder = new Request.Builder()
                .url(maasAddress + url)
                .addHeader("Authorization", "Basic " + authorization)
                .addHeader("Content-Type", "text/yaml")
                .method(method, content != null ? RequestBody.create(content, YAML) : null);

        return builder.build();
    }


    public String doRequestWithStringResponse(Request request, int expectHttpCode) throws IOException {
        log.info("Request: {}", request);
        try (Response response = okHttpClient.newCall(request).execute()) {
            log.info("Response: {}", response);
            String body = response.body().string();
            log.info("Response body: {}", body);
            assertEquals(expectHttpCode, response.code());
            return body;
        }
    }

    // TODO why so much code duplicate? look at the previous method
    public <T> T doRequest(Request request, Class<T> clazz, int... expectHttpCodes) throws IOException {
        var codes = Arrays.stream(expectHttpCodes).boxed().toList();
        var requestWithRequestId = request.newBuilder().addHeader("x-request-id", UUID.randomUUID().toString()).build();
        log.info("Request: {}", requestWithRequestId);

        RetryPolicy<Object> retryPolicy = new RetryPolicy<>()
                .handle(IOException.class)
                .handle(AssertionError.class)
                .withMaxRetries(10)
                .withDelay(Duration.ofSeconds(1));

        return Failsafe.with(retryPolicy).get(() -> {
            try (Response response = okHttpClient.newCall(requestWithRequestId).execute()) {
                var body = response.body().string();
                log.info("Response: {}\n\tBody: {}", response, body);
                if (expectHttpCodes.length == 0) {
                    assertEquals(SC_OK, response.code());
                } else {
                    assertTrue(codes.contains(response.code()), "Unexpected response code received: " + response.code() + ", expected: " + codes);
                }
                if (clazz == null) {
                    return null;
                }

                objectMapper.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
                return (!body.isBlank() ? objectMapper.readValue(body, clazz) : null);
            }
        });
    }

    public void doRequest(Request request, int... expectHttpCodes) throws IOException {
        doRequest(request, null, expectHttpCodes);
    }

    public String getBasicAuthorization(String user, String password) {
        return java.util.Base64.getEncoder().encodeToString((user + ":" + password).getBytes());
    }

    public String getProtocolFromUrl(String url) {
        Matcher matcher = SERVICE_NAME_WITH_NAMESPACE_PATTERN.matcher(url);
        assertTrue(matcher.find());
        return matcher.group(1);
    }

    public String getServiceNameFromUrl(String url) {
        Matcher matcher = SERVICE_NAME_WITH_NAMESPACE_PATTERN.matcher(url);
        assertTrue(matcher.find());
        return matcher.group(2);
    }

    public String getServiceNamespaceFromUrl(String url) {
        Matcher matcher = SERVICE_NAME_WITH_NAMESPACE_PATTERN.matcher(url);
        assertTrue(matcher.find());
        return matcher.group(3);
    }

    public int getServicePortFromUrl(String url) {
        Matcher matcher = SERVICE_NAME_WITH_NAMESPACE_PATTERN.matcher(url);
        assertTrue(matcher.find());
        return Integer.parseInt(matcher.group(5));
    }

    public String getVhostFromCnn(String cnn) {
        Matcher matcher = RABBIT_CNN_PATTERN.matcher(cnn);
        assertTrue(matcher.find());
        return matcher.group(3);
    }
}
