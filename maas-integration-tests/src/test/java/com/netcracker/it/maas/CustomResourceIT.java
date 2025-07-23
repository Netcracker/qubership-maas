package com.netcracker.it.maas;

import lombok.SneakyThrows;
import okhttp3.Request;
import org.junit.jupiter.api.Test;
import org.yaml.snakeyaml.Yaml;

import java.io.IOException;
import java.util.UUID;

import static com.netcracker.it.maas.MaasITHelper.*;
import static org.apache.http.HttpStatus.SC_OK;

public class CustomResourceIT extends AbstractMaasWithInitsIT {

    @Test
    void checkCreatingTopic() throws IOException {
        var classifierName = "cr-test-" + UUID.randomUUID();
        var topicCR = """
                apiVersion: core.qubership.org/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.qubership.org/sessionId: 'session'
                    app.kubernetes.io/part-of: Cloud-Core
                    app.kubernetes.io/managed-by-operator: core-operator
                spec:
                  topicNameTemplate: '%namespace%-%name%'
                  replicationFactor: inherit
                """
                .replace("{name}", classifierName)
                .replace("{namespace}", TEST_NAMESPACE);

        //todo fix replaces

        Request request = helper.createJsonRequestWithNamespace(CR_APPLY, getMaasBasicAuth(), loadYaml(topicCR), POST, TEST_NAMESPACE);
        helper.doRequest(request, Object.class, SC_OK);
    }

    @Test
    void checkCreatingTopic_replicationFactor_int() throws IOException {
        var classifierName = "cr-test-" + UUID.randomUUID();
        var topicCR = """
                apiVersion: core.qubership.org/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.qubership.org/sessionId: 'session'
                    app.kubernetes.io/part-of: Cloud-Core
                    app.kubernetes.io/managed-by-operator: core-operator
                spec:
                  topicNameTemplate: '%namespace%-%name%'
                  replicationFactor: -1
                """
                .replace("{name}", classifierName)
                .replace("{namespace}", TEST_NAMESPACE);

        Request request = helper.createJsonRequestWithNamespace(CR_APPLY, getMaasBasicAuth(), loadYaml(topicCR), POST, TEST_NAMESPACE);
        helper.doRequest(request, Object.class, SC_OK);
    }

    @Test
    void checkCreatingTopic_replicationFactor_inherit() throws IOException {
        var classifierName = "cr-test-" + UUID.randomUUID();
        var topicCR = """
                apiVersion: core.qubership.org/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.qubership.org/sessionId: 'session'
                    app.kubernetes.io/part-of: Cloud-Core
                    app.kubernetes.io/managed-by-operator: core-operator
                spec:
                  topicNameTemplate: '%namespace%-%name%'
                  replicationFactor: inherit
                """
                .replace("{name}", classifierName)
                .replace("{namespace}", TEST_NAMESPACE);

        Request request = helper.createJsonRequestWithNamespace(CR_APPLY, getMaasBasicAuth(), loadYaml(topicCR), POST, TEST_NAMESPACE);
        helper.doRequest(request, Object.class, SC_OK);
    }

    @SneakyThrows
    private Object loadYaml(String s) {
        var yaml = new Yaml();
        return yaml.load(s);
    }
}
