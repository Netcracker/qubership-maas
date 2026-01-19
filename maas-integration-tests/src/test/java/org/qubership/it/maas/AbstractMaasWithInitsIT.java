package org.qubership.it.maas;

import lombok.extern.slf4j.Slf4j;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.TestInfo;
import org.testcontainers.containers.*;
import org.testcontainers.images.builder.ImageFromDockerfile;
import org.testcontainers.utility.DockerImageName;
import org.testcontainers.utility.MountableFile;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.nio.file.attribute.PosixFilePermission;
import java.util.Base64;
import java.util.Map;
import java.util.Set;

import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.junit.jupiter.api.Assertions.assertTrue;

@Slf4j
public abstract class AbstractMaasWithInitsIT extends AbstractMaasIT {

    protected static final Network TEST_NETWORK = Network.newNetwork();

    protected static final KafkaContainer KAFKA_CONTAINER_1 = new KafkaContainer(DockerImageName.parse("confluentinc/cp-kafka:7.5.1")).withNetwork(TEST_NETWORK);
    protected static final KafkaContainer KAFKA_CONTAINER_2 = new KafkaContainer(DockerImageName.parse("confluentinc/cp-kafka:7.5.1")).withNetwork(TEST_NETWORK);

    protected static final RabbitMQContainer RABBITMQ_CONTAINER_1 = new RabbitMQContainer(DockerImageName.parse("rabbitmq:3.12-management")).withNetwork(TEST_NETWORK);
    protected static final RabbitMQContainer RABBITMQ_CONTAINER_2 = new RabbitMQContainer(DockerImageName.parse("rabbitmq:3.12-management")).withNetwork(TEST_NETWORK);

    protected static final PostgreSQLContainer<?> POSTGRES_CONTAINER = new PostgreSQLContainer<>(DockerImageName.parse("postgres:16.2")).withNetwork(TEST_NETWORK);

    private static final int OIDC_SERVER_PORT = 55199;
    private static final String OIDC_SERVER_HOSTNAME = "oidc-server";
    private static final Path oidcTokenTempFile;
    protected static final K8sAuthHelper k8sAuthHelper;

    protected static final GenericContainer<?> OIDC_SERVER_CONTAINER = new GenericContainer<>("ghcr.io/navikt/mock-oauth2-server:latest")
            .withExposedPorts(OIDC_SERVER_PORT)
            .withNetwork(TEST_NETWORK)
            .withNetworkAliases(OIDC_SERVER_HOSTNAME)
            .withEnv(Map.of(
                    "SERVER_PORT", String.valueOf(OIDC_SERVER_PORT),
                    "SERVER_HOSTNAME", OIDC_SERVER_HOSTNAME
            ));

