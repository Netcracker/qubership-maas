package org.qubership.it.maas.kafka;

import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.kafka.KafkaTopicRequest;
import org.qubership.it.maas.entity.kafka.KafkaTopicResponse;
import net.jodah.failsafe.Failsafe;
import net.jodah.failsafe.RetryPolicy;
import lombok.extern.slf4j.Slf4j;
import org.apache.http.HttpStatus;
import org.apache.kafka.clients.admin.AdminClient;
import org.apache.kafka.clients.admin.NewPartitions;
import org.junit.jupiter.api.Test;

import java.time.Duration;
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
    private static final String MISMATCHED = "maas_discrepancy_mismatched_entities";

    // the collector runs every 2s in the IT env; poll long enough to cross a couple of cycles
    private static final RetryPolicy<Object> METRIC_RETRY = new RetryPolicy<>()
            .handle(AssertionError.class, Exception.class)
            .withMaxRetries(20)
            .withDelay(Duration.ofSeconds(2));

    @Test
    void registeredLostAndMismatchedAreReported() throws Exception {
        assumeTrue(getKafkaInstances().length > 0, "no kafka instances, skipping");
        String suffix = UUID.randomUUID().toString().substring(0, 8);

        KafkaTopicResponse lostTopic = createKafkaTopic(HttpStatus.SC_CREATED, KafkaTopicRequest.builder()
                .classifier(createSimpleClassifier("disc-mon-lost-" + suffix)).numPartitions(1).build());
        KafkaTopicResponse mismatchTopic = createKafkaTopic(HttpStatus.SC_CREATED, KafkaTopicRequest.builder()
                .classifier(createSimpleClassifier("disc-mon-mismatch-" + suffix)).numPartitions(2).build());

        // both registered and in sync
        Failsafe.with(METRIC_RETRY).run(() -> {
            assertTrue(helper.sumDiscrepancyMetric(REGISTERED, KAFKA, TEST_NAMESPACE) >= 2,
                    "expected at least 2 registered entities in namespace " + TEST_NAMESPACE);
            assertEquals(0, helper.sumDiscrepancyMetric(LOST, KAFKA, TEST_NAMESPACE), "no lost entities expected initially");
            assertEquals(0, helper.sumDiscrepancyMetric(MISMATCHED, KAFKA, TEST_NAMESPACE), "no mismatched entities expected initially");
        });

        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "no default kafka or unsupported auth mechanism, skipping");

        // delete a registered topic directly on the broker -> it must be reported as "lost"
        deleteKafkaTopic(kafkaProp, lostTopic.getName());
        Failsafe.with(METRIC_RETRY).run(() ->
                assertTrue(helper.sumDiscrepancyMetric(LOST, KAFKA, TEST_NAMESPACE) >= 1,
                        "expected >=1 lost entity after broker-side delete of " + lostTopic.getName()));

        // change the partition count of a registered topic on the broker -> "mismatched"
        increasePartitionsOnBroker(kafkaProp, mismatchTopic.getName(), 3);
        Failsafe.with(METRIC_RETRY).run(() ->
                assertTrue(helper.sumDiscrepancyMetric(MISMATCHED, KAFKA, TEST_NAMESPACE) >= 1,
                        "expected >=1 mismatched entity after repartitioning " + mismatchTopic.getName()));
    }

    private void increasePartitionsOnBroker(Properties props, String topicName, int newPartitionCount) throws Exception {
        try (AdminClient adminClient = AdminClient.create(props)) {
            adminClient.createPartitions(Map.of(topicName, NewPartitions.increaseTo(newPartitionCount))).all().get();
        }
    }
}
