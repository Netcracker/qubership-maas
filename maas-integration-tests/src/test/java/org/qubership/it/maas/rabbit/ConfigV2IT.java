package org.qubership.it.maas.rabbit;

import org.qubership.it.maas.entity.SearchCriteria;
import org.qubership.it.maas.entity.ConfigV2Resp;
import org.qubership.it.maas.entity.rabbit.RabbitInstance;
import org.qubership.it.maas.entity.rabbit.VhostConfigResponse;
import org.qubership.it.maas.entity.rabbit.VirtualHostResponse;
import lombok.extern.slf4j.Slf4j;
import org.apache.http.HttpStatus;
import org.hamcrest.CoreMatchers;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.util.Arrays;
import java.util.Map;

import static org.qubership.it.maas.MaasITHelper.TEST_NAMESPACE;
import static org.apache.http.HttpStatus.*;
import static org.hamcrest.MatcherAssert.assertThat;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.junit.jupiter.api.Assumptions.assumeTrue;

@Slf4j
class ConfigV2IT extends RabbitTest {

    @Test
    void rabbitInstanceDesignatorNameInConfig() throws Exception {
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assumeTrue(rabbitInstances.length >= 2, "rabbitInstanceDesignator test requires at least two instances. Skip test");

        RabbitInstance firstInstance = rabbitInstances[0];
        RabbitInstance secondInstance = rabbitInstances[1];

        String cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec: \n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  shared: |\n" +
                        "    apiVersion: nc.maas.rabbit/v2\n" +
                        "    kind: instance-designator\n" +
                        "    spec:\n" +
                        "        namespace: " + TEST_NAMESPACE + "\n" +
                        "        defaultInstance: " + firstInstance.getId() + "\n" +
                        "        selectors:\n" +
                        "        - classifierMatch:\n" +
                        "            name: orders\n" +
                        "          instance: " + firstInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "  services: \n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "          apiVersion: nc.maas.rabbit/v1\n" +
                        "          kind: vhost\n" +
                        "          spec:\n" +
                        "             classifier: { name: orders, namespace: " + TEST_NAMESPACE + "}\n" +
                        "             "
        ;

        ConfigV2Resp reply;
        reply = applyConfigV2(SC_OK, cfg);

        assertEquals("ok", reply.getStatus());

        Map<String, Object> classifierRabbit = createSimpleClassifier("orders", "", TEST_NAMESPACE);
        VhostConfigResponse vhostConfig = getRabbitVhostByClassifier(SC_OK, classifierRabbit);
        assertThat(vhostConfig.getVhost().getCnn(), CoreMatchers.containsString(String.format("%s.%s", TEST_NAMESPACE, "orders")));
        assertThat(vhostConfig.getVhost().getCnn(), CoreMatchers.containsString(firstInstance.getAmqpUrl()));

        Map<String, Object> classifierRabbitTenant = createSimpleClassifier("configV2VhostIT", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        VirtualHostResponse virtualHost = createVirtualHost(SC_CREATED, classifierRabbitTenant);
        assertThat(virtualHost.getCnn(), CoreMatchers.containsString(secondInstance.getAmqpUrl()));


        deleteRabbitInstanceDesignator(SC_OK);
    }

    @Test
    void rabbitInstanceDesignatorUpdate() throws Exception {
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assumeTrue(rabbitInstances.length >= 2, "rabbitInstanceDesignator test requires at least two instances. Skip test");

        RabbitInstance firstInstance = rabbitInstances[0];
        RabbitInstance secondInstance = rabbitInstances[1];

        String cfg;
        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec: \n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  shared: |\n" +
                        "    apiVersion: nc.maas.rabbit/v2\n" +
                        "    kind: instance-designator\n" +
                        "    spec:\n" +
                        "        namespace: " + TEST_NAMESPACE + "\n" +
                        "        defaultInstance: " + firstInstance.getId() + "\n" +
                        "        selectors:\n" +
                        "        - classifierMatch:\n" +
                        "            name: orders\n" +
                        "          instance: " + firstInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "  services: \n" +
                        "    - serviceName: order-processor\n" +
                        "      config: |+\n" +
                        "          apiVersion: nc.maas.rabbit/v1\n" +
                        "          kind: vhost\n" +
                        "          spec:\n" +
                        "             classifier: { name: configV2VhostIT, namespace: " + TEST_NAMESPACE + ", tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645 }\n" +
                        "             "
        ;

        ConfigV2Resp reply;
        reply = applyConfigV2(SC_OK, cfg);

        assertEquals("ok", reply.getStatus());

        //check Rabbit
        Map<String, Object> classifierRabbit = createSimpleClassifier("configV2VhostIT", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        VhostConfigResponse vhostConfig = getRabbitVhostByClassifier(HttpStatus.SC_OK, classifierRabbit);
        assertThat(vhostConfig.getVhost().getCnn(), CoreMatchers.containsString(String.format("%s.%s.%s", TEST_NAMESPACE, "82133ba8-4bf9-4659-9e02-62e608bab645", "configV2VhostIT")));
        assertThat(vhostConfig.getVhost().getCnn(), CoreMatchers.containsString(secondInstance.getAmqpUrl()));

        Map<String, Object> classifierRabbitName = createSimpleClassifier("orders", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        VirtualHostResponse virtualHost = createVirtualHost(SC_CREATED, classifierRabbitName);
        assertThat(virtualHost.getCnn(), CoreMatchers.containsString(firstInstance.getAmqpUrl()));

        Map<String, Object> classifierRabbitDefault = createSimpleClassifier("default", "", TEST_NAMESPACE);
        VirtualHostResponse virtualHostDefault = createVirtualHost(SC_CREATED, classifierRabbitDefault);
        assertThat(virtualHostDefault.getCnn(), CoreMatchers.containsString(firstInstance.getAmqpUrl()));

        //update

        cfg =
                "apiVersion: nc.maas.config/v2\n" +
                        "kind: config\n" +
                        "spec: \n" +
                        "  version: v1\n" +
                        "  namespace: " + TEST_NAMESPACE + "\n" +
                        "  shared: |\n" +
                        "    apiVersion: nc.maas.rabbit/v2\n" +
                        "    kind: instance-designator\n" +
                        "    spec:\n" +
                        "        namespace: " + TEST_NAMESPACE + "\n" +
                        "        defaultInstance: " + secondInstance.getId() + "\n" +
                        "        selectors:\n" +
                        "        - classifierMatch:\n" +
                        "            name: orders-2\n" +
                        "          instance: " + secondInstance.getId() + "\n" +
                        "        - classifierMatch:\n" +
                        "            tenantId: 82133ba8-4bf9-4659-9e02-62e608bab645\n" +
                        "          instance: " + firstInstance.getId() + "\n"
        ;

        reply = applyConfigV2(SC_OK, cfg);
        assertEquals("ok", reply.getStatus());

        //check Rabbit, vhost should stay the same
        classifierRabbit = createSimpleClassifier("configV2VhostIT", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        vhostConfig = getRabbitVhostByClassifier(HttpStatus.SC_OK, classifierRabbit);
        assertThat(vhostConfig.getVhost().getCnn(), CoreMatchers.containsString(String.format("%s.%s.%s", TEST_NAMESPACE, "82133ba8-4bf9-4659-9e02-62e608bab645", "configV2VhostIT")));
        assertThat(vhostConfig.getVhost().getCnn(), CoreMatchers.containsString(secondInstance.getAmqpUrl()));

        classifierRabbitDefault = createSimpleClassifier("default-2", "", TEST_NAMESPACE);
        virtualHostDefault = createVirtualHost(SC_CREATED, classifierRabbitDefault);
        assertThat(virtualHostDefault.getCnn(), CoreMatchers.containsString(secondInstance.getAmqpUrl()));

        classifierRabbitName = createSimpleClassifier("orders-2", "82133ba8-4bf9-4659-9e02-62e608bab645", TEST_NAMESPACE);
        virtualHost = createVirtualHost(SC_CREATED, classifierRabbitName);
        assertThat(virtualHost.getCnn(), CoreMatchers.containsString(secondInstance.getAmqpUrl()));

        deleteRabbitInstanceDesignator(new int[]{HttpStatus.SC_OK});
    }

    @Test
    void deleteDefaultInstanceRabbit() throws Exception {
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assumeTrue(rabbitInstances.length >= 2, "deleteDefaultInstanceRabbit test requires at least two instances. Skip test");

        RabbitInstance defaultInstance, secondInstance;
        if (rabbitInstances[0].getIsDefault()) {
            defaultInstance = rabbitInstances[0];
            secondInstance = rabbitInstances[1];
        } else {
            defaultInstance = rabbitInstances[1];
            secondInstance = rabbitInstances[0];
        }

        deleteRabbitInstance(defaultInstance, SC_BAD_REQUEST);
        deleteRabbitInstance(secondInstance, SC_OK);
        deleteRabbitInstance(defaultInstance, SC_OK);
    }

    @Test
    void deleteRabbitInstanceById() throws IOException {
        log.info("Request instances list");
        RabbitInstance[] rabbitInstances = getRabbitInstances();
        assumeTrue(rabbitInstances.length >= 2, "deleteRabbitInstanceById test requires at least two instances. Skip test");

        RabbitInstance instanceToDelete = Arrays.stream(rabbitInstances).filter(i -> !i.getIsDefault()).findAny().get();
        log.info("Delete instance {}", instanceToDelete);

        log.info("Create vhost");
        Map<String, Object> classifier = createSimpleClassifier("test-name");
        createVirtualHost(SC_CREATED, classifier, instanceToDelete.getId());

        RabbitInstance onlyIdInstance = new RabbitInstance();
        onlyIdInstance.setId(instanceToDelete.getId());

        log.info("Test deletion denial due of existing vhost in instance");
        deleteRabbitInstance(onlyIdInstance, SC_BAD_REQUEST);

        log.info("Delete vhost in instance");
        deleteVhost(SearchCriteria.builder().classifier(classifier).build(), SC_NO_CONTENT);

        log.info("Delete cleaned up instance from registrations");
        deleteRabbitInstance(onlyIdInstance, SC_OK);
    }
}
