package com.netcracker.it.maas.rabbit;

import com.netcracker.it.maas.AbstractMaasWithInitsIT;
import com.netcracker.it.maas.entity.ConfigV2Resp;
import com.netcracker.it.maas.entity.rabbit.RabbitConfigResp;
import com.netcracker.it.maas.entity.rabbit.RabbitInstance;
import com.netcracker.it.maas.entity.rabbit.VhostConfigResponse;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import com.rabbitmq.client.*;
import lombok.extern.slf4j.Slf4j;
import lombok.val;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

import static com.netcracker.it.maas.MaasITHelper.RABBIT_RECOVERY_PATH;
import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.apache.http.HttpStatus.SC_OK;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.*;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

@Slf4j
class ApplyRabbitConfigIT extends RabbitTest {

    @Test
    public void testRabbitRecovery() throws Exception {
        String cfg;
        ConfigV2Resp reply;

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
                        "            entities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1-non-v\n" +
                        "                queues:\n" +
                        "                - name: q1-non-v\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1-non-v\n" +
                        "                  destination: q1-non-v\n" +
                        "                  routing_key: key\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        //check success creation
        String version;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message, resp;

            //non-v
            message = "some message";
            resp = sendAndGetMsg(channel, "key", message, "e1-non-v", "q1-non-v");
            assertEquals(message, resp);

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            channel.exchangeDelete("e1");
            channel.exchangeDelete("e1-non-v");
            channel.queueDelete("q1-v1");
            channel.queueDelete("q1-non-v");
            channel.close();
        }

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            //checking that no such exchange
            String resp = "";
            String message = "abc";
            //checking that no such queue
            resp = "";
            try {
                channel.basicGet("q1-v1", false);
                resp = "no exception";
            } catch (IOException e) {
                assertNotNull(e);
            }
            assertEquals("", resp);
        }

        String resp = "";
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            //checking that no such exchange

            String message = "abc";
            try {
                channel.basicPublish("e1", "", null, message.getBytes("UTF-8"));
                resp = "no exception";
            } catch (Exception e) {
                assertNotNull(e);
            }
        } catch (Exception e) {
            assertEquals("", resp);
        }

        // recover

        Request request = helper.createJsonRequest(RABBIT_RECOVERY_PATH, getMaasBasicAuth(), "", "POST");

        helper.doRequest(request);

        // check working

        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message;

            //non-v
            message = "some message";
            resp = sendAndGetMsg(channel, "key", message, "e1-non-v", "q1-non-v");
            assertEquals(message, resp);

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);
        }

    }

    @Test
    public void testRabbitRecoveryChangeInstance() throws Exception {
        String cfg;
        ConfigV2Resp reply;

        //create entities in first default instance

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
                        "            entities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1-non-v\n" +
                        "                queues:\n" +
                        "                - name: q1-non-v\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1-non-v\n" +
                        "                  destination: q1-non-v\n" +
                        "                  routing_key: key\n" +
                        "            versionedEntities:\n" +
                        "                exchanges:\n" +
                        "                - name: e1\n" +
                        "                queues:\n" +
                        "                - name: q1\n" +
                        "                  durable: false\n" +
                        "                bindings:\n" +
                        "                - source: e1\n" +
                        "                  destination: q1\n" +
                        "                  routing_key: key\n"
        ;

        reply = applyConfigV2(200, cfg);
        VirtualHostResponse virtualHost = reply.getMsResponses().get(0).getResult().getData().getVhost();

        //check success creation
        String version;
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            String message, resp;

            //non-v
            message = "some message";
            resp = sendAndGetMsg(channel, "key", message, "e1-non-v", "q1-non-v");
            assertEquals(message, resp);

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            //clear for not storing garbage for next test
            channel.exchangeDelete("e1");
            channel.exchangeDelete("e1-v1");
            channel.exchangeDelete("e1-non-v");
            channel.queueDelete("q1-v1");
            channel.queueDelete("q1-non-v");
            channel.close();
        }

        //delete second instance and rename default instance to it. save initial default to recover in the end of test
        changeDefaultInstancePropsToSecondInstance();

        Request request = helper.createJsonRequest(RABBIT_RECOVERY_PATH, getMaasBasicAuth(), "", "POST");
        helper.doRequest(request);

        //get updated vhost connection
        VhostConfigResponse vhostConfig = getRabbitVhostByClassifier(SC_OK, createSimpleClassifier("vers-test"));

        //check success for new instance
        try (Connection connection = createRabbitConnect(vhostConfig.getVhost())) {
            Channel channel = connection.createChannel();
            String message, resp;

            //non-v
            message = "some message";
            resp = sendAndGetMsg(channel, "key", message, "e1-non-v", "q1-non-v");
            assertEquals(message, resp);

            //candidate route v1
            version = "v1";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", version);
            assertEquals(message, resp);

            //default route
            version = "-";
            message = "some message to " + version;
            resp = sendAndGetMsgVersioned(channel, "key", message, version, "e1", "q1", "v1");
            assertEquals(message, resp);

            channel.close();
        }
    }

    @Test
    public void changeDefaultInstancePropsToSecondInstance() throws Exception {
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assumeTrue(rabbitInstances.length >= 2, "deleteDefaultInstanceRabbit test requires at least two instances. Skip test");

        RabbitInstance defaultInstance, secondInstance;
        if (rabbitInstances[0].getIsDefault()) {
            defaultInstance = rabbitInstances[0];
            secondInstance = rabbitInstances[1];
        } else {
            defaultInstance = rabbitInstances[1];
            secondInstance = rabbitInstances[0];
        }

        deleteRabbitInstance(secondInstance, SC_OK);

        defaultInstance.setAmqpUrl(secondInstance.getAmqpUrl());
        defaultInstance.setApiUrl(secondInstance.getApiUrl());
        defaultInstance.setUser(secondInstance.getUser());
        defaultInstance.setPassword(secondInstance.getPassword());
        RabbitInstance updatedDefaultInstance = updateRabbitInstance(defaultInstance);
        assertTrue(updatedDefaultInstance.getIsDefault());
        assertEquals(defaultInstance.getId(), updatedDefaultInstance.getId());
        assertEquals(secondInstance.getAmqpUrl(), updatedDefaultInstance.getAmqpUrl());
        assertEquals(secondInstance.getApiUrl(), updatedDefaultInstance.getApiUrl());
        assertEquals(secondInstance.getUser(), updatedDefaultInstance.getUser());
        assertEquals(secondInstance.getPassword(), updatedDefaultInstance.getPassword());
    }

    @Test
    public void createSingleExchangeWithoutExistingVhost() throws IOException {
        Map<String, Object> exchange = new HashMap<>();
        exchange.put("name", "ApplyRabbitConfigIT-exchange");
        log.info("Applying config with exchange {}", convertMapToStream(exchange));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(exchange, null, null);

        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData().getEntities().getExchanges());
        assertNull(reply.getResult().getData().getEntities().getQueues());
        assertNull(reply.getResult().getData().getEntities().getBindings());
        assertNull(reply.getResult().getData().getPolicies());
        assertEquals(1, reply.getResult().getData().getEntities().getExchanges().length);
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getEntities().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getResult().getData().getEntities().getExchanges()[0].get("name"));
    }

    @Test
    public void createSingleExchangeWithExistingVhost() throws IOException {
        createVirtualHost(201, createSimpleClassifier("ApplyRabbitConfigIT", "it-test"));

        Map<String, Object> exchange = new HashMap<>();
        exchange.put("name", "ApplyRabbitConfigIT-exchange");
        log.info("Applying config with exchange {}", convertMapToStream(exchange));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(exchange, null, null);

        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData().getEntities().getExchanges());
        assertNull(reply.getResult().getData().getEntities().getQueues());
        assertNull(reply.getResult().getData().getEntities().getBindings());
        assertNull(reply.getResult().getData().getPolicies());
        assertEquals(1, reply.getResult().getData().getEntities().getExchanges().length);
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getEntities().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getResult().getData().getEntities().getExchanges()[0].get("name"));
    }

    @Test
    public void applyRabbitConfigWithoutExistingVhost() throws IOException {
        Map<String, Object> exchange = getExchange();
        Map<String, Object> queue = getQueue();
        Map<String, Object> binding = getBinding();
        Map<String, Object> policy = getPolicy();
        //log.info("Applying config with exchange {}, queue {}, binding {}, policy {}", convertMapToStream(exchange), convertMapToStream(queue), convertMapToStream(binding), convertMapToStream(policy));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(exchange, queue, binding, policy);

        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData().getEntities().getExchanges());
        assertNotNull(reply.getResult().getData().getEntities().getQueues());
        assertNotNull(reply.getResult().getData().getEntities().getBindings());
        assertNotNull(reply.getResult().getData().getPolicies());
        assertEquals(1, reply.getResult().getData().getEntities().getExchanges().length);
        assertEquals(1, reply.getResult().getData().getPolicies().length);
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getEntities().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getResult().getData().getEntities().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getRequest().getSpec().getEntities().getQueues()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getResult().getData().getEntities().getQueues()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getEntities().getBindings()[0].get("source"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getRequest().getSpec().getEntities().getBindings()[0].get("destination"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getResult().getData().getEntities().getBindings()[0].get("source"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getResult().getData().getEntities().getBindings()[0].get("destination"));
        assertEquals("ApplyRabbitConfigIT-policy", reply.getRequest().getSpec().getPolicies()[0].get("name"));
        log.info("{}", reply.getResult().getData().getEntities());
        assertEquals("ApplyRabbitConfigIT-policy", reply.getResult().getData().getPolicies()[0].get("name"));
    }


    @Test
    public void applyConfigWithSingleLazyBinding() throws IOException {
        Map<String, Object> binding = getBinding();

        log.info("Applying config with binding {}", convertMapToStream(binding));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(200, null, null, binding, null, ApplyConfigOperation.CREATE);

        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData());
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getEntities().getBindings()[0].get("source"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getRequest().getSpec().getEntities().getBindings()[0].get("destination"));
    }

    @Test
    public void applyConfigWithLazyBinding() throws Exception {
        Map<String, Object> exchange = getExchange();
        Map<String, Object> queue = getQueue();

        Map<String, Object> binding = new HashMap<>();
        binding.put("destination", "ApplyRabbitConfigIT-queue");
        binding.put("source", "ApplyRabbitConfigIT-exchange");
        binding.put("routing_key", "route");

        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(null, queue, binding, null);
        assertEquals("ok", reply.getResult().getStatus());

        reply = applySingleConfigWithEntities(exchange, null, null, null);
        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData());

        String message, resp;

        try (Connection connection = createRabbitConnect(reply.getResult().getData().getVhost())) {
            Channel channel = connection.createChannel();
            //default route e1
            message = "some message";
            resp = sendAndGetMsg(channel, "route", message, "ApplyRabbitConfigIT-exchange", "ApplyRabbitConfigIT-queue");
            assertEquals(message, resp);


            binding.put("properties_key", "route");
            //remove binding
            reply = applySingleConfigWithEntities(200, null, null, binding, null, ApplyConfigOperation.DELETE);
            assertEquals("ok", reply.getResult().getStatus());
            assertEquals(1, reply.getResult().getData().getDeletions().getBindings().length);

            // check that no message
            message = "some message2";
            try {
                channel.basicPublish("ApplyRabbitConfigIT-exchange", "route",
                        new AMQP.BasicProperties.Builder()
                                .build(),
                        message.getBytes());
                channel.basicGet("ApplyRabbitConfigIT-queue", false);
            } catch (IOException e) {
                assertNotNull(e);
            }
        }
    }

    @Test
    public void applyConfigWithLazyBindingAndExchangeWithBinding() throws Exception {
        Map<String, Object> exchange = getExchange();
        Map<String, Object> queue = getQueue();

        Map<String, Object> binding = new HashMap<>();
        binding.put("destination", "ApplyRabbitConfigIT-queue");
        binding.put("source", "ApplyRabbitConfigIT-exchange");
        binding.put("routing_key", "route");

        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(null, queue, binding, null);
        assertEquals("ok", reply.getResult().getStatus());

        reply = applySingleConfigWithEntities(exchange, null, binding, null);
        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData());

        String message, resp;

        try (Connection connection = createRabbitConnect(reply.getResult().getData().getVhost())) {
            Channel channel = connection.createChannel();
            //default route e1
            message = "some message";
            resp = sendAndGetMsg(channel, "route", message, "ApplyRabbitConfigIT-exchange", "ApplyRabbitConfigIT-queue");
            assertEquals(message, resp);


            binding.put("properties_key", "route");
            //remove binding
            reply = applySingleConfigWithEntities(200, null, null, binding, null, ApplyConfigOperation.DELETE);
            assertEquals("ok", reply.getResult().getStatus());
            assertEquals(1, reply.getResult().getData().getDeletions().getBindings().length);

            // check that no message
            message = "some message2";
            try {
                channel.basicPublish("ApplyRabbitConfigIT-exchange", "route",
                        new AMQP.BasicProperties.Builder()
                                .build(),
                        message.getBytes());
                channel.basicGet("ApplyRabbitConfigIT-queue", false);
            } catch (IOException e) {
                assertNotNull(e);
            }
        }
    }

    private String sendAndGetMsgVersioned(Channel channel, String route, String message, String headerVersion, String exchangeName, String queueName, String queueVersion) throws IOException {
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
            log.error("Timout during processing message: {}", message);
        }

        channel.basicCancel(tag);

        return result;
    }

    private String sendAndGetMsg(Channel channel, String route, String message, String exchangeName, String queueName) throws IOException {
        val future = new CompletableFuture<String>();
        DeliverCallback callback = (tag, msg) -> {
            future.complete(new String(msg.getBody(), StandardCharsets.UTF_8));
        };

        CancelCallback cancelCallback = (x) -> {
        };
        String tag = channel.basicConsume(queueName, true, callback, cancelCallback);

        channel.basicPublish(exchangeName, route,
                new AMQP.BasicProperties.Builder()
                        .build(),
                message.getBytes());


        String result = null;
        try {
            result = future.get(3, TimeUnit.SECONDS);
        } catch (Exception e) {
            log.error("Timout during processing message: {}", message);
        }

        channel.basicCancel(tag);

        return result;
    }


    @Test
    public void updateSingleExchangeWithoutExistingVhost() throws IOException {
        Map<String, Object> exchange = new HashMap<>();
        exchange.put("name", "ApplyRabbitConfigIT-exchange");
        exchange.put("durable", "true");
        log.info("Applying config with exchange {}", convertMapToStream(exchange));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(exchange, null, null);

        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(1, reply.getResult().getData().getEntities().getExchanges().length);

        Map<String, Object> exchangeNew = new HashMap<>();
        exchangeNew.put("name", "ApplyRabbitConfigIT-exchange");
        exchangeNew.put("durable", "false");
        log.info("Applying config with exchange {}", convertMapToStream(exchangeNew));
        reply = applySingleConfigWithEntities(exchangeNew, null, null);


        assertEquals("ok", reply.getResult().getStatus());
        assertNotNull(reply.getResult().getData().getEntities().getExchanges());
        assertEquals(1, reply.getResult().getData().getEntities().getExchanges().length);
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getEntities().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getResult().getData().getEntities().getExchanges()[0].get("name"));
        assertEquals("false", reply.getRequest().getSpec().getEntities().getExchanges()[0].get("durable"));
        assertThat(reply.getResult().getData().getUpdateStatus()[0].get("reason").toString(), CoreMatchers.containsString("inequivalent arg 'durable' for exchange"));
    }


    @Test
    public void deletePreviouslyCreatedExchangeAndQueueAndPolicy() throws IOException {
        Map<String, Object> exchange = getExchange();
        Map<String, Object> queue = getQueue();
        Map<String, Object> policy = getPolicy();
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(exchange, queue, null, policy);

        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(1, reply.getResult().getData().getEntities().getExchanges().length);
        assertEquals(1, reply.getResult().getData().getEntities().getQueues().length);

        reply = applySingleConfigWithEntities(200, exchange, queue, null, policy, ApplyConfigOperation.DELETE);

        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(1, reply.getResult().getData().getDeletions().getExchanges().length);
        assertEquals(1, reply.getResult().getData().getDeletions().getQueues().length);
        assertEquals(1, reply.getResult().getData().getDeletions().getPolicies().length);
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getRequest().getSpec().getDeletions().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-exchange", reply.getResult().getData().getDeletions().getExchanges()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getRequest().getSpec().getDeletions().getQueues()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-queue", reply.getResult().getData().getDeletions().getQueues()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-policy", reply.getRequest().getSpec().getDeletions().getPolicies()[0].get("name"));
        assertEquals("ApplyRabbitConfigIT-policy", reply.getResult().getData().getDeletions().getPolicies()[0].get("name"));
    }


    @Test
    public void createQueueBadRequest() throws IOException {
        Map<String, Object> arguments = new HashMap<>();
        arguments.put("x-queue-type", "quorum");
        arguments.put("x-message-ttl", "1000");

        Map<String, Object> queue = new HashMap<>();
        queue.put("name", "ApplyRabbitConfigIT-queue");
        queue.put("arguments", arguments);
        log.info("Applying config with queue {}", convertMapToStream(queue));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(500, null, queue, null, null, ApplyConfigOperation.CREATE);

        assertEquals("error", reply.getResult().getStatus());
        assertThat(reply.getResult().getError(), CoreMatchers.containsString("bad_request"));
    }

    @Test
    public void createQueueWithBigNumberArgument() throws IOException {
        Map<String, Object> arguments = new HashMap<>();
        arguments.put("x-message-ttl", 100000000);

        Map<String, Object> queue = new HashMap<>();
        queue.put("name", "ApplyRabbitConfigIT-queue");
        queue.put("arguments", arguments);
        log.info("Applying config with queue {}", convertMapToStream(queue));
        RabbitConfigResp.SingleReply reply = applySingleConfigWithEntities(200, null, queue, null, null, ApplyConfigOperation.CREATE);

        assertEquals("ok", reply.getResult().getStatus());
        assertEquals(1, reply.getResult().getData().getEntities().getQueues().length);
        assertEquals("ApplyRabbitConfigIT-queue", reply.getRequest().getSpec().getEntities().getQueues()[0].get("name"));
    }


    public Map<String, Object> getExchange() {
        Map<String, Object> exchange = new HashMap<>();
        Map<String, Object> argumentsExchange = new HashMap<>();
        argumentsExchange.put("some-argument", "smth");
        exchange.put("name", "ApplyRabbitConfigIT-exchange");
        exchange.put("type", "direct");
        exchange.put("durable", "false");
        exchange.put("arguments", argumentsExchange);
        return exchange;
    }

    public Map<String, Object> getQueue() {
        Map<String, Object> queue = new HashMap<>();
        Map<String, Object> argumentsQueue = new HashMap<>();
        argumentsQueue.put("x-dead-letter-exchange", "cpq-quote-modify-dlx");
        queue.put("name", "ApplyRabbitConfigIT-queue");
        queue.put("durable", "false");
        queue.put("arguments", argumentsQueue);
        return queue;
    }

    public Map<String, Object> getBinding() {
        Map<String, Object> binding = new HashMap<>();
        Map<String, Object> argumentsBindings = new HashMap<>();
        argumentsBindings.put("x-some-header", "header");
        binding.put("destination", "ApplyRabbitConfigIT-queue");
        binding.put("source", "ApplyRabbitConfigIT-exchange");
        binding.put("routing_key", "route");
        return binding;
    }

    public Map<String, Object> getPolicy() {
        Map<String, Object> policy = new HashMap<>();
        Map<String, Object> definitionPolicy = new HashMap<>();
        definitionPolicy.put("ha-mode", "exactly");
        definitionPolicy.put("ha-params", 1);
        definitionPolicy.put("ha-sync-mode", "automatic");
        policy.put("name", "ApplyRabbitConfigIT-policy");
        policy.put("pattern", "^mirrored\\.");
        policy.put("definition", definitionPolicy);
        policy.put("apply-to", "exchanges");
        return policy;
    }
}
