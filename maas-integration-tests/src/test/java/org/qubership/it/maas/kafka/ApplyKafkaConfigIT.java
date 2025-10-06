package org.qubership.it.maas.kafka;

import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.ConfigV2Req;
import org.qubership.it.maas.entity.kafka.KafkaConfigReq;
import org.qubership.it.maas.entity.kafka.KafkaTemplateConfigRequest;
import org.qubership.it.maas.entity.kafka.KafkaTenantTopicConfigRequest;
import org.qubership.it.maas.entity.kafka.KafkaTenantTopicConfigResponse;
import org.qubership.it.maas.entity.rabbit.RabbitConfigResp;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Collections;
import java.util.List;
import java.util.Map;

import static org.qubership.it.maas.MaasITHelper.*;
import static org.apache.http.HttpStatus.*;
import static org.junit.jupiter.api.Assertions.*;

@Slf4j
class ApplyKafkaConfigIT extends AbstractMaasWithInitsIT {

    @Test
    public void applyTopicConfiguration_ExternalManagedAndWithEmptyName() throws IOException {
        KafkaConfigReq kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec("", createSimpleClassifier("ApplyConfigIT", "it-test"), true))
                .build();
        Request request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), Collections.singletonList(kafkaConfigReq), "POST");
        helper.doRequest(request, RabbitConfigResp.SingleReply[].class, HttpStatus.SC_INTERNAL_SERVER_ERROR);
    }

    @Test
    public void applyTopicConfiguration_ExternalManagedFalse() throws IOException {
        KafkaConfigReq kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec("ApplyConfigIT", createSimpleClassifier("ApplyConfigIT", "it-test"), false))
                .build();
        Request request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), Collections.singletonList(kafkaConfigReq), "POST");
        RabbitConfigResp.SingleReply[] replies = helper.doRequest(request, RabbitConfigResp.SingleReply[].class, SC_OK);
        assertEquals(1, replies.length);
        assertEquals("ok", replies[0].getResult().getStatus());
        assertNull(replies[0].getResult().getError());
    }

    @Test
    public void applyTopicConfiguration_UpdatePartitions() throws IOException {
        String requestString = "apiVersion: nc.maas.config/v2\n" +
                "kind: config\n" +
                "spec:\n" +
                "  version: v1\n" +
                "  namespace: maas-it-test\n" +
                "  services:\n" +
                "  - serviceName: maas-it-test-service\n" +
                "    config: |\n" +
                "      ---\n" +
                "      apiVersion: nc.maas.kafka/v1\n" +
                "      kind: topic\n" +
                "      pragma:\n" +
                "        on-entity-exists: merge\n" +
                "      spec:\n" +
                "        classifier:\n" +
                "          name: topic-2\n" +
                "          namespace: maas-it-test\n" +
                "        name: \"maas-it-test.topic-2\"\n" +
                "        numPartitions: 1\n" +
                "        replicationFactor: inherit";
        Request request1 = helper.createRequestV2ByYaml(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), requestString, "POST");
        helper.doRequest(request1, Map.class, SC_OK);

        requestString = requestString.replace("numPartitions: 1", "numPartitions: 2");
        Request request2 = helper.createRequestV2ByYaml(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), requestString, "POST");
        helper.doRequest(request2, Map.class, SC_OK);
    }

    @Test
    void applyTopicConfiguration_InheritReplicationFactor() throws IOException {
        String configStr =
                "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: tenant-topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: inherit\n" +
                        "\n" +
                        "---\n" +
                        "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: tenant-topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc2\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: -1\n" +
                        "\n" +
                        "---\n" +
                        "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: tenant-topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc3\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: 1\n" +
                        "\n" +
                        "---\n" +
                        "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc3\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: 1";
        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version("v1")
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("agreement-mgmt-core").
                                config(configStr).
                                build()
                        }
                )
                .build();
        ConfigV2Req config = new ConfigV2Req(outerSpec);

        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");
        Map<String, Object> replies = helper.doRequest(request, Map.class, SC_OK);
        assertEquals("ok", replies.get("status"));
    }

    @Test
    void applyTopicConfiguration_WithInvalidReplicationFactor() throws IOException {
        String configStr =
                "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: tenant-topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: inherit\n" +
                        "    name: maas-it-test.abc" +
                        "\n" +
                        "---\n" +
                        "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: tenant-topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc2\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: -1\n" +
                        "    name: maas-it-test.abc2" +
                        "\n" +
                        "---\n" +
                        "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: tenant-topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc3\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: NOT_A_NUMBER\n" +
                        "    name: maas-it-test.abc3" +
                        "\n" +
                        "---\n" +
                        "apiVersion: nc.maas.kafka/v1\n" +
                        "kind: topic\n" +
                        "spec:\n" +
                        "    classifier:\n" +
                        "        name: abc4\n" +
                        "        namespace: maas-it-test\n" +
                        "    replicationFactor: 1\n" +
                        "    name: maas-it-test.abc4";
        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version("v1")
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("agreement-mgmt-core").
                                config(configStr).
                                build()
                        }
                )
                .build();
        ConfigV2Req config = new ConfigV2Req(outerSpec);

        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");
        Map<String, Object> replies = helper.doRequest(request, Map.class, HttpStatus.SC_BAD_REQUEST);
        assertEquals("error", replies.get("status"));
        assertTrue(((String) replies.get("error")).contains("NOT_A_NUMBER")); // error says something about invalid value
    }

    @Test
    void applyDeleteTopicConfiguration() throws IOException {
        String configStr =
                "apiVersion: nc.maas.kafka/v2\n" +
                        "kind: topic-delete\n" +
                        "spec:\n" +
                        "  classifier: \n" +
                        "    name: my-test\n" +
                        "    namespace: maas-it-test";

        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version("v1")
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("agreement-mgmt-core").
                                config(configStr).
                                build()
                        }
                )
                .build();
        Map<String, Object> classifier = createSimpleClassifier("my-test", "", TEST_NAMESPACE);
        getKafkaTopicByClassifier(SC_NOT_FOUND, classifier);
        createKafkaTopic(SC_CREATED, classifier);
        getKafkaTopicByClassifier(SC_OK, classifier);

        ConfigV2Req config = new ConfigV2Req(outerSpec);
        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");
        Map<String, Object> replies = helper.doRequest(request, Map.class, SC_OK);
        assertEquals("ok", replies.get("status"));
        getKafkaTopicByClassifier(SC_NOT_FOUND, classifier);
    }

    @Test
    void applyDeleteTenantTopicConfiguration() throws IOException {
        String configStr =
                "apiVersion: nc.maas.kafka/v2\n" +
                        "kind: tenant-topic-delete\n" +
                        "spec:\n" +
                        "  classifier: \n" +
                        "    name: tenant-topic-name\n" +
                        "    namespace: maas-it-test";

        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version("v1")
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("agreement-mgmt-core").
                                config(configStr).
                                build()
                        }
                )
                .build();
        String tenantTopicName = "tenant-topic-name";
        Map<String, Object> classifier = createSimpleClassifier(tenantTopicName, null, TEST_NAMESPACE);
        KafkaTenantTopicConfigRequest.Spec tenantTopicSpec = KafkaTenantTopicConfigRequest.Spec.builder()
                .classifier(classifier)
                .build();

        KafkaTenantTopicConfigResponse.SingleReply tenantTopicReply = createKafkaTenantTopic(SC_OK, tenantTopicSpec);
        assertEquals("ok", tenantTopicReply.getResult().getStatus());

        List<Map<String, Object>> tenants = List.of(Collections.singletonMap("externalId", "test-tenant"));
        applyTenants(SC_OK, tenants);
        getKafkaTopicByClassifier(SC_OK, createSimpleClassifier(tenantTopicName, "test-tenant", TEST_NAMESPACE));

        ConfigV2Req config = new ConfigV2Req(outerSpec);
        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");
        Map<String, Object> replies = helper.doRequest(request, Map.class, SC_OK);
        assertEquals("ok", replies.get("status"));
        getKafkaTopicByClassifier(SC_NOT_FOUND, createSimpleClassifier(tenantTopicName, "test-tenant", TEST_NAMESPACE));
    }

    @Test
    void applyDeleteTopicTemplateConfiguration() throws IOException {
        String configStr =
                "apiVersion: nc.maas.kafka/v2\n" +
                        "kind: topic-template-delete\n" +
                        "spec:\n" +
                        "  name: my-test-template";

        ConfigV2Req.Spec outerSpec = ConfigV2Req.Spec.builder()
                .namespace(TEST_NAMESPACE)
                .version("v1")
                .services(
                        new ConfigV2Req.Service[]{ConfigV2Req.Service.builder().serviceName("agreement-mgmt-core").
                                config(configStr).
                                build()
                        }
                )
                .build();
        assertKafkaTemplates(0);
        createKafkaTemplate(SC_OK, KafkaTemplateConfigRequest.Spec.builder().name("my-test-template").build());
        assertKafkaTemplates(1);

        ConfigV2Req config = new ConfigV2Req(outerSpec);
        Request request = helper.createYamlRequestV2(APPLY_CONFIG_V2_PATH, getMaasBasicAuth(), config, "POST");
        Map<String, Object> replies = helper.doRequest(request, Map.class, SC_OK);
        assertEquals("ok", replies.get("status"));
        assertKafkaTemplates(0);
    }
}
