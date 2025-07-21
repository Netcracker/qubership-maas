package com.netcracker.it.maas.rabbit;

import com.fasterxml.jackson.databind.ObjectMapper;
import com.netcracker.it.maas.entity.SearchCriteria;
import com.netcracker.it.maas.entity.rabbit.VirtualHostResponse;
import lombok.extern.slf4j.Slf4j;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.net.URL;
import java.sql.*;
import java.util.Map;

import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.*;

@Slf4j
class VirtualHostBasicOperationsIT extends RabbitTest {

    @Test
    public void checkCreatingVirtualHost() throws IOException {
        VirtualHostResponse virtualHost = createVirtualHost(201);
        assertThat(virtualHost.getCnn(), CoreMatchers.anyOf(CoreMatchers.containsString("amqp://"), CoreMatchers.containsString("amqps://")));
        assertNotNull(virtualHost.getUsername());
        assertThat(virtualHost.getPassword(), CoreMatchers.containsString("plain:"));
    }

    @Test
    public void alreadyCreatedVhost() throws IOException {
        createVirtualHost(201);
        VirtualHostResponse virtualHost = createVirtualHost(200);// http status should be 200
        assertThat(virtualHost.getCnn(), CoreMatchers.containsString(String.format("%s.%s.%s", TEST_NAMESPACE, "it-test", "VirtualHostBasicOperationsIT")));
    }

    @Test
    public void createVhostWithPgConnect() throws IOException, SQLException {
        VirtualHostResponse virtualHost = createVirtualHost(201);
        int count = 0;
        String vhost = null, user = null, password = null, namespace = null, classifier = null;
        try (Connection connection = createPgConnect();
             Statement statement = connection.createStatement()) {
            try {
                assertTrue(statement.executeQuery("select 1;").next(), "Expect postgresql to return 1 row");
            } catch (Throwable ex) {
                throw new RuntimeException(ex);
            }
            String query = String.format("SELECT * FROM rabbit_vhosts as reg WHERE reg.user = '%s' and reg.password = '%s'", virtualHost.getUsername(), virtualHost.getPassword());
            log.info("query: {}", query);
            ResultSet rs = statement.executeQuery(query);
            while (rs.next()) {
                count++;
                vhost = rs.getString("vhost");
                user = rs.getString("user");
                password = rs.getString("password");
                namespace = rs.getString("namespace");
                classifier = rs.getString("classifier");
                log.info("Selected row vhost {}, user {}, password {}, namespace {}, classifier {}", vhost, user, password, namespace, classifier);
            }
        }
        assertEquals(1, count);
        assertEquals(virtualHost.getUsername(), user);
        assertEquals(virtualHost.getPassword(), password);
        assertEquals(TEST_NAMESPACE, namespace);
        assertEquals(createSimpleClassifier("VirtualHostBasicOperationsIT", "it-test"), jsonStringToMap(classifier));
        assertThat(virtualHost.getCnn(), CoreMatchers.containsString(vhost));
    }

    @Test
    public void deleteVirtualHost() throws IOException, SQLException {
        Map<String, Object> vHostClassifier = createSimpleClassifier("VirtualHostBasicOperationsIT", "it-test");
        SearchCriteria searchForm = SearchCriteria.builder().classifier(vHostClassifier).build();
        deleteVhost(searchForm, 404);
        VirtualHostResponse virtualHost = createVirtualHost(201);
        deleteVhost(searchForm, 204);
        deleteVhost(searchForm, 404);
        try (Connection connection = createPgConnect();
             Statement statement = connection.createStatement()) {
            try {
                assertTrue(statement.executeQuery("select 1;").next(), "Expect postgresql to return 1 row");
            } catch (Throwable ex) {
                throw new RuntimeException(ex);
            }
            String query = String.format("SELECT * FROM rabbit_vhosts as reg WHERE reg.user = '%s' and reg.password = '%s'", virtualHost.getUsername(), virtualHost.getPassword());
            log.info("query: {}", query);
            ResultSet rs = statement.executeQuery(query);
            while (rs.next()) {
                fail("Virual host was not deleted");
            }
        }
    }

    private Map jsonStringToMap(String str) throws IOException {
        return new ObjectMapper().readValue(str, Map.class);
    }

    private Connection createPgConnect() {
        log.info("createPgConnect: MaasITHelper is {}", helper);
        String pgHostAddress = helper.getSecretEnv("maas-db-postgresql-credentials-secret", "pg_address");
        String pgDb = helper.getSecretEnv("maas-db-postgresql-credentials-secret", "dbname");
        String pgUsername = helper.getSecretEnv("maas-db-postgresql-credentials-secret", "username");
        String pgPassword = helper.getSecretEnv("maas-db-postgresql-credentials-secret", "password");
        log.info("Got pg host address {}, db {}, username {}, pass {}", pgHostAddress, pgDb, pgUsername, pgPassword);
        assertNotNull(pgHostAddress);
        assertNotNull(pgDb);
        assertNotNull(pgUsername);
        assertNotNull(pgPassword);

        String url;
        String serviceNamespace = helper.getServiceNamespaceFromUrl(pgHostAddress);
        String serviceName = helper.getServiceNameFromUrl(pgHostAddress);
        if (isInternalKubernetesService(serviceNamespace, serviceName)) {
            URL pgConnect = portForwardService.createPortForward(serviceNamespace, serviceName);
            url = String.format("jdbc:postgresql://%s:%d/%s", pgConnect.getHost(), pgConnect.getPort(), pgDb);
        } else {
            url = String.format("jdbc:postgresql://%s/%s", pgHostAddress, pgDb);
        }

        try {
            return DriverManager.getConnection(url, pgUsername, pgPassword);
        } catch (SQLException ex) {
            throw new RuntimeException(ex);
        }
    }
}
