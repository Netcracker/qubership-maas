package com.netcracker.it.maas.rabbit;

import com.netcracker.it.maas.bg2.BGState;
import com.netcracker.it.maas.bg2.BgNamespace;
import com.netcracker.it.maas.bg2.BgResponse;
import com.netcracker.it.maas.entity.ConfigV2Resp;
import com.netcracker.it.maas.entity.rabbit.CpMessage;
import com.netcracker.it.maas.entity.rabbit.MsConfig;
import com.netcracker.it.maas.entity.rabbit.RabbitConfigValidation;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import com.rabbitmq.client.*;
import lombok.SneakyThrows;
import lombok.extern.slf4j.Slf4j;
import lombok.val;
import okhttp3.Request;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.Date;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

import static com.netcracker.it.maas.MaasITHelper.*;
import static org.apache.http.HttpStatus.*;
import static org.junit.jupiter.api.Assertions.*;

@Slf4j
class ApplyRabbitBlueGreenConfigIT extends RabbitTest {

    ConfigV2Resp reply;

    @Test
    public void installCandidateAndUpdate() throws Exception {
        Map<String, Object>[] exchanges = new HashMap[1];
        exchanges[0] = getExchange();

        Map<String, Object>[] queues = new HashMap[1];
        queues[0] = getQueue();

        Map<String, Object>[] bindings = new HashMap[1];
        bindings[0] = getBinding();

        MsConfig[] msConfigs = new MsConfig[]{
                new MsConfig("ms1", exchanges, null, null),
                new MsConfig("ms2", null, queues, bindings),
        };

        String version = "v1";
        reply = applyMsConfigs(200, msConfigs, version);
        assertEquals("ok", reply.getStatus());

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        log.info("Created virtual host {}", virtualHost);
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message, resp;

            //candidate route
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "v1");
            assertEquals(message, resp);

