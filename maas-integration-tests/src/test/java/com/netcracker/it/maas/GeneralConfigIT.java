package com.netcracker.it.maas;

import com.netcracker.it.maas.entity.kafka.*;
import com.netcracker.it.maas.entity.rabbit.RabbitConfigReq;
import com.netcracker.it.maas.entity.rabbit.RabbitConfigResp;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.*;

import static com.netcracker.it.maas.MaasITHelper.APPLY_CONFIG_PATH;
import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.*;

@Slf4j
class GeneralConfigIT extends AbstractMaasWithInitsIT {

    @Test
    public void createSingleExchangeAndTopic() throws IOException {
        List<Object> yamls = new ArrayList<>();
        Map<String, Object> classifier = createSimpleClassifier("ApplyConfigIT", "it-test");

        //rabbit
        Map<String, Object> exchange = new HashMap<>();
        exchange.put("name", "ApplyConfigIT-exchange");
        Map<String, Object>[] exchanges = new HashMap[1];
        exchanges[0] = exchange;

        RabbitConfigReq rabbitConfigReq = RabbitConfigReq.builder()
                .apiVersion(API_RABBIT_VERSION)
                .kind(KIND_VHOST)
                .spec(RabbitConfigReq.Spec.builder()
                        .classifier(classifier)
                        .instanceId(DEFAULT_RABBIT_INSTANCE)
                        .entities(RabbitConfigReq.RabbitEntities.builder()
                                .exchanges(exchanges)
                                .build())
                        .build())
                .build();

        yamls.add(rabbitConfigReq);

        //kafka
        final String name = "KafkaTopicBasicOperationsIT." + UUID.randomUUID();
        KafkaConfigReq kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec(name, classifier))
                .build();

        yamls.add(kafkaConfigReq);

        Request request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), yamls, "POST");
        RabbitConfigResp.SingleReply[] replies = helper.doRequest(request, RabbitConfigResp.SingleReply[].class);

        for(RabbitConfigResp.SingleReply reply : replies){
            assertEquals("ok", reply.getResult().getStatus());
            assertNull(reply.getResult().getError());
        }

    }

    @Test
    public void deleteNamespaceWithAllEntities() throws IOException {

        //create objects in namespace
        //topic
        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED);
        assertEquals(TEST_NAMESPACE, topic.getNamespace());

        //template and topic by template
        String templateName = "IT-test-template-" + UUID.randomUUID();
        KafkaTemplateConfigRequest.Spec spec1 = KafkaTemplateConfigRequest.Spec.builder()
                .name(templateName)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, spec1);
        assertEquals(TEST_NAMESPACE, reply.getResult().getData().getNamespace());

        String topicName = "KafkaTopicBasicOperationsIT." + UUID.randomUUID();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .name(topicName)
                .classifier(createSimpleClassifier(topicName))
                .template(templateName)
                .build();

        KafkaTopicResponse topicResp = createKafkaTopic(HttpStatus.SC_CREATED, request);
        assertEquals(TEST_NAMESPACE, topicResp.getNamespace());

        //lazy-topics
        String lazyTopicName = "IT-test-lazy-topic-" + UUID.randomUUID();
        KafkaLazyTopicConfigRequest.Spec spec = KafkaLazyTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(lazyTopicName))
                .numPartitions(1)
                .replicationFactor("1")
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();
        KafkaLazyTopicConfigResponse.SingleReply lazyTopic = createKafkaLazyTopic(200, spec);
        assertEquals("ok", lazyTopic.getResult().getStatus());

        //tenants and tenant-topics
        String tenantTopicName1300 = "IT-test-tenant-topic-" + UUID.randomUUID();
        KafkaTenantTopicConfigRequest.Spec tenantTopicSpec300 = KafkaTenantTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(tenantTopicName1300))
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1300"))
                .build();

        KafkaTenantTopicConfigResponse.SingleReply tenantTopicReply = createKafkaTenantTopic(200, tenantTopicSpec300);
        assertEquals("ok", tenantTopicReply.getResult().getStatus());
        assertEquals(0, tenantTopicReply.getResult().getData().length);

        List<Map<String, Object>> tenants = List.of(Map.of("externalId", "300"));
        SyncTenantsResponse[] responses = applyTenants(200, tenants);
        assertEquals(1, responses.length);
        assertEquals("300", responses[0].getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, responses[0].getTenant().get("namespace"));
        assertEquals(1, responses[0].getTopics().length);

        //vhosts
        VirtualHostResponse virtualHost = createVirtualHost(201);
        assertNotNull(virtualHost.getUsername());

        //delete namespace
        deleteNamespace(TEST_NAMESPACE);
        initAccount(agentAccount, TEST_NAMESPACE);

        //check no objects in namespace left
        //topics
        Map<String, Object> topicsByNamespace = Collections.singletonMap("namespace", TEST_NAMESPACE);
        List<KafkaTopicResponse> response = searchKafkaTopics(topicsByNamespace, HttpStatus.SC_OK);
        assertTrue(response == null || response.isEmpty());

        //templates
        assertKafkaTemplates(0);

        //lazy-topics, tenant-topics;
        assertKafkaLazyTopics(0);
        assertKafkaTenantTopics(0);

        //tenants
        assertKafkaTenants(0);
    }
}