    static {
        try {
            oidcTokenTempFile = Files.createTempFile("", "");
            k8sAuthHelper = new K8sAuthHelper("http://%s:%s/default".formatted(OIDC_SERVER_HOSTNAME, OIDC_SERVER_PORT));
            Files.writeString(oidcTokenTempFile, k8sAuthHelper.getServiceAccountToken());
            try {
                Set<PosixFilePermission> permissions = Set.of(
                        PosixFilePermission.OWNER_READ,
                        PosixFilePermission.OWNER_WRITE,
                        PosixFilePermission.GROUP_READ,
                        PosixFilePermission.OTHERS_READ
                );
                Files.setPosixFilePermissions(oidcTokenTempFile, permissions);
            } catch (UnsupportedOperationException e) {
                oidcTokenTempFile.toFile().setReadable(true, true);
                oidcTokenTempFile.toFile().setWritable(true, true);
                oidcTokenTempFile.toFile().setReadable(true, false);
            }
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    protected static final GenericContainer<?> MAAS_CONTAINER = new GenericContainer<>(
            new ImageFromDockerfile()
                    .withFileFromPath(".", Paths.get("../maas"))
    )
            .withEnv(Map.of(
                            "DB_POSTGRESQL_ADDRESS", "%s:5432".formatted(POSTGRES_CONTAINER.getNetworkAliases().getFirst()),
                            "DB_POSTGRESQL_DATABASE", POSTGRES_CONTAINER.getDatabaseName(),
                            "DB_POSTGRESQL_USERNAME", POSTGRES_CONTAINER.getUsername(),
                            "DB_POSTGRESQL_PASSWORD", POSTGRES_CONTAINER.getPassword(),
                            "HEALTH_CHECK_INTERVAL", "1s"
                    )
            )
            .withNetwork(TEST_NETWORK)
            .withExposedPorts(8080)
            .withCopyFileToContainer(MountableFile.forHostPath(oidcTokenTempFile.toAbsolutePath()), "/var/run/secrets/kubernetes.io/serviceaccount/token")
            .dependsOn(POSTGRES_CONTAINER);

    static {
        POSTGRES_CONTAINER.start();

        KAFKA_CONTAINER_1.start();
        KAFKA_CONTAINER_2.start();

        RABBITMQ_CONTAINER_1.start();
        RABBITMQ_CONTAINER_2.start();

        OIDC_SERVER_CONTAINER.start();

        MAAS_CONTAINER.start();

        assertTrue(createManagersAccount());
        assertTrue(registerRabbitInstance(RABBITMQ_CONTAINER_1.getNetworkAliases().getFirst(), RABBITMQ_CONTAINER_1.getAdminUsername(), RABBITMQ_CONTAINER_1.getAdminPassword()));
        assertTrue(registerRabbitInstance(RABBITMQ_CONTAINER_2.getNetworkAliases().getFirst(), RABBITMQ_CONTAINER_2.getAdminUsername(), RABBITMQ_CONTAINER_2.getAdminPassword()));
        assertTrue(registerKafkaInstance(KAFKA_CONTAINER_1.getNetworkAliases().getFirst()));
        assertTrue(registerKafkaInstance(KAFKA_CONTAINER_2.getNetworkAliases().getFirst()));
    }

    @BeforeEach
    void logTestStart(TestInfo testInfo) {
        log.info(">>>>>> Starting test: {}", testInfo.getDisplayName());
    }

    @BeforeEach
    public void setUpAbstractMaasIT() {
        log.info(">>> Start setup test ==============================================================================================");
        initHelper();
        initAccount(agentAccount, TEST_NAMESPACE);
        log.info(">>> Test setup finished ---------------------------------------------------------------------------------------------");
    }

    @AfterEach
    public void tearDownAbstractMaasIT() throws IOException {
        log.info(">>> Start cleanup test ---------------------------------------------------------------------------------------------");
        deleteNamespace(TEST_NAMESPACE);
        deleteAccount(agentAccount, TEST_NAMESPACE);
        log.info(">>> Test cleanup finished  ==============================================================================================");
    }

    protected static String getMaasContainerAddress() {
        return "http://%s:%d/".formatted(MAAS_CONTAINER.getHost(), MAAS_CONTAINER.getFirstMappedPort());
    }

    private static boolean createManagersAccount() {
        Container.ExecResult execResult = null;
        try {
            execResult = MAAS_CONTAINER.execInContainer(
                    "curl",
                    "--insecure",
                    "-X", "POST",
                    "--retry", "5",
                    "http://localhost:8080/api/v1/auth/account/manager",
                    "-H", "Content-Type: application/json",
                    "-d", "{\"username\": \"manager\",\"password\": \"manager\"}"
            );
        } catch (Exception e) {
            Assertions.fail(e);
        }
        return execResult.getExitCode() == 0;
    }

    private static boolean registerKafkaInstance(String name) {
        Container.ExecResult execResult = null;
        try {
            execResult = MAAS_CONTAINER.execInContainer(
                    "curl",
                    "--insecure",
                    "-X", "POST",
                    "--retry", "5",
                    "http://localhost:8080/api/v1/kafka/instance",
                    "-H", "Authorization: Basic " + Base64.getEncoder().encodeToString("manager:manager".getBytes()),
                    "-H", "Content-Type: application/json",
                    "-d", """
                            {
                              "id": "%s",
                              "addresses": {
                                "PLAINTEXT": [
                                  "%s:9092"
                                ]
                              },
                              "maasProtocol": "PLAINTEXT"
                            }
                            """.formatted(name, name)
            );
        } catch (Exception e) {
            Assertions.fail(e);
        }
        return execResult.getExitCode() == 0;
    }

    private static boolean registerRabbitInstance(String name, String username, String password) {
        Container.ExecResult execResult = null;
        try {
            execResult = MAAS_CONTAINER.execInContainer(
                    "curl",
                    "--insecure",
                    "-X", "POST",
                    "--retry", "5",
                    "http://localhost:8080/api/v1/rabbit/instance",
                    "-H", "Authorization: Basic " + Base64.getEncoder().encodeToString("manager:manager".getBytes()),
                    "-H", "Content-Type: application/json",
                    "-d", """
                            {
                              "id" : "%s",
                              "apiUrl": "http://%s:15672/api/",
                              "amqpUrl": "amqp://%s:5672",
                              "user": "%s",
                              "password": "%s"
                            }
                            """.formatted(name, name, name, username, password)
            );
        } catch (Exception e) {
            Assertions.fail(e);
        }
        return execResult.getExitCode() == 0;
    }
}
