package org.qubership.it.maas;

import com.google.gson.Gson;
import org.qubership.it.maas.entity.Account;
import lombok.extern.slf4j.Slf4j;
import okhttp3.Response;
import okhttp3.ResponseBody;
import org.apache.http.HttpStatus;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.ArrayList;
import java.util.Map;

import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;

@Slf4j
class UsersAndSecurityIT extends AbstractMaasWithInitsIT {

    @BeforeEach
    public void setUp() {
        deletion();
    }
    @AfterEach
    public void clearAfter() {
        deletion();
    }


    public void deletion()  {
        log.info("(BEFORE) Clean account");
        deleteAccount(Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace")
                .roles( new ArrayList<String>() {{
                    add("agent");
                 }})
                .build());

        deleteAccount(Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace1")
                .roles( new ArrayList<String>() {{
                    add("agent");
                }})
                .build());

        deleteAccount(Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace2")
                .roles( new ArrayList<String>() {{
                    add("agent");
                }})
                .build());
    }

    @Test
    public void createUserAgentSeveralTimes() throws IOException {
        Account agentAccount = Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace")
                .roles( new ArrayList<String>() {{
                    add("agent");
                 }})
                .build();

        Response response = createAccount(agentAccount);
        assertEquals(response.code(), HttpStatus.SC_CREATED);
        ResponseBody body = response.body();
        assertNotNull(body);
        log.info("Response: {}", response);
        Account createdAccount = new Gson().fromJson(body.string(), Account.class);
        assertEquals(createdAccount.getUsername(), createdAccount.getUsername());
        assertEquals(createdAccount.getPassword(), createdAccount.getPassword());

        response = createAccount(agentAccount);
        assertEquals(response.code(), HttpStatus.SC_OK);
    }

    @Test
    public void createUserAgentWithDifNamespace() throws IOException {
        Account agentAccount1 = Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace1")
                .roles( new ArrayList<String>() {{
                    add("agent");
                }})
                .build();

        //unique constraint on username
        Account agentAccount2 = Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace2")
                .roles( new ArrayList<String>() {{
                    add("agent");
                }})
                .build();

        Response response;
        response = createAccount(agentAccount1);
        assertEquals(HttpStatus.SC_CREATED, response.code());
        response = createAccount(agentAccount2);
        assertEquals(HttpStatus.SC_INTERNAL_SERVER_ERROR, response.code());
    }

    @Test
    public void noAccessWithWrongNamespace() throws IOException {
        Account agentAccount = Account.builder()
                .username("user-agent-test-username")
                .password("user-agent-test-password")
                .namespace("user-agent-test-namespace")
                .roles( new ArrayList<String>() {{
                    add("agent");
                }})
                .build();

        Response response;
        response = createAccount(agentAccount);
        assertEquals(response.code(), HttpStatus.SC_CREATED);

        Map<String, Object> classifier = createSimpleClassifier("UsersAndSecurityConfigIT", "it-test", "user-agent-test-namespace");
        getKafkaTopicByClassifierWithAccount(HttpStatus.SC_NOT_FOUND, classifier, agentAccount, "user-agent-test-namespace");
        getKafkaTopicByClassifierWithAccount(HttpStatus.SC_FORBIDDEN, classifier, agentAccount, "wrong-namespace");
    }

}
