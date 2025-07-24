package org.qubership.it.maas.kafka;

import lombok.extern.slf4j.Slf4j;
import org.apache.http.HttpStatus;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;
import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.kafka.*;

import java.io.IOException;
import java.util.*;

import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assumptions.assumeTrue;
import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;

@Slf4j
class KafkaTenantTopicsIT extends AbstractMaasWithInitsIT {
    @Test
    public void createTenantTopicsAndApplyTenants() throws IOException {
        //create tenant topic with template
        String templateName = "IT-test-template-" + UUID.randomUUID().toString();
        KafkaTemplateConfigRequest.Spec templateSpec = KafkaTemplateConfigRequest.Spec.builder()
                .name(templateName)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1200"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, templateSpec);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals("1200", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));

        String tenantTopicName1200 = "IT-test-tenant-topic-" + UUID.randomUUID().toString();
        Map<String, Object> tenantTopicClassifier = createSimpleClassifier(tenantTopicName1200);
        KafkaTenantTopicConfigRequest.Spec tenantTopicSpec = KafkaTenantTopicConfigRequest.Spec.builder()
                .classifier(tenantTopicClassifier)
                .template(templateName)
                .build();

        KafkaTenantTopicConfigResponse.SingleReply tenantTopicReply = createKafkaTenantTopic(200, tenantTopicSpec);
        assertEquals("ok", tenantTopicReply.getResult().getStatus());
        assertEquals(0, tenantTopicReply.getResult().getData().length);

        //create tenant topic without template
        String tenantTopicName1300 = "IT-test-tenant-topic-" + UUID.randomUUID().toString();
        KafkaTenantTopicConfigRequest.Spec tenantTopicSpec300 = KafkaTenantTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(tenantTopicName1300, null, TEST_NAMESPACE))
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1300"))
                .build();

        tenantTopicReply = createKafkaTenantTopic(200, tenantTopicSpec300);
        assertEquals("ok", tenantTopicReply.getResult().getStatus());
        assertEquals(0, tenantTopicReply.getResult().getData().length);

        //apply 2 tenants, expecting 2 topics for 2 responses for 2 tenants
        List<Map<String, Object>> tenants = Arrays.asList(Collections.singletonMap("externalId", "200"), Collections.singletonMap("externalId", "300"));
        SyncTenantsResponse[] responses = applyTenants(200, tenants);

        assertEquals(2, responses.length);

        SyncTenantsResponse respTenant200, respTenant300;
        if (responses[0].getTenant().get("externalId").equals("200")) {
            respTenant200 = responses[0];
            respTenant300 = responses[1];
        } else {
            respTenant300 = responses[1];
            respTenant200 = responses[0];
        }

        //check tenant with 200 id
        assertEquals("200", respTenant200.getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, respTenant200.getTenant().get("namespace"));
        assertEquals(2, respTenant200.getTopics().length);

        KafkaTopicResponse respTopic1200, respTopic1300;
        if (respTenant200.getTopics()[0].getName().contains(tenantTopicName1200)) {
            respTopic1200 = respTenant200.getTopics()[0];
            respTopic1300 = respTenant200.getTopics()[1];
        } else {
            respTopic1300 = respTenant200.getTopics()[1];
            respTopic1200 = respTenant200.getTopics()[0];
        }

        assertThat(respTopic1200.getName(), CoreMatchers.containsString(tenantTopicName1200));
        assertEquals("1200", respTopic1200.getActualSettings().getConfigs().get("flush.ms"));
        assertThat(respTopic1300.getName(), CoreMatchers.containsString(tenantTopicName1300));
        assertEquals("1300", respTopic1300.getActualSettings().getConfigs().get("flush.ms"));

        //check tenant with 300 id
        assertEquals("300", respTenant300.getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, respTenant300.getTenant().get("namespace"));
        assertEquals(2, respTenant300.getTopics().length);

        if (respTenant300.getTopics()[0].getName().contains(tenantTopicName1200)) {
            respTopic1200 = respTenant300.getTopics()[0];
            respTopic1300 = respTenant300.getTopics()[1];
        } else {
            respTopic1300 = respTenant300.getTopics()[1];
            respTopic1200 = respTenant300.getTopics()[0];
        }

        assertThat(respTopic1200.getName(), CoreMatchers.containsString(tenantTopicName1200));
        assertEquals("1200", respTopic1200.getActualSettings().getConfigs().get("flush.ms"));
        assertThat(respTopic1300.getName(), CoreMatchers.containsString(tenantTopicName1300));
        assertEquals("1300", respTopic1300.getActualSettings().getConfigs().get("flush.ms"));

