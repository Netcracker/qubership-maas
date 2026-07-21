package org.qubership.it.maas.kafka;

import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.kafka.KafkaTopicResponse;
import net.jodah.failsafe.Failsafe;
import net.jodah.failsafe.RetryPolicy;
import lombok.extern.slf4j.Slf4j;
import org.apache.http.HttpStatus;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.NewTopic;
import org.junit.jupiter.api.Test;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import java.util.Properties;
import java.util.UUID;

import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

/**
 * Integration test for the registry-vs-broker discrepancy metrics (maas_discrepancy_*).
 * The maas container is started with DISCREPANCY_METRICS_INTERVAL=2s (see AbstractMaasWithInitsIT),
 * so the collector recalculates fast enough to be asserted with a short retry loop.
 */
@Slf4j
class DiscrepancyMonitoringIT extends AbstractMaasWithInitsIT {

    private static final String KAFKA = "Kafka";
    private static final String REGISTERED = "maas_discrepancy_registered_entities";
    private static final String LOST = "maas_discrepancy_lost_entities";
    private static final String GHOST = "maas_discrepancy_ghost_entities";

    // the collector runs every 2s in the IT env; poll long enough to cross a couple of cycles
    private static final RetryPolicy<Object> METRIC_RETRY = new RetryPolicy<>()
            .handle(AssertionError.class, Exception.class)
            .withMaxRetries(20)
            .withDelay(Duration.ofSeconds(2));

    @Test
    void registeredLostAndGhostAreReported() throws Exception {
        assumeTrue(getKafkaInstances().length > 0, "no kafka instances, skipping");
        String suffix = UUID.randomUUID().toString().substring(0, 8);

        Map<String, Object> classifier1 = createSimpleClassifier("disc-mon-1-" + suffix);
        Map<String, Object> classifier2 = createSimpleClassifier("disc-mon-2-" + suffix);
        KafkaTopicResponse topic1 = createKafkaTopic(HttpStatus.SC_CREATED, classifier1);
        createKafkaTopic(HttpStatus.SC_CREATED, classifier2);

        // both registered topics are reflected, nothing lost yet
        Failsafe.with(METRIC_RETRY).run(() -> {
            assertTrue(helper.sumDiscrepancyMetric(REGISTERED, KAFKA, TEST_NAMESPACE) >= 2,
                    "expected at least 2 registered entities in namespace " + TEST_NAMESPACE);
            assertEquals(0, helper.sumDiscrepancyMetric(LOST, KAFKA, TEST_NAMESPACE), "no lost entities expected initially");
        });

        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "no default kafka or unsupported auth mechanism, skipping");

        // delete a registered topic directly on the broker -> it must be reported as "lost"
        deleteKafkaTopic(kafkaProp, topic1.getName());
        Failsafe.with(METRIC_RETRY).run(() ->
                assertTrue(helper.sumDiscrepancyMetric(LOST, KAFKA, TEST_NAMESPACE) >= 1,
                        "expected >=1 lost entity after broker-side delete of " + topic1.getName()));

        // create a maas.* topic directly on the broker that maas does not know about -> "ghost"
        String ghostTopic = "maas." + TEST_NAMESPACE + ".ghost-" + suffix;
        createTopicOnBroker(kafkaProp, ghostTopic);
        Failsafe.with(METRIC_RETRY).run(() ->
                assertTrue(helper.sumDiscrepancyMetric(GHOST, KAFKA, TEST_NAMESPACE) >= 1,
                        "expected >=1 ghost entity after creating " + ghostTopic + " on the broker"));

        deleteKafkaTopic(kafkaProp, ghostTopic);
    }

    private void createTopicOnBroker(Properties props, String topicName) throws Exception {
        try (AdminClient adminClient = AdminClient.create(props)) {
            adminClient.createTopics(List.of(new NewTopic(topicName, 1, (short) 1))).all().get();
        }
    }
}
