package org.qubership.it.maas.rabbit;

import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.rabbit.VirtualHostResponse;
import net.jodah.failsafe.Failsafe;
import net.jodah.failsafe.RetryPolicy;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Credentials;
import okhttp3.OkHttpClient;
import okhttp3.Request;
import okhttp3.Response;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.time.Duration;
import java.util.UUID;

import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * Integration test for the registry-vs-broker discrepancy metrics on the RabbitMQ side.
 * The maas container runs with DISCREPANCY_METRICS_INTERVAL=2s (see AbstractMaasWithInitsIT).
 * Vhosts have no comparable configuration, so only registered/lost are checked for rabbit.
 */
@Slf4j
class DiscrepancyMonitoringIT extends AbstractMaasWithInitsIT {

    private static final String RABBIT = "RabbitMQ";
    private static final String REGISTERED = "maas_discrepancy_registered_entities";
    private static final String LOST = "maas_discrepancy_lost_entities";

    private static final RetryPolicy<Object> METRIC_RETRY = new RetryPolicy<>()
            .handle(AssertionError.class, Exception.class)
            .withMaxRetries(20)
            .withDelay(Duration.ofSeconds(2));

    private final OkHttpClient rabbitClient = new OkHttpClient();

    @Test
    void registeredAndLostAreReportedForRabbit() throws Exception {
        String suffix = UUID.randomUUID().toString().substring(0, 8);

        VirtualHostResponse vhost1 = createVirtualHost(HttpStatus.SC_CREATED,
                createSimpleClassifier("disc-mon-rabbit-1-" + suffix, "", TEST_NAMESPACE));
        createVirtualHost(HttpStatus.SC_CREATED,
                createSimpleClassifier("disc-mon-rabbit-2-" + suffix, "", TEST_NAMESPACE));

        // both registered vhosts are reflected, nothing lost yet
        Failsafe.with(METRIC_RETRY).run(() -> {
            assertTrue(helper.sumDiscrepancyMetric(REGISTERED, RABBIT, TEST_NAMESPACE) >= 2,
                    "expected at least 2 registered rabbit vhosts in namespace " + TEST_NAMESPACE);
            assertEquals(0, helper.sumDiscrepancyMetric(LOST, RABBIT, TEST_NAMESPACE), "no lost vhosts expected initially");
        });

        // delete a registered vhost directly on the broker -> it must be reported as "lost"
        String vhostName = helper.getVhostFromCnn(vhost1.getCnn());
        deleteVhostOnBroker(vhostName);
        Failsafe.with(METRIC_RETRY).run(() ->
                assertTrue(helper.sumDiscrepancyMetric(LOST, RABBIT, TEST_NAMESPACE) >= 1,
                        "expected >=1 lost vhost after broker-side delete of " + vhostName));
    }

    // vhosts are created on the default (first registered) rabbit instance = RABBITMQ_CONTAINER_1
    private void deleteVhostOnBroker(String vhost) throws IOException {
        String url = RABBITMQ_CONTAINER_1.getHttpUrl() + "/api/vhosts/" + vhost;
        Request request = new Request.Builder()
                .url(url)
                .header("Authorization", Credentials.basic(
                        RABBITMQ_CONTAINER_1.getAdminUsername(), RABBITMQ_CONTAINER_1.getAdminPassword()))
                .delete()
                .build();
        try (Response response = rabbitClient.newCall(request).execute()) {
            assertTrue(response.isSuccessful(),
                    "DELETE vhost " + vhost + " on broker failed with code " + response.code());
        }
    }
}
