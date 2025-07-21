package com.netcracker.it.maas;

import com.netcracker.it.maas.entity.ConfigV2Resp;
import com.netcracker.it.maas.entity.kafka.KafkaInstance;
import com.netcracker.it.maas.entity.kafka.KafkaTopicResponse;
import com.netcracker.it.maas.entity.rabbit.VhostConfigResponse;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import com.rabbitmq.client.AMQP;
import com.rabbitmq.client.Channel;
import com.rabbitmq.client.Connection;
import lombok.extern.slf4j.Slf4j;
import org.apache.http.HttpStatus;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;

import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.Map;

import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.apache.http.HttpStatus.*;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assumptions.assumeFalse;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

@Slf4j
class ConfigV2IT extends AbstractMaasWithInitsIT {

    @Test
    public void configV2BothKafkaRabbitIT() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec: \n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services: \n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            entities:\n" +
                        "                exchanges:\n" +
                        "                - name: non-vers-e1\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                  type: direct\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.kafka/v1\n" +
                        "        kind: topic\n" +
                        "        spec:\n" +
                        "            classifier: \n" +
                        "                name: configV2TopicIT\n" +
                        "                namespace: " + TEST_NAMESPACE + "\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "                name: vers-test\n" +
                        "                namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        ConfigV2Resp reply;
        reply = applyConfigV2(SC_OK, cfg);

        //check Rabbit

        Map<String, Object> classifier = createSimpleClassifier("vers-test");
        VirtualHostResponse virtualHost = createVirtualHost(SC_OK, classifier);

        String version;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message;

            //candidate route
            version = "v1";
            message = "some message to " + version;
            channel.basicPublish("e1", "key",
                    new AMQP.BasicProperties.Builder()
                            .headers(Collections.singletonMap("version", version))
                            .build(),
                    message.getBytes());

