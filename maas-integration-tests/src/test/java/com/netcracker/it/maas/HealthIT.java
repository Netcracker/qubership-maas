package com.netcracker.it.maas;

import com.netcracker.it.maas.entity.HealthResponse;
import com.netcracker.it.maas.entity.HealthItem;
import com.netcracker.it.maas.entity.kafka.KafkaTopicResponse;
import com.netcracker.it.maas.entity.rabbit.RabbitInstance;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;
import org.testcontainers.shaded.org.awaitility.Awaitility;

import java.io.IOException;
import java.util.Arrays;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.concurrent.TimeUnit;
import java.util.stream.Collectors;

import static com.netcracker.it.maas.MaasITHelper.*;
import static com.netcracker.it.maas.Utils.retry;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertEquals;

@Slf4j
class HealthIT extends AbstractMaasWithInitsIT {

    @Test
    public void checkHealthStatus() throws Exception {
        retry(20, () -> {
            log.info("Check maas health status");
            Request request = helper.createJsonRequest("health", "", null, MaasITHelper.GET);
            HealthResponse actualHealth = helper.doRequest(request, HealthResponse.class, 200);
            log.info("Actual health response {}", actualHealth);
            HealthResponse expectHealth = new HealthResponse(
                    new HealthItem("UP"),
                    new HealthItem("UP"),
                    new HealthItem("UP"));
            assertEquals(expectHealth, actualHealth);
        });
    }

    @Test
    public void checkMonitoring() throws IOException, InterruptedException {
        Thread.sleep(1_000); // wait until instance registration. Correlated with health.check.interval
        VirtualHostResponse virtualHost = createVirtualHost(HttpStatus.SC_CREATED);
        KafkaTopicResponse topic = createKafkaTopic(HttpStatus.SC_CREATED);

        String cfg = "apiVersion: nc.maas.config/v2\n" +
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
                "                  type: direct\n";

        applyConfigV2(200, cfg);

        RabbitInstance rabbitInstance = getDefaultRabbitInstance();
        getDefaultKafkaInstance();

        StringBuilder address = new StringBuilder();
        for (Map.Entry<String, List<String>> entry : topic.getAddresses().entrySet()) {
            address.append(String.format("{\"%s\": [\"%s\"]}", entry.getKey(), entry.getValue().get(0)));
        }

        String rabbitString = String.format("maas_rabbit{namespace=\"%s\", broker_host=\"%s\", microservice=\"%s\", vhost=\"%s\", broker_status=\"%s\"} 1", TEST_NAMESPACE, rabbitInstance.getAmqpUrl(), "", helper.getVhostFromCnn(virtualHost.getCnn()), "UP");
        String rabbitStringConfig = String.format("maas_rabbit{namespace=\"%s\", broker_host=\"%s\", microservice=\"%s\", vhost=\"%s\", broker_status=\"%s\"} 1", TEST_NAMESPACE, rabbitInstance.getAmqpUrl(), "order-processor", "maas." + TEST_NAMESPACE + ".vers-test", "UP");
        String kafkaString = String.format("maas_kafka{namespace=\"%s\", broker_host=\"%s\", topic=\"%s\", broker_status=\"%s\"} 1", TEST_NAMESPACE, address, topic.getName(), "UP");

        Request request = helper.createJsonRequest(MONITORING_PATH, getMaasBasicAuth(), null, GET);
        String monitoringResponse = helper.doRequestWithStringResponse(request, HttpStatus.SC_OK);
        String filteredMonitoringResponse = Arrays.stream(monitoringResponse.split("\r\n|\r|\n"))
                .filter(Objects::nonNull)
                .filter(s -> s.contains("namespace=\"%s\"".formatted(TEST_NAMESPACE)))
                .collect(Collectors.joining(System.lineSeparator()));

        log.info("MaaS monitoring response: {}", filteredMonitoringResponse);

        assertEquals(3, countLines(filteredMonitoringResponse));
        assertThat(filteredMonitoringResponse, CoreMatchers.containsString(rabbitString));
        assertThat(filteredMonitoringResponse, CoreMatchers.containsString(rabbitStringConfig));
        assertThat(filteredMonitoringResponse, CoreMatchers.containsString(kafkaString));
    }

    private static int countLines(String str) {
        String[] lines = str.split("\r\n|\r|\n");
        return lines.length;
    }

}
