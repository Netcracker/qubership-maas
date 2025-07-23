package com.netcracker.it.maas.kafka;

import com.netcracker.it.maas.AbstractMaasWithInitsIT;
import com.netcracker.it.maas.bg2.BGState;
import com.netcracker.it.maas.bg2.BgNamespace;
import com.netcracker.it.maas.bg2.BgNamespaces;
import com.netcracker.it.maas.entity.kafka.KafkaConfigReq;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import org.junit.jupiter.api.Test;

import java.util.Collections;
import java.util.Date;

import static com.netcracker.it.maas.MaasITHelper.*;
import static org.apache.http.HttpStatus.*;

@Slf4j
class ApplyKafkaBlueGreenConfigIT extends AbstractMaasWithInitsIT {
    @Test
    void kafkaBg2AllowedNamespaces() throws Exception {
        Request request = helper.createJsonRequest(BG2_DESTROY_DOMAIN, getMaasBasicAuth(), new BgNamespaces(TEST_NAMESPACE, TEST_NAMESPACE_2_BG, ""), "DELETE");
        helper.doRequest(request, SC_OK, SC_NOT_FOUND);

        KafkaConfigReq kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec("ApplyKafkaBlueGreenConfigIT", createSimpleClassifier("ApplyKafkaBlueGreenConfigIT", "it-test", TEST_NAMESPACE), false))
                .build();
        request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), Collections.singletonList(kafkaConfigReq), "POST");
        helper.doRequest(request, null, SC_OK);

        kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec("ApplyKafkaBlueGreenConfigIT", createSimpleClassifier("ApplyKafkaBlueGreenConfigIT", "it-test", TEST_NAMESPACE_2_BG), false))
                .build();
        request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), Collections.singletonList(kafkaConfigReq), "POST");
        helper.doRequest(request, SC_INTERNAL_SERVER_ERROR);

        BGState bgNamespacesPair = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "idle", null),
                new Date()
        );

        request = helper.createJsonRequest(BG2_INIT_DOMAIN, getMaasBasicAuth(), bgNamespacesPair, "POST");
        helper.doRequest(request, SC_OK);

        BGState bgState = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "candidate", "v2"),
                new Date());
        request = helper.createJsonRequest(BG2_WARMUP, getMaasBasicAuth(), bgState, "POST");
        helper.doRequest(request, SC_OK);

        kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec("ApplyKafkaBlueGreenConfigIT", createSimpleClassifier("ApplyKafkaBlueGreenConfigIT", "it-test", TEST_NAMESPACE), false))
                .build();
        request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), Collections.singletonList(kafkaConfigReq), "POST");
        helper.doRequest(request, SC_OK);

        kafkaConfigReq = KafkaConfigReq.builder()
                .apiVersion(API_KAFKA_VERSION)
                .kind(KIND_KAFKA)
                .spec(new KafkaConfigReq.Spec("ApplyKafkaBlueGreenConfigIT", createSimpleClassifier("ApplyKafkaBlueGreenConfigIT", "it-test", TEST_NAMESPACE_2_BG), false))
                .build();
        request = helper.createYamlRequest(APPLY_CONFIG_PATH, getMaasBasicAuth(), Collections.singletonList(kafkaConfigReq), "POST");
        helper.doRequest(request, SC_OK);
    }
}
