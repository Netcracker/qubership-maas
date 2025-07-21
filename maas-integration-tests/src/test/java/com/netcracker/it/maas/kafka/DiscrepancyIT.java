package com.netcracker.it.maas.kafka;

import com.google.gson.Gson;
import com.google.gson.reflect.TypeToken;
import com.netcracker.it.maas.AbstractMaasWithInitsIT;
import com.netcracker.it.maas.MaasITHelper;
import com.netcracker.it.maas.entity.kafka.KafkaTopicResponse;
import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.ToString;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Properties;

import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

@Slf4j
class DiscrepancyIT extends AbstractMaasWithInitsIT {

    private final static String OK_STATUS = "ok";
    private final static String ABSENT_STATUS = "absent";

    @Test
    void discrepancyReport() throws Exception {
        Map<String, Object> firstTopicClassifier = createSimpleClassifier("first-test-topic", "it-test");
        KafkaTopicResponse firstTopic = createKafkaTopic(HttpStatus.SC_CREATED, firstTopicClassifier);
        Map<String, Object> secondTopicClassifier = createSimpleClassifier("second-test-topic", "it-test");
        KafkaTopicResponse secondTopic = createKafkaTopic(HttpStatus.SC_CREATED, secondTopicClassifier);
        List<DiscrepancyItem> discrepancy = getDiscrepancyReport();
        log.info("Discrepancy items before topic deletion: {}", discrepancy);

        assertEquals(firstTopic.getName(), discrepancy.get(0).getName());
        assertEquals(firstTopicClassifier, discrepancy.get(0).getClassifier());
        assertEquals(OK_STATUS, discrepancy.get(0).getStatus());

        assertEquals(secondTopic.getName(), discrepancy.get(1).getName());
        assertEquals(secondTopicClassifier, discrepancy.get(1).getClassifier());
        assertEquals(OK_STATUS, discrepancy.get(1).getStatus());

        Properties kafkaProp = preparePropertiesAndPortForwardKafka();
        assumeTrue(kafkaProp != null, "There is no default kafka or incorrect auth mechanism. Skip tests");
        deleteKafkaTopic(kafkaProp, firstTopic.getName());
        discrepancy = getDiscrepancyReport();
        log.info("Discrepancy items after topic deletion: {}", discrepancy);

        assertEquals(firstTopic.getName(), discrepancy.get(0).getName());
        assertEquals(firstTopicClassifier, discrepancy.get(0).getClassifier());
        assertEquals(ABSENT_STATUS, discrepancy.get(0).getStatus());

        assertEquals(secondTopic.getName(), discrepancy.get(1).getName());
        assertEquals(secondTopicClassifier, discrepancy.get(1).getClassifier());
        assertEquals(OK_STATUS, discrepancy.get(1).getStatus());

        deleteKafkaTopic(kafkaProp, secondTopic.getName());
    }

    private List<DiscrepancyItem> getDiscrepancyReport() throws IOException {
        Request request = helper.createJsonRequest(MaasITHelper.DISCREPANCY_REPORT_PATH + "/" + TEST_NAMESPACE, "", null, MaasITHelper.GET);
        String discrepancyJson = helper.doRequestWithStringResponse(request, 200);
        List<DiscrepancyItem> discrepancy = new Gson().fromJson(discrepancyJson, new TypeToken<List<DiscrepancyItem>>() {
        }.getType());
        discrepancy.sort(Comparator.comparing(DiscrepancyItem::getName));
        return discrepancy;
    }

    @Getter
    @ToString
    @AllArgsConstructor
    static class DiscrepancyItem {
        private String name;
        private Map<String, String> classifier;
        private String status;
    }
}
