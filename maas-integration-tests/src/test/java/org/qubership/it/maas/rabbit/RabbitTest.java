package org.qubership.it.maas.rabbit;

import org.qubership.it.maas.AbstractMaasWithInitsIT;
import org.qubership.it.maas.entity.SearchCriteria;
import org.qubership.it.maas.entity.rabbit.RabbitInstance;
import lombok.extern.slf4j.Slf4j;
import org.junit.jupiter.api.AfterEach;
import org.junit.jupiter.api.BeforeEach;

import java.io.IOException;
import java.util.Arrays;
import java.util.Comparator;

import static org.apache.http.HttpStatus.*;

@Slf4j
public abstract class RabbitTest extends AbstractMaasWithInitsIT {
    private RabbitInstance[] rabbitInstances;

    @BeforeEach
    public void setUpRabbitTest() throws IOException {
        log.info(">>> Start setup Rabbit test");
        rabbitInstances = getRabbitInstances();
        log.info("Cleanup missed vhosts from failed tests");
        for (RabbitInstance instance : rabbitInstances) {
            deleteVhost(SearchCriteria.builder().instance(instance.getId()).build(), SC_NO_CONTENT, SC_NOT_FOUND);
        }
        log.info(">>> Test setup Rabbit finished");
    }

    @AfterEach
    public void tearDownRabbitTest() throws IOException {
        log.info(">>> Start cleanup Rabbit test");
        RabbitInstance[] instances = getRabbitInstances();
        Arrays.stream(instances).sorted(Comparator.comparing(RabbitInstance::getIsDefault)).forEach(instance -> {
            try {
                deleteVhost(SearchCriteria.builder().instance(instance.getId()).build(), SC_NO_CONTENT, SC_NOT_FOUND);
                deleteRabbitInstance(instance, SC_OK);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        });
        for (RabbitInstance rabbitInstance : this.rabbitInstances) {
            createRabbitInstance(rabbitInstance);
        }
        log.info(">>> Test cleanup Rabbit finished");
    }

}
