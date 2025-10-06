package org.qubership.it.maas.kafka;

import lombok.extern.slf4j.Slf4j;
import org.apache.commons.lang3.StringUtils;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;
import org.opentest4j.AssertionFailedError;
import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.TmfErrorResponse;
import org.qubership.it.maas.entity.kafka.*;

import java.io.IOException;
import java.util.Collections;
import java.util.Map;
import java.util.UUID;

import static org.junit.jupiter.api.Assertions.*;
import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE_2_BG;

@Slf4j
class KafkaTemplatesAndLazyTopicsIT extends AbstractMaasWithInitsIT {

    @Test
    public void createTemplate() throws IOException {
        String name = "IT-test-template-" + UUID.randomUUID().toString();
        KafkaTemplateConfigRequest.Spec spec = KafkaTemplateConfigRequest.Spec.builder()
                .name(name)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, spec);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(name, reply.getRequest().getSpec().getName());
        assertEquals("1", reply.getRequest().getSpec().getNumPartitions().toString());
        assertEquals("1", reply.getRequest().getSpec().getReplicationFactor().toString());
        assertEquals("1001", reply.getRequest().getSpec().getConfigs().get("flush.ms"));
        assertEquals(name, reply.getResult().getData().getName());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getReplicationFactor().toString());
        assertEquals("1001", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));
        assertNull(reply.getResult().getData().getPreviousSettings());

        reply = createKafkaTemplate(200, spec);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(name, reply.getResult().getData().getName());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getReplicationFactor().toString());
        assertEquals("1001", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));
        assertEquals("1", reply.getResult().getData().getPreviousSettings().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getPreviousSettings().getReplicationFactor().toString());
        assertEquals("1001", reply.getResult().getData().getPreviousSettings().getConfigs().get("flush.ms"));
    }

    @Test
    public void createAndChangeTemplate() throws IOException {
        String name = "IT-test-template-" + UUID.randomUUID().toString();
        KafkaTemplateConfigRequest.Spec spec1 = KafkaTemplateConfigRequest.Spec.builder()
                .name(name)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, spec1);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(name, reply.getResult().getData().getName());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getReplicationFactor().toString());
        assertEquals("1001", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));
        assertNull(reply.getResult().getData().getPreviousSettings());

        KafkaTemplateConfigRequest.Spec spec2 = KafkaTemplateConfigRequest.Spec.builder()
                .name(name)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "2002"))
                .build();
        reply = createKafkaTemplate(200, spec2);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(name, reply.getResult().getData().getName());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getCurrentSettings().getReplicationFactor().toString());
        assertEquals("2002", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));
        assertEquals("1", reply.getResult().getData().getPreviousSettings().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getPreviousSettings().getReplicationFactor().toString());
        assertEquals("1001", reply.getResult().getData().getPreviousSettings().getConfigs().get("flush.ms"));
    }

    @Test
    public void createTopicByTemplate() throws IOException {
        String templateName = "IT-test-template-" + UUID.randomUUID().toString();
        KafkaTemplateConfigRequest.Spec spec1 = KafkaTemplateConfigRequest.Spec.builder()
                .name(templateName)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, spec1);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(templateName, reply.getResult().getData().getName());

        String topicName = "KafkaTopicBasicOperationsIT." + UUID.randomUUID().toString();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .name(topicName)
                .classifier(createSimpleClassifier(topicName))
                .template(templateName)
                .build();

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, request);
        assertEquals(topicName, topic.getName());

        topic = createKafkaTopic(HttpStatus.SC_OK, request);
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertEquals(topicName, topic.getName());
        assertEquals(Integer.valueOf(1), topic.getRequestedSettings().getNumPartitions());
        assertEquals(Integer.valueOf(1), topic.getActualSettings().getNumPartitions());
        assertEquals("1001", topic.getRequestedSettings().getConfigs().get("flush.ms"));
        assertEquals("1001", topic.getActualSettings().getConfigs().get("flush.ms"));
    }

    @Test
    public void createTopicWithNonExistingTemplate() throws IOException {
        String topicName = "KafkaTopicBasicOperationsIT." + UUID.randomUUID();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .name(topicName)
                .classifier(createSimpleClassifier(topicName))
                .template("no-exists-template")
                .build();

        createKafkaTopic(HttpStatus.SC_BAD_REQUEST, request, TmfErrorResponse.class);
    }

    @Test
    public void createTopicByTemplateAndChangeTemplate() throws IOException {
        String templateName = "IT-test-template-" + UUID.randomUUID().toString();
        KafkaTemplateConfigRequest.Spec spec1 = KafkaTemplateConfigRequest.Spec.builder()
                .name(templateName)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, spec1);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(templateName, reply.getResult().getData().getName());

        String topicName = "KafkaTopicBasicOperationsIT." + UUID.randomUUID().toString();
        KafkaTopicRequest request = KafkaTopicRequest.builder()
                .name(topicName)
                .classifier(createSimpleClassifier(topicName))
                .template(templateName)
                .configs(Collections.singletonMap("flush.ms", "2002"))
                .build();

        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED, request);
        assertEquals(topicName, topic.getName());

        topic = getKafkaTopicByClassifier(HttpStatus.SC_OK, createSimpleClassifier(topicName));
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertEquals(topicName, topic.getName());
        assertEquals(Integer.valueOf(1), topic.getRequestedSettings().getNumPartitions());
        assertEquals(Integer.valueOf(1), topic.getActualSettings().getNumPartitions());
        assertEquals("1001", topic.getRequestedSettings().getConfigs().get("flush.ms"));
        assertEquals("1001", topic.getActualSettings().getConfigs().get("flush.ms"));


        KafkaTemplateConfigRequest.Spec spec2 = KafkaTemplateConfigRequest.Spec.builder()
                .name(templateName)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "2002"))
                .build();
        reply = createKafkaTemplate(200, spec2);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(templateName, reply.getResult().getData().getName());
        assertEquals("2002", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));
        assertEquals("1001", reply.getResult().getData().getPreviousSettings().getConfigs().get("flush.ms"));
        //assertEquals(topicName, reply.getResult().getData().getUpdatedTopics()[0].get("name"));

        topic = getKafkaTopicByClassifier(HttpStatus.SC_OK, createSimpleClassifier(topicName));
        assertEquals(topicName, topic.getName());
        assertEquals(Integer.valueOf(1), topic.getRequestedSettings().getNumPartitions());
        assertEquals(Integer.valueOf(1), topic.getActualSettings().getNumPartitions());
        assertEquals("2002", topic.getRequestedSettings().getConfigs().get("flush.ms"));
        assertEquals("2002", topic.getActualSettings().getConfigs().get("flush.ms"));
    }

    @Test
    public void createUpdateLazyTopicWithoutTemplate() throws IOException {
        String name = "IT-test-lazy-topic-" + UUID.randomUUID().toString();
        KafkaLazyTopicConfigRequest.Spec spec = KafkaLazyTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(name))
                .numPartitions(1)
                .replicationFactor("1")
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();
        KafkaLazyTopicConfigResponse.SingleReply reply = createKafkaLazyTopic(200, spec);

        assertEquals("ok", reply.getResult().getStatus());
        try {
            assertEquals(createSimpleClassifier(name), reply.getResult().getData().getClassifier());
        } catch (AssertionFailedError err) {
            assertEquals(createSimpleClassifier(name, "", TEST_NAMESPACE_2_BG), reply.getResult().getData().getClassifier());
        }
        assertEquals("1", reply.getResult().getData().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getReplicationFactor());
        assertEquals("1001", reply.getResult().getData().getConfigs().get("flush.ms"));

        KafkaLazyTopicConfigRequest.Spec spec2 = KafkaLazyTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(name))
                .numPartitions(1)
                .replicationFactor("1")
                .configs(Collections.singletonMap("flush.ms", "1022"))
                .build();
        reply = createKafkaLazyTopic(200, spec2);
        assertEquals("ok", reply.getResult().getStatus());
        try {
            assertEquals(createSimpleClassifier(name), reply.getResult().getData().getClassifier());
        } catch (AssertionFailedError err) {
            assertEquals(createSimpleClassifier(name, "", TEST_NAMESPACE_2_BG), reply.getResult().getData().getClassifier());
        }
        assertEquals("1", reply.getResult().getData().getNumPartitions().toString());
        assertEquals("1", reply.getResult().getData().getReplicationFactor());
        assertEquals("1022", reply.getResult().getData().getConfigs().get("flush.ms"));
    }

    @Test
    public void createApplyLazyTopicWithDiffTenantsByTemplate() throws IOException {
        String templateName = "IT-test-template-" + UUID.randomUUID();
        KafkaTemplateConfigRequest.Spec templateSpec = KafkaTemplateConfigRequest.Spec.builder()
                .name(templateName)
                .numPartitions(1)
                .replicationFactor(1)
                .configs(Collections.singletonMap("flush.ms", "1201"))
                .build();

        KafkaTemplateConfigResponse.SingleReply reply = createKafkaTemplate(200, templateSpec);
        assertEquals("ok", reply.getResult().getStatus());
        assertEquals("1201", reply.getResult().getData().getCurrentSettings().getConfigs().get("flush.ms"));

        String lazyTopicName = "IT-test-lazy-topic" + UUID.randomUUID().toString();
        KafkaLazyTopicConfigRequest.Spec lazyTopicSpec = KafkaLazyTopicConfigRequest.Spec.builder()
                .classifier(createSimpleClassifier(lazyTopicName))
                .template(templateName)
                .build();
        KafkaLazyTopicConfigResponse.SingleReply lazyTopicReply = createKafkaLazyTopic(200, lazyTopicSpec);
        assertEquals("ok", lazyTopicReply.getResult().getStatus());
        try {
            assertEquals(createSimpleClassifier(lazyTopicName), lazyTopicReply.getResult().getData().getClassifier());
        } catch (AssertionFailedError err) {
            assertEquals(createSimpleClassifier(lazyTopicName, "", TEST_NAMESPACE_2_BG), lazyTopicReply.getResult().getData().getClassifier());
        }
        assertEquals(templateName, lazyTopicReply.getResult().getData().getTemplate());

        KafkaTopicResponse topic = applyLazyTopic(201, createSimpleClassifier(lazyTopicName));
        assertTrue(topic.getName().startsWith("maas." + TEST_NAMESPACE));
        assertEquals(TEST_NAMESPACE, topic.getNamespace());
        assertTrue(topic.getName().contains("IT-test-lazy-topic"));
        assertTrue(topic.getName().contains("it-test"));
        assertNotNull(topic.getInstance());
        assertFalse(StringUtils.isBlank(topic.getInstance()));
        assertNotNull(topic.getAddresses());
        assertFalse(topic.getAddresses().isEmpty());
        assertEquals(templateName, topic.getTemplate());

        Map<String, Object> classifier1 = createSimpleClassifier(lazyTopicName, "123");
        Map<String, Object> classifier2 = createSimpleClassifier(lazyTopicName, "456");

        topic = applyLazyTopic(HttpStatus.SC_CREATED, classifier1);
        assertEquals(topic.getClassifier(), classifier1);
        assertEquals("1201", topic.getActualSettings().getConfigs().get("flush.ms"));
        applyLazyTopic(HttpStatus.SC_OK, classifier1);

        topic = applyLazyTopic(HttpStatus.SC_CREATED, classifier2);
        assertEquals(topic.getClassifier(), classifier2);
        assertEquals("1201", topic.getActualSettings().getConfigs().get("flush.ms"));
    }

    @Test
    public void conflictLazyTopicWithoutTemplate() throws IOException {
        String lazyTopicName = "IT-test-lazy-topic-" + UUID.randomUUID();
        String tenantId = "123";
        Map<String, Object> classifier1 = createSimpleClassifier(lazyTopicName, tenantId);
        Map<String, Object> classifier2 = createSimpleClassifier(lazyTopicName);

        KafkaLazyTopicConfigRequest.Spec spec = KafkaLazyTopicConfigRequest.Spec.builder()
                .classifier(classifier1)
                .numPartitions(1)
                .replicationFactor("1")
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();
        KafkaLazyTopicConfigResponse.SingleReply reply = createKafkaLazyTopic(200, spec);
        assertEquals("ok", reply.getResult().getStatus());

        spec = KafkaLazyTopicConfigRequest.Spec.builder()
                .classifier(classifier2)
                .numPartitions(1)
                .replicationFactor("1")
                .configs(Collections.singletonMap("flush.ms", "1001"))
                .build();
        reply = createKafkaLazyTopic(200, spec);
        assertEquals("ok", reply.getResult().getStatus());

        applyLazyTopic(HttpStatus.SC_CONFLICT, classifier1, TmfErrorResponse.class);
    }
}