        //adding 3 tenants, 2 already exist
        tenants = Arrays.asList(Collections.singletonMap("externalId", "200"), Collections.singletonMap("externalId", "300"), Collections.singletonMap("externalId", "400"));
        responses = applyTenants(200, tenants);
        assertEquals(1, responses.length);
        assertEquals("400", responses[0].getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, responses[0].getTenant().get("namespace"));
        assertEquals(2, responses[0].getTopics().length);

        if (responses[0].getTopics()[0].getName().contains(tenantTopicName1200)) {
            respTopic1200 = responses[0].getTopics()[0];
            respTopic1300 = responses[0].getTopics()[1];
        } else {
            respTopic1300 = responses[0].getTopics()[1];
            respTopic1200 = responses[0].getTopics()[0];
        }

        assertThat(respTopic1200.getName(), CoreMatchers.containsString(tenantTopicName1200));
        assertEquals("1200", respTopic1200.getActualSettings().getConfigs().get("flush.ms"));
        assertThat(respTopic1300.getName(), CoreMatchers.containsString(tenantTopicName1300));
        assertEquals("1300", respTopic1300.getActualSettings().getConfigs().get("flush.ms"));

        deleteTenantTopic(200, tenantTopicClassifier);
    }

    @Test
    public void applyTenantAfterTenantTopicDeclaration() throws IOException {
        List<Map<String, Object>> tenants = List.of(Collections.singletonMap("externalId", "200"));
        SyncTenantsResponse[] responses = applyTenants(200, tenants);

        assertEquals(1, responses.length);
        assertEquals("200", responses[0].getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, responses[0].getTenant().get("namespace"));
        assertEquals(0, responses[0].getTopics().length);


        //create tenant topic without template
        String tenantTopicName1300 = "IT-test-tenant-topic-" + UUID.randomUUID().toString();
        KafkaTenantTopicConfigRequest.Spec tenantTopicSpec1300 = KafkaTenantTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(tenantTopicName1300))
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1300"))
                .build();

        KafkaTenantTopicConfigResponse.SingleReply tenantTopicReply = createKafkaTenantTopic(200, tenantTopicSpec1300);
        assertEquals("ok", tenantTopicReply.getResult().getStatus());
        assertEquals(1, tenantTopicReply.getResult().getData().length);
        assertEquals("200", tenantTopicReply.getResult().getData()[0].getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, tenantTopicReply.getResult().getData()[0].getTenant().get("namespace"));
        assertEquals(1, tenantTopicReply.getResult().getData()[0].getTopics().length);
        assertThat(tenantTopicReply.getResult().getData()[0].getTopics()[0].getName(), CoreMatchers.containsString(tenantTopicName1300));
        assertEquals("1300", tenantTopicReply.getResult().getData()[0].getTopics()[0].getActualSettings().getConfigs().get("flush.ms"));
    }

    /*
    1. add tenant
    2. add topic with custom name to kafka
    3. add tenant topic with custom name, merge should be ok
    4. check created topic in maas and kafka
     */
    @Test
    public void applyTenantTopicWithNamePatternAndMerge() throws IOException {
        String name = "topic_name";
        String tenantId = "123";
        String finalTopicName = tenantId + "-" + name;

        //create topic in kafka
        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "There is no default kafka or incorrect auth mechanism. Skip tests");
        createKafkaTopic(kafkaProp, finalTopicName);

        //add tenant
        List<Map<String, Object>> tenants = List.of(Collections.singletonMap("externalId", tenantId));
        SyncTenantsResponse[] responses = applyTenants(200, tenants);
        assertEquals(1, responses.length);
        assertEquals(tenantId, responses[0].getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, responses[0].getTenant().get("namespace"));
        assertEquals(0, responses[0].getTopics().length);

        //create tenant topic
        String pattern = "{{tenantId}}-{{name}}";
        KafkaTenantTopicConfigRequest.Spec tenantTopicSpec = KafkaTenantTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(name))
                .name(pattern)
                .build();

        KafkaTenantTopicConfigResponse.SingleReply tenantTopicReply = createKafkaTenantTopic(200, tenantTopicSpec);
        assertEquals("ok", tenantTopicReply.getResult().getStatus());
        assertEquals(1, tenantTopicReply.getResult().getData().length);
        assertEquals(tenantId, tenantTopicReply.getResult().getData()[0].getTenant().get("externalId"));
        assertEquals(TEST_NAMESPACE, tenantTopicReply.getResult().getData()[0].getTenant().get("namespace"));
        assertEquals(1, tenantTopicReply.getResult().getData()[0].getTopics().length);
        assertEquals(tenantTopicReply.getResult().getData()[0].getTopics()[0].getName(), finalTopicName);

        //check
        KafkaTopicResponse topic = getKafkaTopicByClassifier(HttpStatus.SC_OK, createSimpleClassifier(name, tenantId));
        assertEquals(finalTopicName, topic.getName());

        //topic now in maas and will be deleted by namespace
    }
}