            //default route
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "v1");
            assertEquals(message, resp);

            channel.close();
        }

        exchanges = new HashMap[1];
        Map<String, Object> exchange = new HashMap<>();
        exchange.put("name", "Ea");
        exchange.put("type", "fanout");
        exchanges[0] = exchange;

        queues = new HashMap[2];
        Map<String, Object> queueA = new HashMap<>();
        queueA.put("name", "Qb");
        Map<String, Object> queueB = new HashMap<>();
        queueB.put("name", "Qc");
        queues[0] = queueA;
        queues[1] = queueB;

        bindings = new HashMap[2];
        Map<String, Object> bindingA = new HashMap<>();
        bindingA.put("source", "Ea");
        bindingA.put("destination", "Qb");
        bindingA.put("routing_key", "route");
        Map<String, Object> bindingB = new HashMap<>();
        bindingB.put("source", "Ea");
        bindingB.put("destination", "Qc");
        bindings[0] = bindingA;
        bindings[1] = bindingB;

        msConfigs = new MsConfig[]{
                new MsConfig("ms1", exchanges, null, null),
                new MsConfig("ms2", null, queues, bindings),
        };

        version = "v1";
        reply = applyMsConfigs(200, msConfigs, version);
        assertEquals("ok", reply.getStatus());

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message;

            //candidate route
            version = "v1";
            message = "some message to " + version;
            channel.basicPublish("Ea", "route",
                    new AMQP.BasicProperties.Builder()
                            .headers(Collections.singletonMap("version", version))
                            .build(),
                    message.getBytes());
            String resp = "";
            try {
                channel.basicGet("Qa-v1", false);
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
        }

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message;
            version = "v1";
            message = "some message to " + version;

            GetResponse response = channel.basicGet("Qb-v1", false);
            String stringResponse = new String(response.getBody(), StandardCharsets.UTF_8);
            assertEquals(message, stringResponse);

            response = channel.basicGet("Qc-v1", false);
            stringResponse = new String(response.getBody(), StandardCharsets.UTF_8);
            assertEquals(message, stringResponse);

            channel.close();
        }
    }

    @Test
    public void rolloutAndPromoteAndInstallCandidate() throws Exception {
        Map<String, Object>[] exchanges = new HashMap[1];
        exchanges[0] = getExchange();

        Map<String, Object>[] queues = new HashMap[1];
        queues[0] = getQueue();

        Map<String, Object>[] bindings = new HashMap[1];
        bindings[0] = getBinding();

        MsConfig[] msConfigs = new MsConfig[]{
                new MsConfig("ms1", exchanges, null, null),
                new MsConfig("ms2", null, queues, bindings),
        };

        String version = "v1";
        reply = applyMsConfigs(200, msConfigs, version);
        assertEquals("ok", reply.getStatus());

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        log.info("Created virtual host {}", virtualHost);
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            channel.basicQos(1);
            String message, resp;

            //check route
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v1");
            assertEquals(message, resp);

            //check default
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v1");
            assertEquals(message, resp);

            //install candidate
            version = "v2";
            reply = applyMsConfigs(200, msConfigs, version);
            assertEquals("ok", reply.getStatus());

            //check route
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v2");
            assertEquals(message, resp);

            //check default
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v1");
            assertEquals(message, resp);

            //check default
            version = "";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v1");
            assertEquals(message, resp);

            //check default
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v1");
            assertEquals(message, resp);

            //check route
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v1");
            assertEquals(message, resp);

            //check route
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v2");
            assertEquals(message, resp);

            //promote
            changeActiveVersion(new CpMessage[]{
                    new CpMessage("v1", "LEGACY"),
                    new CpMessage("v2", "ACTIVE"),
            });

            //check default
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, message, version, "Qa", "v2");
            assertEquals(message, resp);

            //prepare candidate
            Map<String, Object> queueB = new HashMap<>();
            queueB.put("name", "Qb");

            Map<String, Object> bindingAB = new HashMap<>();
            bindingAB.put("source", "Ea");
            bindingAB.put("destination", "Qb");
            bindingAB.put("routing_key", "routeB");

            queues = new HashMap[1];
            queues[0] = queueB;

            bindings = new HashMap[1];
            bindings[0] = bindingAB;

            msConfigs = new MsConfig[]{
                    new MsConfig("ms3", null, queues, bindings),
            };

            //install candidate
            version = "v3";
            reply = applyMsConfigs(200, msConfigs, version);
            assertEquals("ok", reply.getStatus());
            assertEquals("ms3", reply.getMsResponses().get(0).getRequest().getServiceName());

            //check route to ms2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "routeA", message, version, "Qa", "v1");
            assertEquals(message, resp);

            //check route to ms2 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "routeA", message, version, "Qa", "v2");
            assertEquals(message, resp);

            //check route to ms2 v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "routeA", message, version, "Qa", "v2");
            assertEquals(message, resp);

            //check default to ms2 (v2)
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "routeA", message, version, "Qa", "v2");
            assertEquals(message, resp);

            //check route to ms3 v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "routeB", message, version, "Qb", "v3");
            assertEquals(message, resp);

            //check default to ms3 (none)
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "routeB", message, version, "Qb", "v3");
            assertNull(resp);

            //prepare candidate
            queueB = new HashMap<>();
            queueB.put("name", "Qb");

            bindingAB = new HashMap<>();
            bindingAB.put("source", "Ea");
            bindingAB.put("destination", "Qb");
            bindingAB.put("routing_key", "routeA");

            queues = new HashMap[1];
            queues[0] = queueB;

            bindings = new HashMap[1];
            bindings[0] = bindingAB;

            msConfigs = new MsConfig[]{
                    new MsConfig("ms4", null, queues, bindings),
            };

            //install candidate
            version = "v4";
            reply = applyMsConfigs(200, msConfigs, version);
            assertEquals("ok", reply.getStatus());
            assertEquals("ms4", reply.getMsResponses().get(0).getRequest().getServiceName());


            GetResponse response;
            String stringResponse;

            //check route to ms2 v2
            version = "v2";
            message = "some message to " + version;
            channel.basicPublish("Ea", "routeA",
                    new AMQP.BasicProperties.Builder()
                            .headers(Collections.singletonMap("version", version))
                            .build(),
                    message.getBytes());

            response = channel.basicGet("Qa-v2", false);
            stringResponse = new String(response.getBody(), StandardCharsets.UTF_8);
            assertEquals(message, stringResponse);

            response = channel.basicGet("Qb-v4", false);
            assertNull(response);

            //check default to ms2 (v2)
            version = "-";
            message = "some message to " + version;
            channel.basicPublish("Ea", "routeA",
                    new AMQP.BasicProperties.Builder()
                            .headers(Collections.singletonMap("version", version))
                            .build(),
                    message.getBytes());

            response = channel.basicGet("Qa-v2", false);
            stringResponse = new String(response.getBody(), StandardCharsets.UTF_8);
            assertEquals(message, stringResponse);

            response = channel.basicGet("Qb-v4", false);
            assertNull(response);


            //check route to ms2 and ms4 v4
            version = "v4";
            message = "some message to " + version;
            channel.basicPublish("Ea", "routeA",
                    new AMQP.BasicProperties.Builder()
                            .headers(Collections.singletonMap("version", version))
                            .build(),
                    message.getBytes());


            message = "some message to " + version;

            response = channel.basicGet("Qa-v2", false);
            stringResponse = new String(response.getBody(), StandardCharsets.UTF_8);
            assertEquals(message, stringResponse);

            response = channel.basicGet("Qb-v4", false);
            stringResponse = new String(response.getBody(), StandardCharsets.UTF_8);
            assertEquals(message, stringResponse);

            channel.close();
        }
    }

    @Test
    public void rabbitBg2Workflow() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: bill-processor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        kind: vhost\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            entities:\n" +
                        "                exchanges:\n" +
                        "                - name: bill-exchange\n" +
                        "                  type: fanout\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            entities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);


        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;

        BGState bgNamespacesPair = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "idle", null),
                new Date()
        );

        Request request = helper.createJsonRequest(BG2_INIT_DOMAIN, getMaasBasicAuth(), bgNamespacesPair, "POST");
        helper.doRequest(request, BgResponse.class);


        BGState bgState = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "candidate", "v2"),
                new Date());
        request = helper.createJsonRequest(BG2_WARMUP, getMaasBasicAuth(), bgState, "POST");
        helper.doRequest(request, BgResponse.class);
    }


    @SneakyThrows
    @Test
    public void versionRouterConflict() throws Exception {
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
                        "              name: ApplyRabbitConfigIT\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "              tenantId: it-test\n" +
                        "            entities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;
        ConfigV2Resp replyV2 = applyConfigV2(200, cfg);


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
                        "              name: ApplyRabbitConfigIT\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "              tenantId: it-test\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        replyV2 = applyConfigV2(200, cfg);

        String version;
        try (Connection connection = createRabbitConnect(replyV2.getMsResponses().get(0).getResult().getData().getVhost())) {
            Channel channel = connection.createChannel();
            String message;

            //candidate route
            version = "v1";
            message = "some message to " + version;

            String resp = "";
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_separated_6() throws Exception {
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
                        "                - name: e1"
        ;

        ConfigV2Resp reply;
        reply = applyConfigV2(200, cfg);

        //we do not promote v1, it is promoted by default

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
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

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

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
            String resp = "";
            try {
                channel.basicGet("q1-v1", false);
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            //checking that queue exists but empty
            channel = connection.createChannel();
            GetResponse getResponse = channel.basicGet("q1-v2", false);
            assertNull(getResponse);

            //candidate route v2
            channel = connection.createChannel();
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_separated_10() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-processor\n" +
                        "      config: \n" +
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

        //since lazy binding feature it is possible to have binding without exchange
        reply = applyConfigV2(200, cfg);
    }

    @Test
    public void TestConfigV2RabbitBg_separated_11() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key2"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        //ROLLBACK TEST

        //promote
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);
        }

        //rollback
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "ACTIVE"),
                new CpMessage("v2", "CANDIDATE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

    }

    @Test
    public void TestConfigV2RabbitBg_separated_12() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
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
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_separated_13() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-processor\n" +
                        "      config: \n"
        ;

        //since lazy binding feature it is possible to have binding without exchange
        reply = applyConfigV2(200, cfg);
    }

    @Test
    public void TestConfigV2RabbitBg_separated_15() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        //it is get or create operation in MaaS - we're getting vhost which was created in v1
        VirtualHostResponse virtualHost = createVirtualHost(200, createSimpleClassifier("vers-test", null));

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        //ROLLBACK TEST

        //promote
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        //rollback
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "ACTIVE"),
                new CpMessage("v2", "CANDIDATE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_separated_16() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
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
                        "    - serviceName: order-executor\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3 - should be null, no such queue
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertNull(resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_separated_17() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
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
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3 - should be ok
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v3");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_separated_18() throws Exception {
        apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
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
                        "                - name: e1"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        //ROLLBACK TEST

        //promote
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "ARCHIVE"),
                new CpMessage("v2", "LEGACY"),
                new CpMessage("v3", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 - goes to default now
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }


        //rollback
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v2", "ACTIVE"),
                new CpMessage("v3", "CANDIDATE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 - goes to default after delete_candidate implementation
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_single_21() throws Exception {
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_single_24() throws Exception {
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v3");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_single_25() throws Exception {
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_two_bindings_27() throws Exception {
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
                        "                  routing_key: key1\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key2"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 key1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v1 key2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route key1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route key2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 key1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v1 key2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2 key1
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2 key2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route key1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route key2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 key1
            version = "v1";
            message = "some message key1 to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v1 key2
            version = "v1";
            message = "some message key2 to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2 key1
            version = "v2";
            message = "some message key1 to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2 key2
            version = "v2";
            message = "some message key2 to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3 key1
            version = "v3";
            message = "some message key1 to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v3 key2
            version = "v3";
            message = "some message key2 to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route key1
            version = "-";
            message = "some message key1 to " + version;
            resp = sendAndGetMsg(channel, "key1", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route key2
            version = "-";
            message = "some message key2 to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);


        }
    }

    @Test
    public void TestConfigV2RabbitBg_Q_to_new_E_28() throws Exception {
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "                - source: e2\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "    - serviceName: order-executor\n" +
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
                        "                - name: e2"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2 e1
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route v2 e2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v2");
            assertEquals(message, resp);
        }

    }

    @Test
    public void TestConfigV2RabbitBg_Q_to_new_E_29() throws Exception {
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e2\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "    - serviceName: order-executor\n" +
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
                        "                - name: e2"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2 e1
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertNull(resp);

            //candidate route v2 e2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v2");
            assertEquals(message, resp);
        }

    }

    @Test
    public void TestConfigV2RabbitBg_Q_to_new_E_30() throws Exception {
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
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "                - source: e2\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "    - serviceName: order-smth\n" +
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
                        "                - name: e2"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2 e1
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route v2 e2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v2");
            assertEquals(message, resp);
        }

        //ROLLBACK TEST

        //promote
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v2");
            assertEquals(message, resp);

            //candidate route v2 e1
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route v2 e2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v2");
            assertEquals(message, resp);
        }

        //rollback
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "ACTIVE"),
                new CpMessage("v2", "CANDIDATE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v1 e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v1");
            assertNull(resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v1");
            assertNull(resp);

            //candidate route v2 e1
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route v2 e2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e2", "q1", "v2");
            assertEquals(message, resp);
        }

    }

    @Test
    public void TestConfigV2RabbitBg_Q_or_E_moved_31() throws Exception {
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
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-processor\n" +
                        "      config: \n" +
                        "    - serviceName: order-executor\n" +
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

        }

    }

    @Test
    public void TestConfigV2RabbitBg_Q_or_E_moved_32() throws Exception {
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
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-executor\n" +
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
                        "                  routing_key: key"
        ;

        applyConfigV2(SC_BAD_REQUEST, cfg);
    }

    @Test
    public void TestConfigV2RabbitBg_Q_or_E_moved_33() throws Exception {
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
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: \n" +
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

        }

    }

    @Test
    public void TestConfigV2RabbitBg_Q_or_E_moved_34() throws Exception {
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
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  routing_key: key"
        ;

        applyConfigV2(SC_BAD_REQUEST, cfg);
    }

    @Test
    public void TestConfigV2RabbitBg_update_35() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-smth\n" +
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
                        "                - name: e2\n" +
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q2\n" +
                        "                bindings:\n" +
                        "                - source: e2\n" +
                        "                  destination: q2\n" +
                        "                  routing_key: key2"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

        }
    }

    @Test
    public void TestConfigV2RabbitBg_update_36() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2();

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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

        }
    }

    @Test
    public void TestConfigV2RabbitBg_update_36_1() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2();

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
                        "                  type: direct\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: new_key\n"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertNull(resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertNull(resp);

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "new_key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "new_key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);


            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_update_37() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-processor\n" +
                        "      config:\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: "
        ;

        reply = applyConfigV2(200, cfg);

        //it is get or create operation in MaaS - we're getting vhost which was created in v1
        VirtualHostResponse virtualHost = createVirtualHost(200, createSimpleClassifier("vers-test", null));

        String version, message, resp = null;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

        }
    }

    @Test
    public void TestConfigV2RabbitBg_update_38() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2();

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
                        "                  type: direct\n" +
                        "                - name: e3\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                - name: q3\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "                - source: e3\n" +
                        "                  destination: q3\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertNull(resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertNull(resp);

            //candidate route e3
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e3", "q3", "v1");
            assertEquals(message, resp);

            //default route e3
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e3", "q3", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

        }
    }

    @Test
    public void TestConfigV2RabbitBg_update_39_1() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2();

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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: direct\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //candidate route e1 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route e2 v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //candidate route e1 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route e2 v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }

    }

    @Test
    public void TestConfigV2RabbitBg_update_39_2() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2();

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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-changed-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //candidate route e1 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //candidate route e1 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

            //candidate route e2 v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }

    }

    @Test
    public void TestConfigV2RabbitBg_update_39_3() throws Exception {

        apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2_diffVhosts();

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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = createVirtualHost(200, createSimpleClassifier("vers-test", null));
        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

        }

        VirtualHostResponse vh2 = createVirtualHost(200, createSimpleClassifier("vh2", null));
        try (Connection connection = createRabbitConnect(vh2)) {
            Channel channel = connection.createChannel();

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //candidate route e2
            version = "v1";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: direct\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();

            //candidate route e1 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

        }

        try (Connection connection = createRabbitConnect(vh2)) {
            Channel channel = connection.createChannel();

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);


            //candidate route e2 v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: fanout\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: true\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key3"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route e1 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route e1
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = "";
            try {
                resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();

            //candidate route e1 v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "e1", "q1", "v2");
            assertEquals(message, resp);

        }

        try (Connection connection = createRabbitConnect(vh2)) {
            Channel channel = connection.createChannel();

            //candidate route e2 v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //candidate route e2 v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);

            //default route e2
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key2", message, version, "e2", "q2", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestBillPaymentVisaTax_40() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: bill-processor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        kind: vhost\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: bill-exchange\n" +
                        "                  type: fanout\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);
        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: bill-processor\n" +
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
                        "                - name: bill-exchange\n" +
                        "                  type: topic\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue\n" +
                        "                  routing_key: non-visa"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);
        }

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);
        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: visa-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: visa-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: visa-queue\n" +
                        "                  routing_key: visa"
        ;

        reply = applyConfigV2(200, cfg);

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v3 - non-visa case
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v3 - visa case
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "visa", message, version, "bill-exchange", "visa-queue", "v3");
            assertEquals(message, resp);


            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v4\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: bill-processor\n" +
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
                        "                - name: bill-exchange\n" +
                        "                  type: fanout\n" +
                        "    - serviceName: tax-tracker\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "              queues:\n" +
                        "              - name: tax-queue\n" +
                        "              bindings:\n" +
                        "              - source: bill-exchange\n" +
                        "                destination: tax-queue"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
                new CpMessage("v3", "CANDIDATE"),
                new CpMessage("v4", "CANDIDATE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v3 - non-visa case
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v3 - visa case
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "visa", message, version, "bill-exchange", "visa-queue", "v3");
            assertEquals(message, resp);

            //candidate route v4
            version = "v4";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4 second fanout message
            resp = getMsg(channel, "payment-queue", "v2");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);
        }

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "ARCHIVE"),
                new CpMessage("v2", "LEGACY"),
                new CpMessage("v4", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 - E-v1 and Q-v1 were deleted, now default is active v4, its queue is v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);


            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v3 visa case - E-v3 and visa-queue-v3 were deleted, now default is active v4, exchange is fanout type, so it will send message to payment-queue-v2 even with such header
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);

            //check that there are no payment-queue-v1 and visa-queue-v3
            resp = "";
            try {
                getMsg(channel, "payment-queue", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();
            resp = "";
            try {
                getMsg(channel, "visa-queue", "v3");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();

            //candidate route v4
            version = "v4";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4 second fanout message
            resp = getMsg(channel, "payment-queue", "v2");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v5\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "              queues:\n" +
                        "              - name: payment-queue\n" +
                        "              bindings:\n" +
                        "              - source: bill-exchange\n" +
                        "                destination: payment-queue\n" +
                        "    - serviceName: tax-tracker\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "              queues:\n" +
                        "              - name: tax-queue\n" +
                        "              bindings:\n" +
                        "              - source: bill-exchange\n" +
                        "                destination: tax-queue"
        ;

        reply = applyConfigV2(200, cfg);


        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1 - E-v1 and Q-v1 were deleted, now default is active v4, its queue is v2
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v3 visa case - E-v3 and visa-queue-v3 were deleted, now default is active v4, exchange is fanout type, so it will send message to payment-queue-v2 even with such header
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4
            version = "v4";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4 second fanout message
            resp = getMsg(channel, "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v4
            version = "v5";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v5");
            assertEquals(message, resp);

            //candidate route v4 second fanout message
            resp = getMsg(channel, "payment-queue", "v5");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);
        }

        //promote
        changeActiveVersion(new CpMessage[]{
                new CpMessage("v2", "ARCHIVE"),
                new CpMessage("v4", "LEGACY"),
                new CpMessage("v5", "ACTIVE"),
        });


        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v2 - E v2 should be deleted, but queue and ms should stay. message goes to default v5
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v5");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v5");
            assertEquals(message, resp);

            //candidate route v4
            version = "v4";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4 second fanout message
            resp = getMsg(channel, "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v5
            version = "v5";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v5");
            assertEquals(message, resp);

            //candidate route v5 second fanout message
            resp = getMsg(channel, "payment-queue", "v5");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v5");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v5");
            assertEquals(message, resp);

            //checking empty old queues are null
            resp = getMsg(channel, "payment-queue", "v2");
            assertNull(resp);

            resp = getMsg(channel, "tax-queue", "v4");
            assertNull(resp);
        }

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v4", "ACTIVE"),
                new CpMessage("v5", "CANDIDATE"),
        });


        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();


            //candidate route v2 - E v2 should be deleted, but queue and ms should stay. message goes to default v4, its queue is v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4
            version = "v4";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v4");
            assertEquals(message, resp);

            //candidate route v4 second fanout message
            resp = getMsg(channel, "payment-queue", "v2");
            assertEquals(message, resp);

            //candidate route v5
            version = "v5";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "tax-queue", "v5");
            assertEquals(message, resp);

            //candidate route v5 second fanout message
            resp = getMsg(channel, "payment-queue", "v5");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v2");
            assertEquals(message, resp);

            //default route second fanout message
            resp = getMsg(channel, "tax-queue", "v4");
            assertEquals(message, resp);

            //checking empty old queues are null
            resp = getMsg(channel, "payment-queue", "v5");
            assertNull(resp);

            resp = getMsg(channel, "tax-queue", "v5");
            assertNull(resp);

            //check that there are no payment-queue-v1 and visa-queue-v3
            resp = "";
            try {
                resp = getMsg(channel, "payment-queue", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);

            channel = connection.createChannel();
            resp = "";
            try {
                resp = getMsg(channel, "visa-queue", "v3");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
        }

    }

    @Test
    public void TestDeleteCandidate_41() throws Exception {
        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: bill-processor\n" +
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
                        "                - name: bill-exchange\n" +
                        "                  type: fanout\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        String version, message, resp;

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: at-least-one-service\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v3\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v4\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "ACTIVE"),
                new CpMessage("v2", "CANDIDATE"),
                new CpMessage("v3", "CANDIDATE"),
                new CpMessage("v4", "CANDIDATE"),
        });


        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v3");
            assertEquals(message, resp);

            //candidate route v4
            version = "v4";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v4");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);
        }

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //check that there are no payment-queue-v3 and payment-queue-v4
            resp = "";
            try {
                getMsg(channel, "payment-queue", "v3");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
            channel = connection.createChannel();

            resp = "";
            try {
                getMsg(channel, "payment-queue", "v4");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
        }


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v5\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);


        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v6\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
                new CpMessage("v5", "CANDIDATE"),
                new CpMessage("v6", "CANDIDATE"),
        });


        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v3
            version = "v3";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);

            //candidate route v5
            version = "v5";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v5");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v1");
            assertEquals(message, resp);
        }

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v2", "LEGACY"),
                new CpMessage("v5", "ACTIVE"),
        });

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v7\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: payment-gateway\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: payment-queue\n" +
                        "                bindings:\n" +
                        "                - source: bill-exchange\n" +
                        "                  destination: payment-queue"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v5", "LEGACY"),
                new CpMessage("v7", "ACTIVE"),
        });


        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v5
            version = "v5";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v5");
            assertEquals(message, resp);

            //candidate route v7
            version = "v7";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "", message, version, "bill-exchange", "payment-queue", "v7");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "non-visa", message, version, "bill-exchange", "payment-queue", "v7");
            assertEquals(message, resp);

            //check that there are no payment-queue-v3 and payment-queue-v4
            resp = "";
            try {
                resp = getMsg(channel, "payment-queue", "v1");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
            channel = connection.createChannel();

            resp = "";
            try {
                getMsg(channel, "payment-queue", "v6");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
            channel = connection.createChannel();

            resp = "";
            try {
                resp = getMsg(channel, "payment-queue", "v7");
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals(resp, "no exception");

        }
    }


    /*
    modelling situation when entity was added in db, yet not commited, then created in Rabbit, and then Rabbit crashed and now NO db record about this entity, but entity is in Rabbit:
    1. create entity manually in Rabbit
    2. create it via config (repeat same)
    3. check that it is okay (it would be 200, not 201)
    */
    @Test
    public void TestIdempotencyForRabbitEntityCreation() throws Exception {

        //applying config to create vhost
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
                        "              namespace: " + TEST_NAMESPACE + "\n"
        ;

        reply = applyConfigV2(200, cfg);

        String version, message, resp;

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //AE, VR, versioned exchange
            channel.exchangeDeclare("e1-ae", BuiltinExchangeType.FANOUT);
            channel.exchangeDeclare("e1", BuiltinExchangeType.HEADERS, false, false, Collections.singletonMap("alternate-exchange", "e1-ae"));
            channel.exchangeDeclare("e1-v1", BuiltinExchangeType.DIRECT);

            channel.exchangeBind("e1-v1", "e1", "*", Collections.singletonMap("version", "v1"));
            channel.exchangeBind("e1-v1", "e1-ae", "");

            channel.queueDeclare("q1-v1", false, false, false, null);
            channel.queueBind("q1-v1", "e1-v1", "key");
        }

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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }


    /*
    modelling situation when entity was deleted in db, yet not commited, then deleted in Rabbit, and then Rabbit crashed and now there is STILL db record about this entity, but NO entity in Rabbit:
    1. create entity via config
    2. delete it manually
    3. delete it via config change
    4. check that it is okay (it would be 404, not 202)
    */
    @Test
    public void TestIdempotencyForRabbitEntityDeletion() throws Exception {
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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);

        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //AE, VR, versioned exchange
            channel.exchangeDelete("e1-ae");
            channel.exchangeDelete("e1");
            channel.exchangeDelete("e1-v1");

            channel.queueDelete("q1-v1");
        }

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
                        "    - serviceName: order-processor\n" +
                        "      config:"
        ;

        reply = applyConfigV2(200, cfg);
    }


    //test for distributed transaction
    @Test
    public void TestCorrectWorkAfterValidationFail() throws Exception {
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
                        "                - name: q2\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(SC_BAD_REQUEST, cfg);

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
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(SC_OK, cfg);
    }


    @Test
    public void TestConfigV2RabbitBg_LazyBinding_UpdateCandidate() throws Exception {
        RabbitConfigValidation validation = validateRabbitConfigs();
        assertEquals(0, validation.getBindings().length);

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
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
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        validation = validateRabbitConfigs();
        assertEquals(1, validation.getBindings().length);
        assertEquals("v1", validation.getBindings()[0].getExchangeVersion());
        assertEquals("v1", validation.getBindings()[0].getQueueVersion());
        assertEquals(helper.getVhostFromCnn(virtualHost.getCnn()), validation.getBindings()[0].getVhost());

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
                        "                - name: e1\n"
        ;

        reply = applyConfigV2(200, cfg);

        validation = validateRabbitConfigs();
        assertEquals(0, validation.getBindings().length);

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }
    }

    @Test
    public void TestConfigV2RabbitBg_LazyBinding_NewCandidate() throws Exception {
        RabbitConfigValidation validation = validateRabbitConfigs();
        assertEquals(0, validation.getBindings().length);

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  services:\n" +
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
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        validation = validateRabbitConfigs();
        log.info("Lazy binding: {}", validation.getBindings());
        assertEquals(1, validation.getBindings().length);
        assertEquals("v1", validation.getBindings()[0].getExchangeVersion());
        assertEquals("v1", validation.getBindings()[0].getQueueVersion());
        assertEquals(helper.getVhostFromCnn(virtualHost.getCnn()), validation.getBindings()[0].getVhost());

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                - name: e1\n"
        ;

        reply = applyConfigV2(200, cfg);

        validation = validateRabbitConfigs();
        log.info("Lazy binding: {}", validation.getBindings());
        assertEquals(1, validation.getBindings().length);
        assertEquals("v1", validation.getBindings()[0].getExchangeVersion());
        assertEquals("v1", validation.getBindings()[0].getQueueVersion());
        assertEquals(helper.getVhostFromCnn(virtualHost.getCnn()), validation.getBindings()[0].getVhost());

        String version, message, resp;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", version);
            assertNull(resp);

            //candidate route v2
            version = "v2";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsg(channel, "key", message, version, "e1", "q1", "v1");
            assertNull(resp);
        }
    }


    private void apply_Ms1WithE1_Ms2WithQ1AndBinding() throws IOException {
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
                        "                  type: direct\n" +
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

        reply = applyConfigV2(200, cfg);
    }

    private void apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion() throws IOException {
        apply_Ms1WithE1_Ms2WithQ1AndBinding();

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec:\n" +
                        "  version: v2\n" +
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
                        "                  type: direct"
        ;

        reply = applyConfigV2(200, cfg);

        changeActiveVersion(new CpMessage[]{
                new CpMessage("v1", "LEGACY"),
                new CpMessage("v2", "ACTIVE"),
        });
    }

    private void apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2() throws IOException {
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
                        "                  type: direct\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "    - serviceName: order-smth\n" +
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
                        "                - name: e2\n" +
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q2\n" +
                        "                bindings:\n" +
                        "                - source: e2\n" +
                        "                  destination: q2\n" +
                        "                  routing_key: key2"
        ;

        reply = applyConfigV2(200, cfg);
    }

    private void apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE2Q2B2_diffVhosts() throws IOException {
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
                        "                  type: direct\n" +
                        "    - serviceName: order-executor\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vers-test\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n" +
                        "    - serviceName: order-smth\n" +
                        "      config: |+\n" +
                        "        ---\n" +
                        "        apiVersion: nc.maas.rabbit/v2\n" +
                        "        kind: vhost\n" +
                        "        spec:\n" +
                        "            classifier:\n" +
                        "              name: vh2\n" +
                        "              namespace: " + TEST_NAMESPACE + "\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e2\n" +
                        "                  type: direct\n" +
                        "                queues:\n" +
                        "                - name: q2\n" +
                        "                bindings:\n" +
                        "                - source: e2\n" +
                        "                  destination: q2\n" +
                        "                  routing_key: key2"
        ;

        reply = applyConfigV2(200, cfg);
    }

    private String sendAndGetMsg(Channel channel, String message, String version, String queueVersion) throws IOException, TimeoutException {
        return sendAndGetMsg(channel, "routeA", message, version, "Qa", queueVersion);
    }

    private String sendAndGetMsg(Channel channel, String message, String version, String queueName, String queueVersion) throws IOException, TimeoutException {
        return sendAndGetMsg(channel, "routeA", message, version, queueName, queueVersion);
    }

    private String sendAndGetMsg(Channel channel, String route, String message, String version, String queueName, String queueVersion) throws IOException, TimeoutException {
        channel.basicPublish("Ea", route,
                new AMQP.BasicProperties.Builder()
                        .headers(Collections.singletonMap("version", version))
                        .build(),
                message.getBytes());

        GetResponse resp = channel.basicGet(queueName + "-" + queueVersion, false);
        return (resp != null ? new String(resp.getBody(), StandardCharsets.UTF_8) : null);
    }

    private String sendAndGetMsg(Channel channel, String route, String message, String headerVersion, String exchangeName, String queueName, String queueVersion) throws IOException {
        val future = new CompletableFuture<String>();
        DeliverCallback callback = (tag, msg) -> {
            future.complete(new String(msg.getBody(), StandardCharsets.UTF_8));
        };

        CancelCallback cancelCallback = (x) -> {
        };
        String tag = channel.basicConsume(queueName + "-" + queueVersion, true, callback, cancelCallback);

        channel.basicPublish(exchangeName, route,
                new AMQP.BasicProperties.Builder()
                        .headers(Collections.singletonMap("version", headerVersion))
                        .build(),
                message.getBytes());

        String result = null;
        try {
            result = future.get(3, TimeUnit.SECONDS);
        } catch (Exception e) {
            log.error("Timeout during processing message: {}", message);
        }

        channel.basicCancel(tag);

        return result;
    }

    private String getMsg(Channel channel, String queueName, String queueVersion) throws IOException {
        GetResponse resp = channel.basicGet(queueName + "-" + queueVersion, true);
        return (resp != null ? new String(resp.getBody(), StandardCharsets.UTF_8) : null);
    }

    public Map<String, Object> getExchange() {
        Map<String, Object> exchange = new HashMap<>();
        Map<String, Object> argumentsExchange = new HashMap<>();
        argumentsExchange.put("some-argument", "smth");
        exchange.put("name", "Ea");
        exchange.put("type", "direct");
        exchange.put("durable", "false");
        exchange.put("arguments", argumentsExchange);
        return exchange;
    }

    public Map<String, Object> getExchangeForUpdate() {
        Map<String, Object> exchange = new HashMap<>();
        exchange.put("name", "E1");
        exchange.put("type", "fanout");
        return exchange;
    }

    public Map<String, Object> getQueue() {
        Map<String, Object> queue = new HashMap<>();
        Map<String, Object> argumentsQueue = new HashMap<>();
        argumentsQueue.put("x-dead-letter-exchange", "cpq-quote-modify-dlx");
        queue.put("name", "Qa");
        queue.put("durable", "false");
        queue.put("arguments", argumentsQueue);
        return queue;
    }

    public Map<String, Object> getQueueForUpdate() {
        Map<String, Object> queue = new HashMap<>();
        Map<String, Object> argumentsQueue = new HashMap<>();
        argumentsQueue.put("x-dead-letter-exchange", "cpq-quote-modify-dlx");
        queue.put("name", "ApplyRabbitConfigIT-queue2");
        queue.put("durable", "true");
        queue.put("arguments", argumentsQueue);
        return queue;
    }

    public Map<String, Object> getBinding() {
        Map<String, Object> binding = new HashMap<>();
        binding.put("source", "Ea");
        binding.put("destination", "Qa");
        binding.put("routing_key", "routeA");
        return binding;
    }

    public Map<String, Object> getBindingForUpdate() {
        Map<String, Object> binding = new HashMap<>();
        binding.put("destination", "ApplyRabbitConfigIT-queue2");
        binding.put("source", "ApplyRabbitConfigIT-exchange");
        binding.put("routing_key", "route");
        return binding;
    }
}
