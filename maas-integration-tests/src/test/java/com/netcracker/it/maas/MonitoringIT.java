package com.netcracker.it.maas;

import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Arrays;
import java.util.Objects;
import java.util.Set;
import java.util.UUID;
import java.util.stream.Collectors;

import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class MonitoringIT extends AbstractMaasWithInitsIT {

    @Test
    void cleanStatsOnCleanNamespace() throws IOException {
        String randomSuffix = UUID.randomUUID().toString().substring(0, 8);
        deleteNamespace("another-" + TEST_NAMESPACE);

        Set<String> matchedAuditRecords = filterRecords(getRequestAudit(), randomSuffix);
        assertTrue(matchedAuditRecords.isEmpty(),
                "Unexpected records were found with random suffix: '" + randomSuffix + "'. Records: " + matchedAuditRecords);

        createKafkaTopic(HttpStatus.SC_CREATED, createSimpleClassifier("MonitoringIT-1-" + randomSuffix, "it-test"));
        createKafkaTopic(HttpStatus.SC_OK, createSimpleClassifier("MonitoringIT-1-" + randomSuffix, "it-test"));
        createVirtualHost(HttpStatus.SC_CREATED, createSimpleClassifier("MonitoringIT-1-" + randomSuffix, "it-test"));
        createVirtualHost(HttpStatus.SC_OK, createSimpleClassifier("MonitoringIT-1-" + randomSuffix, "it-test"));

        assertFalse(filterRecords(getRequestAudit(), randomSuffix).isEmpty());

        deleteNamespace(TEST_NAMESPACE);
        initAccount(agentAccount, TEST_NAMESPACE);

        assertTrue(filterRecords(getRequestAudit(), randomSuffix).isEmpty());
    }

    private Set<String> getRequestAudit() throws IOException {
        Request request = helper.createJsonRequest(MaasITHelper.MONITORING_ENTITY_REQUEST_AUDIT_PATH, "", null, MaasITHelper.GET);
        String responseBody = helper.doRequestWithStringResponse(request, 200);
        return Arrays.stream(responseBody.split("\n")).filter(e -> e != null && !e.isBlank()).collect(Collectors.toSet());
    }

    private Set<String> filterRecords(Set<String> records, String value) {
        return records.stream().filter(Objects::nonNull).filter(r -> r.contains(value)).collect(Collectors.toSet());
    }
}
