package com.netcracker.it.maas;

import lombok.extern.slf4j.Slf4j;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.TestInfo;

import java.io.IOException;

import static com.netcracker.it.maas.MaasITHelper.TEST_NAMESPACE;

@Slf4j
public abstract class AbstractMaasWithInitsIT extends AbstractMaasIT {

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
}
