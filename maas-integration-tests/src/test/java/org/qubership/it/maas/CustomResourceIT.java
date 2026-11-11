package org.qubership.it.maas;

import lombok.SneakyThrows;
import okhttp3.Request;
import org.junit.jupiter.api.Test;
import org.qubership.it.maas.bg2.BGState;
import org.qubership.it.maas.bg2.BgNamespace;
import org.qubership.it.maas.bg2.BgNamespaces;
import org.yaml.snakeyaml.Yaml;

import java.io.IOException;
import java.util.Date;
import java.util.UUID;

import static org.apache.http.HttpStatus.SC_FORBIDDEN;
import static org.apache.http.HttpStatus.SC_NOT_FOUND;
import static org.apache.http.HttpStatus.SC_OK;
import static org.qubership.it.maas.MaasITHelper.*;

public class CustomResourceIT extends AbstractMaasWithInitsIT {

    @Test
    void checkCreatingTopic() throws IOException {
        var classifierName = "cr-test-" + UUID.randomUUID();
        var topicCR = """
                apiVersion: core.netcracker.com/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.netcracker.com/sessionId: 'session'
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
                apiVersion: core.netcracker.com/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.netcracker.com/sessionId: 'session'
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
                apiVersion: core.netcracker.com/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.netcracker.com/sessionId: 'session'
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

    @Test
    void checkCreatingTopic_bgPeerNamespaceAllowed() throws IOException {
        deleteNamespace(TEST_NAMESPACE_2_BG);
        Request destroyRequest = helper.createJsonRequest(BG2_DESTROY_DOMAIN, getMaasBasicAuth(),
                new BgNamespaces(TEST_NAMESPACE, TEST_NAMESPACE_2_BG, ""), "DELETE");
        helper.doRequest(destroyRequest, SC_OK, SC_NOT_FOUND);

        var bgState = new BGState(
                new BgNamespace(TEST_NAMESPACE, "active", "v1"),
                new BgNamespace(TEST_NAMESPACE_2_BG, "candidate", "v2"),
                new Date()
        );
        Request initRequest = helper.createJsonRequest(BG2_INIT_DOMAIN, getMaasBasicAuth(), bgState, "POST");
        helper.doRequest(initRequest, SC_OK);

        var classifierName = "cr-bg-peer-" + UUID.randomUUID();
        var topicCR = topicCrYaml(classifierName, TEST_NAMESPACE_2_BG);

        Request applyRequest = helper.createJsonRequestWithNamespace(
                CR_APPLY, getMaasBasicAuth(), loadYaml(topicCR), POST, TEST_NAMESPACE_2_BG);
        helper.doRequest(applyRequest, Object.class, SC_OK);

        deleteNamespace(TEST_NAMESPACE_2_BG);
        helper.doRequest(destroyRequest, SC_OK, SC_NOT_FOUND);
    }

    @Test
    void checkCreatingTopic_unboundNamespaceDenied() throws IOException {
        var foreignNamespace = "maas-it-foreign-" + UUID.randomUUID();
        var classifierName = "cr-foreign-" + UUID.randomUUID();
        var topicCR = topicCrYaml(classifierName, foreignNamespace);

        Request applyRequest = helper.createJsonRequestWithNamespace(
                CR_APPLY, getMaasBasicAuth(), loadYaml(topicCR), POST, foreignNamespace);
        helper.doRequest(applyRequest, Object.class, SC_FORBIDDEN);
    }

    private String topicCrYaml(String classifierName, String namespace) {
        return """
                apiVersion: core.netcracker.com/v1
                kind: MaaS
                subKind: Topic
                metadata:
                  name: {name}
                  namespace: {namespace}
                  labels:
                    app.kubernetes.io/instance: 'test-microservice'
                    app.kubernetes.io/managed-by: operator
                    deployer.cleanup/allow: 'true'
                    deployment.netcracker.com/sessionId: 'session'
                    app.kubernetes.io/part-of: Cloud-Core
                    app.kubernetes.io/managed-by-operator: core-operator
                spec:
                  topicNameTemplate: '%namespace%-%name%'
                  replicationFactor: inherit
                """
                .replace("{name}", classifierName)
                .replace("{namespace}", namespace);
    }

    @SneakyThrows
    private Object loadYaml(String s) {
        var yaml = new Yaml();
        return yaml.load(s);
    }
}