            //checking that no such queue
            String resp = new String(channel.basicGet("q1-v1", false).getBody(), StandardCharsets.UTF_8);
            assertEquals(message, resp);
        }

        //check Kafka
        Map<String, Object> classifierKafka = createSimpleClassifier("configV2TopicIT");
        KafkaTopicResponse topic = getKafkaTopicByClassifier(SC_OK, classifierKafka);
        assertEquals(topic.getClassifier().get("name"), classifierKafka.get("name"));
    }

    @Test
    public void configV2EmptyMssIT() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "  - {serviceName: security-scripts-cli, config: ''}\n" +
                        "  - {serviceName: key-manager, config: ''}\n" +
                        "  - {serviceName: identity-provider, config: ''}"
        ;

        applyConfigV2(SC_OK, cfg);
    }

    @Test
    public void configV2CheckParsingErrIT() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: *"
        ;

        ConfigV2Resp resp = applyConfigV2(SC_BAD_REQUEST, cfg);
        assertThat(resp.getError(), CoreMatchers.containsString("bad input, check correctness of your YAML"));
    }

    @Test
    public void configV2CheckInternalMsErrIT() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec: \n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services: \n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            entities:\n" +
                        "                exchanges:\n" +
                        "                - name: non-vers-e1\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                  type: direct\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.kafka/v1\n" +
                        "        kind: topic\n" +
                        "        spec:\n" +
                        "            classifier: \n" +
                        "                name: configV2TopicIT\n" +
                        "                namespace: " + TEST_NAMESPACE + "\n" +
                        "            externallyManaged: true\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "                name: vers-test\n" +
                        "                namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        ConfigV2Resp resp = applyConfigV2(SC_INTERNAL_SERVER_ERROR, cfg);
        assertTrue(resp.getError().contains("server error for internal config of microservice") || resp.getError().contains("server error for internal config of ms"));
    }


    @Test
    public void kafkaInstanceDesignatorNameInConfig() throws Exception {
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
                        "            name: orders\n" +
                        "          instance: " + firstInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645\n" +
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
        reply = applyConfigV2(SC_OK, cfg);

        assertEquals("ok", reply.getStatus());

        //check Kafka
        Map<String, Object> classifierKafka = createSimpleClassifier("orders", "", TEST_NAMESPACE);
        KafkaTopicResponse topic = getKafkaTopicByClassifier(SC_OK, classifierKafka);
        assertEquals(topic.getClassifier().get("name"), classifierKafka.get("name"));
        assertEquals(firstInstance.getId(), topic.getInstance());

        Map<String, Object> classifierKafkaTenant = createSimpleClassifier("no-orders", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        KafkaTopicResponse topic2 = createKafkaTopic(HttpStatus.SC_CREATED, classifierKafkaTenant);
        assertEquals(topic2.getClassifier().get("name"), classifierKafkaTenant.get("name"));
        assertEquals(secondInstance.getId(), topic2.getInstance());

        deleteKafkaInstanceDesignator(SC_OK);
    }


    @Test
    public void kafkaInstanceDesignatorUpdate() throws Exception {
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
                        "            name: orders\n" +
                        "          instance: " + firstInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "  services: \n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "          apiVersion: nc.maas.kafka/v1\n" +
                        "          kind: topic\n" +
                        "          spec:\n" +
                        "             classifier: { name: configV2TopicIT, namespace: " + TEST_NAMESPACE + ", tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645 }\n" +
                        "             "
        ;

        ConfigV2Resp reply;
        reply = applyConfigV2(SC_OK, cfg);

        assertEquals("ok", reply.getStatus());

        //check Kafka
        Map<String, Object> classifierKafka = createSimpleClassifier("configV2TopicIT", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        KafkaTopicResponse topicTenant = getKafkaTopicByClassifier(SC_OK, classifierKafka);
        assertEquals(topicTenant.getClassifier().get("name"), classifierKafka.get("name"));
        assertEquals(secondInstance.getId(), topicTenant.getInstance());

        Map<String, Object> classifierKafka2 = createSimpleClassifier("default-1", "", TEST_NAMESPACE);
        KafkaTopicResponse topicDefault = createKafkaTopic(HttpStatus.SC_CREATED, classifierKafka2);
        assertEquals(topicDefault.getClassifier().get("name"), classifierKafka2.get("name"));
        assertEquals(firstInstance.getId(), topicDefault.getInstance());

        Map<String, Object> classifierName = createSimpleClassifier("orders", "", TEST_NAMESPACE);
        KafkaTopicResponse topicName = createKafkaTopic(HttpStatus.SC_CREATED, classifierName);
        assertEquals(topicName.getClassifier().get("name"), classifierName.get("name"));
        assertEquals(firstInstance.getId(), topicName.getInstance());

        //update

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
                        "        defaultInstance: " + secondInstance.getId() + "\n" +
                        "        selectors:\n" +
                        "        - classifierMatch:\n" +
                        "            name: orders-2\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645\n" +
                        "          instance: " + firstInstance.getId() + "\n"
        ;

        reply = applyConfigV2(SC_OK, cfg);
        assertEquals("ok", reply.getStatus());

        //check Kafka, instance should stay the same
        classifierKafka = createSimpleClassifier("configV2TopicIT", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        topicTenant = getKafkaTopicByClassifier(SC_OK, classifierKafka);
        assertEquals(topicTenant.getClassifier().get("name"), classifierKafka.get("name"));
        assertEquals(secondInstance.getId(), topicTenant.getInstance());

        classifierKafka2 = createSimpleClassifier("default-2", "", TEST_NAMESPACE);
        topicDefault = createKafkaTopic(HttpStatus.SC_CREATED, classifierKafka2);
        assertEquals(topicDefault.getClassifier().get("name"), classifierKafka2.get("name"));
        assertEquals(secondInstance.getId(), topicDefault.getInstance());

        classifierName = createSimpleClassifier("orders-2", "", TEST_NAMESPACE);
        topicName = createKafkaTopic(HttpStatus.SC_CREATED, classifierName);
        assertEquals(topicName.getClassifier().get("name"), classifierName.get("name"));
        assertEquals(secondInstance.getId(), topicName.getInstance());

        deleteKafkaInstanceDesignator(SC_OK);
    }

    @Test
    public void deleteDefaultInstanceKafka() throws Exception {
        KafkaInstance[] kafkaInstances = getKafkaInstances();
        assumeTrue(kafkaInstances.length >= 2, "deleteDefaultInstanceKafka test requires at least two instances. Skip test");

        KafkaInstance defaultInstance, secondInstance;
        if (kafkaInstances[0].getIsDefault()) {
            defaultInstance = kafkaInstances[0];
            secondInstance = kafkaInstances[1];
        } else {
            defaultInstance = kafkaInstances[1];
            secondInstance = kafkaInstances[0];
        }

        deleteKafkaInstance(defaultInstance, SC_BAD_REQUEST);
        deleteKafkaInstance(secondInstance, SC_OK);
        deleteKafkaInstance(defaultInstance, SC_OK);

        KafkaInstance createdDefaultInstance = createKafkaInstance(defaultInstance);
        assertTrue(createdDefaultInstance.getIsDefault());
        KafkaInstance createdSecondInstance = createKafkaInstance(secondInstance);
        assumeFalse(createdSecondInstance.getIsDefault());
    }

    @Test
    public void baseNamespaceKafka() throws Exception {
        String baseNamespace = "baseNamespace";
        String satelliteNamespace1 = "satelliteNamespace1";
        String satelliteNamespace2 = "satelliteNamespace2";
        try {
            String cfg =    "apiVersion: nc.maas.config/v2\n" +
                            "kind: config\n" +
                            "spec: \n" +
                            "  version: v1\n" +
                            "  namespace: " + satelliteNamespace1 + "\n" +
                            "  base-namespace: " + baseNamespace + "\n" +
                            "  services: \n" +
                            "    - serviceName: order-processor\n" +
                            "      config: |+\n" +
                            "        ---\n" +
                            "        apiVersion: nc.maas.kafka/v1\n" +
                            "        kind: topic\n" +
                            "        spec:\n" +
                            "            classifier: \n" +
                            "                name: configV2TopicIT\n" +
                            "                namespace: " + satelliteNamespace1 + "\n";

            applyConfigV2(SC_OK, cfg);

            cfg =
                    "apiVersion: nc.maas.config/v2\n" +
                            "kind: config\n" +
                            "spec: \n" +
                            "  version: v1\n" +
                            "  namespace: " + satelliteNamespace2 + "\n" +
                            "  base-namespace: " + baseNamespace + "\n" +
                            "  services: \n" +
                            "    - serviceName: order-executor\n" +
                            "      config: \n";

            //just to add second sattelite
            applyConfigV2(SC_OK, cfg);

            //check Kafka
            Map<String, Object> classifierKafka = createSimpleClassifier("configV2TopicIT", "", satelliteNamespace1);
            KafkaTopicResponse topic = getKafkaTopicByClassifierWithNamespace(SC_OK, classifierKafka, satelliteNamespace2);
            assertEquals(topic.getClassifier().get("name"), classifierKafka.get("name"));
        } finally {
            deleteNamespace(satelliteNamespace1);
            deleteNamespace(satelliteNamespace2);
            deleteNamespace(baseNamespace);
        }
    }

    @Test
    public void baseNamespaceRabbit() throws Exception {
        String baseNamespace = "baseNamespace";
        String satelliteNamespace1 = "satelliteNamespace1";
        String satelliteNamespace2 = "satelliteNamespace2";
        try {
            String cfg =    "apiVersion: nc.maas.config/v2\n" +
                            "kind: config\n" +
                            "spec: \n" +
                            "  version: v1\n" +
                            "  namespace: " + satelliteNamespace1 + "\n" +
                            "  base-namespace: " + baseNamespace + "\n" +
                            "  services: \n" +
                            "    - serviceName: order-processor\n" +
                            "      config: |+\n" +
                            "        ---\n" +
                            "        apiVersion: nc.maas.rabbit/v2\n" +
                            "        kind: vhost\n" +
                            "        spec:\n" +
                            "            classifier: \n" +
                            "                name: baseNamespaceRabbitTest\n" +
                            "                namespace: " + satelliteNamespace1 + "\n" +
                            "            entities:\n" +
                            "                exchanges:\n" +
                            "                - name: e1\n";

            applyConfigV2(SC_OK, cfg);

            cfg =
                    "apiVersion: nc.maas.config/v2\n" +
                            "kind: config\n" +
                            "spec: \n" +
                            "  version: v1\n" +
                            "  namespace: " + satelliteNamespace2 + "\n" +
                            "  base-namespace: " + baseNamespace + "\n" +
                            "  services: \n" +
                            "    - serviceName: order-executor\n" +
                            "      config: \n";

            //just to add second sattelite
            applyConfigV2(SC_OK, cfg);

            //check Rabbit
            Map<String, Object> classifier = createSimpleClassifier("baseNamespaceRabbitTest", "", satelliteNamespace1);
            VhostConfigResponse vhost = getRabbitVhostByClassifierWithNamespace(SC_OK, classifier, satelliteNamespace1);
            assertThat(vhost.getVhost().getCnn(), CoreMatchers.containsString(String.format("%s.%s", satelliteNamespace1, "baseNamespaceRabbitTest")));
            vhost = getRabbitVhostByClassifierWithNamespace(SC_OK, classifier, satelliteNamespace2);
            assertThat(vhost.getVhost().getCnn(), CoreMatchers.containsString(String.format("%s.%s", satelliteNamespace1, "baseNamespaceRabbitTest")));
        } finally {
            deleteNamespace(satelliteNamespace1);
            deleteNamespace(satelliteNamespace2);
            deleteNamespace(baseNamespace);
        }
    }
}


