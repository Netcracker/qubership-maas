package com.netcracker.it.maas;

import com.google.gson.Gson;
import com.netcracker.it.maas.entity.Account;
import com.netcracker.it.maas.entity.HealthItem;
import com.netcracker.it.maas.entity.HealthResponse;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import okhttp3.Response;
import okhttp3.ResponseBody;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.AfterAll;
import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.List;
import java.util.UUID;

import static com.netcracker.it.maas.AbstractMaasWithInitsIT.MAAS_CONTAINER;
import static com.netcracker.it.maas.Utils.retry;
import static org.junit.jupiter.api.Assertions.*;

@Slf4j
class DisasterRecoveryIT extends AbstractMaasIT {

    @BeforeAll
    static void setUp() {
        MAAS_CONTAINER.stop();
        MAAS_CONTAINER.withEnv("EXECUTION_MODE", "disabled");
        MAAS_CONTAINER.start();
        resetHelper();
    }

    @AfterAll
    static void afterAll() {
        MAAS_CONTAINER.stop();
        MAAS_CONTAINER.withEnv("EXECUTION_MODE", "active");
        MAAS_CONTAINER.start();
        resetHelper();
    }

    @Test
    void checkHealthStatus() throws Exception {
        retry(20, () -> {
            log.info("Check maas health status");
            Request request = helper.createJsonRequest("health", "", null, MaasITHelper.GET);
            HealthResponse actualHealth = helper.doRequest(request, HealthResponse.class);
            log.info("Actual health response {}", actualHealth);
            HealthResponse expectHealth = new HealthResponse(
                    new HealthItem("UP"),
                    new HealthItem("UP"),
                    new HealthItem("UP"));
            assertEquals(expectHealth, actualHealth);
        });
    }

    @Test
    void checkGettingUser() throws IOException {
        Response response = getAccounts();
        assertEquals(HttpStatus.SC_OK, response.code());
        ResponseBody body = response.body();
        assertNotNull(body);
        log.info("Response: {}", response);
        Account[] accounts = new Gson().fromJson(body.string(), Account[].class);
        assertTrue(accounts.length > 0);
    }

    // any modifications should fail for maas in dr mode
    @Test
    void checkCreateAccount() throws IOException {
        Account testAccount = Account.builder()
                .username(UUID.randomUUID().toString())
                .password(UUID.randomUUID().toString())
                .namespace("user-agent-test-namespace")
                .roles(List.of("agent"))
                .build();
        Response response = createAccount(testAccount);
        assertEquals(HttpStatus.SC_METHOD_NOT_ALLOWED, response.code());
    }
}
