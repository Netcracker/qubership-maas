package com.netcracker.it.maas.rabbit;


import com.netcracker.it.maas.entity.SearchCriteria;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import com.rabbitmq.client.AuthenticationFailureException;
import com.rabbitmq.client.Channel;
import com.rabbitmq.client.Connection;
import lombok.extern.slf4j.Slf4j;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;

import java.util.Map;

import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.fail;

@Slf4j
class RabbitAdapterIT extends RabbitTest {

    @Test
    public void checkRabbitConnect() throws Exception {
        VirtualHostResponse virtualHost = createVirtualHost(201);
        log.info("Created virtual host {}", virtualHost);
        try (Connection connection = createRabbitConnect(virtualHost)) {
            Channel channel = connection.createChannel();
            channel.queueDeclare("test_queue", false, false, false, null);
            String message = "some message";
            channel.basicPublish("", "test_queue", null, message.getBytes());
            channel.close();
        }
    }

    @Test
    public void checkConnectAfterDeleting() throws Exception {
        Map<String, Object> classifier = createSimpleClassifier("VirtualHostBasicOperationsIT", "it-test");
        VirtualHostResponse virtualHost = createVirtualHost(201, classifier);
        log.info("Created virtual host {}", virtualHost);
        deleteVhost(SearchCriteria.builder().classifier(classifier).build(), 204);
        try {
            createRabbitConnect(virtualHost, false);
        } catch (AuthenticationFailureException ex) {
            assertThat(ex.getMessage(), CoreMatchers.containsString("ACCESS_REFUSED - Login was refused using authentication mechanism PLAIN"));
            return;
        }
        fail("virtual wasn't deleted and we could perform connect");
    }
}
