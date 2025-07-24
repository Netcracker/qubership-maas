package org.qubership.it.maas;


import com.google.gson.GsonBuilder;
import org.qubership.it.maas.entity.Account;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Request;
import okhttp3.RequestBody;
import okhttp3.Response;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.List;

import static org.qubership.it.maas.MaasITHelper.KAFKA_INSTANCES_PATH;
import static org.junit.jupiter.api.Assertions.*;

@Slf4j
class ServiceIT extends AbstractMaasWithInitsIT {

    @BeforeEach
    public void setUp() throws IOException {
        log.info("(BEFORE) Clean account");
        deletion();
        Account testAccount = Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace")
                .roles(List.of("agent"))
                .build();
        Response response = createAccount(testAccount);
        assertEquals(HttpStatus.SC_CREATED, response.code());
    }

    @AfterEach
    public void clearAfter() {
        log.info("(AFTER) Clean account");
        deletion();
    }


    public void deletion() {
        deleteAccount(Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace")
                .roles(List.of("agent"))
                .build());
        deleteAccount(Account.builder()
                .username("user-agent-test-roles-username")
                .password("user-agent-test-roles-password")
                .namespace("user-agent-test-roles-namespace")
                .roles(List.of("user-agent-test-role"))
                .build());
    }

    @Test
    public void checkWrongPasswordResponseCode() throws IOException {
        Request authRequest = helper.createJsonRequest(KAFKA_INSTANCES_PATH, getBasicAuthStr("user-agent-test-username", ""), null, MaasITHelper.GET);
        Response authResponse = okHttpClient.newCall(authRequest).execute();
        assertEquals(HttpStatus.SC_FORBIDDEN, authResponse.code());
    }

    @Test
    public void checkWrongRolesResponseCode() throws IOException {
        Account accountWrongRoles = Account.builder()
                .username("user-agent-test-roles-username")
                .password("user-agent-test-roles-password")
                .namespace("user-agent-test-roles-namespace")
                .roles(List.of("user-agent-test-role"))
                .build();
        Response response = createAccount(accountWrongRoles);
        assertTrue(response.code() == HttpStatus.SC_CREATED || response.code() == HttpStatus.SC_OK);
        Request authRequest = helper.createJsonRequest(KAFKA_INSTANCES_PATH, getMaasBasicAuth(accountWrongRoles), null, MaasITHelper.GET);
        Response authResponse = okHttpClient.newCall(authRequest).execute();
        assertEquals(HttpStatus.SC_FORBIDDEN, authResponse.code());
    }

    @Test
    public void checkNoNamespaceResponseCode() {
        String initAccountRequest = new GsonBuilder().create().toJson(
                Account.builder()
                        .username("test")
                        .password("test")
                        .namespace("")
                        .roles(List.of("agent"))
                        .build());
        Request noNamespaceRequest = new Request.Builder()
                .url(getMaasContainerAddress() + "api/v1/auth/account/client")
                .addHeader("Authorization", "Basic " + MAAS_MANAGER_ACCOUNT_AUTH_STR)
                .addHeader("X-Origin-Namespace", "")
                .post(RequestBody.create(initAccountRequest.getBytes()))
                .build();
        try (Response noNamespaceResponse = okHttpClient.newCall(noNamespaceRequest).execute()) {
            assertEquals(HttpStatus.SC_BAD_REQUEST, noNamespaceResponse.code());
        } catch (IOException e) {
            log.error("Failed to check response code", e);
            fail("Failed to check response code", e);
        }
    }
}
