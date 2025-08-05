package org.qubership.it.maas.rabbit;

import org.qubership.it.maas.MaasITHelper;
import org.qubership.it.maas.entity.rabbit.RabbitInstance;
import org.qubership.it.maas.entity.rabbit.VirtualHostResponse;
import net.jodah.failsafe.Failsafe;
import okhttp3.Request;
import org.apache.commons.lang3.StringUtils;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Assumptions;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Arrays;
import java.util.Objects;
import java.util.Set;
import java.util.UUID;
import java.util.stream.Collectors;

import static org.qubership.it.maas.MaasITHelper.TEST_MICROSERVICE;
import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;

class MonitoringIT extends RabbitTest {

    String auditResTemplate = "maas_%s_entity_requests{namespace=\"%s\", microservice=\"%s\", %s=\"%s\", instance=\"%s\"} %d";

    @Test
    void addStatOnCreateVhost() throws IOException {
        var rabbitInstances = getRabbitInstances();
        Assumptions.assumeTrue(rabbitInstances.length > 0, "no rabbit instances, skipping");
        var defaultRabbitInstanceId = Arrays.stream(rabbitInstances).filter(RabbitInstance::getIsDefault).findFirst().get().getId();
        String randomSuffix = UUID.randomUUID().toString().substring(0, 8);
        Set<String> matchedAuditRecords = filterRecords(getRequestAudit(), randomSuffix);
        assertTrue(matchedAuditRecords.isEmpty(),
                "Unexpected records were found with random suffix: '" + randomSuffix + "'. Records: " + matchedAuditRecords);

        VirtualHostResponse vhost1 = createVirtualHost(HttpStatus.SC_CREATED, createSimpleClassifier("MonitoringIT-1-" + randomSuffix, "it-test"));
        // retry because audit is async operation on maas side
        Failsafe.with(DEFAULT_RETRY_POLICY).run(() -> {
            Set<String> expectedRecords = Set.of(getRabbitAuditRecord(vhost1, defaultRabbitInstanceId, 1));
            Set<String> actualRecords = filterRecords(getRequestAudit(), randomSuffix);
            assertEquals(expectedRecords, actualRecords);
        });

        VirtualHostResponse vhost2 = createVirtualHost(HttpStatus.SC_OK, createSimpleClassifier("MonitoringIT-1-" + randomSuffix, "it-test"));
        Failsafe.with(DEFAULT_RETRY_POLICY).run(() -> {
            Set<String> expectedRecords = Set.of(getRabbitAuditRecord(vhost2, defaultRabbitInstanceId, 2));
            Set<String> actualRecords = filterRecords(getRequestAudit(), randomSuffix);
            assertEquals(expectedRecords, actualRecords);
        });
        VirtualHostResponse vhost3 = createVirtualHost(HttpStatus.SC_CREATED, createSimpleClassifier("MonitoringIT-2-" + randomSuffix, "it-test"));
        Failsafe.with(DEFAULT_RETRY_POLICY).run(() -> {
            Set<String> expectedRecords = Set.of(getRabbitAuditRecord(vhost2, defaultRabbitInstanceId, 2), getRabbitAuditRecord(vhost3, defaultRabbitInstanceId, 1));
            Set<String> actualRecords = filterRecords(getRequestAudit(), randomSuffix);
            assertEquals(expectedRecords, actualRecords);
        });
    }

    private Set<String> getRequestAudit() throws IOException {
        Request request = helper.createJsonRequest(MaasITHelper.MONITORING_ENTITY_REQUEST_AUDIT_PATH, "", null, MaasITHelper.GET);
        String responseBody = helper.doRequestWithStringResponse(request, 200);
        return Arrays.stream(responseBody.split("\n")).filter(e -> e != null && !e.isBlank()).collect(Collectors.toSet());
    }

    private String cnnToVhost(String cnn) {
        String[] parts = StringUtils.split(cnn, "/");
        return parts[parts.length - 1];
    }

    private String getRabbitAuditRecord(VirtualHostResponse vhost, String instanceId, int count) {
        return String.format(auditResTemplate, "rabbit", TEST_NAMESPACE, TEST_MICROSERVICE, "vhost", cnnToVhost(vhost.getCnn()), instanceId, count);
    }

    private Set<String> filterRecords(Set<String> records, String value) {
        return records.stream().filter(Objects::nonNull).filter(r -> r.contains(value)).collect(Collectors.toSet());
    }
}
