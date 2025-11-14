[[_TOC_]]

# Rest API
## RabbitMQ API
### Register Rabbit instance
First instance registered in MaaS becomes `default`. This instance will be used as `default` for all items with empty instance field. Only one instance can be marked as `default` at particular moment of time. To switch `default` to other instance, just update its declaration with `default` field set to true.
Instance field and its value can be explicitly specified in item declaration. But, explicit declaration of instance id in item configuration is discouraged due to insufficient portability. It's better to move 'item-to-instance' distribution rules to instance-designators configuration. [Instance designators API](#instance-designators-api)

* **URI:**  `{maas_host}/api/v2/rabbit/instance`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with connection properties for Rabbit instance: 
    ```json
    {
      "id" : "<[optional] id for rabbit instance. For example namespace name>",
      "apiUrl": "<http url to Rabbit instance for manager to create and configure users and vhosts>",
      "amqpUrl": "<amqp url to Rabbit instance to interact via AMQP protocol>",
      "user": "<username to access Rabbit>",
      "password": "<password to access Rabbit>",
      "default": "<[optional] bool flag that indicates whether this RabbitMQ instnace should be used by default>"
    }
    ```
    apiUrl format: `http://<your-host>:<http-port>/api`, e.g. `http://rabbit-service.cpq-rabbit:15672/api"` 
    amqpUrl format: `amqp://<your-host>:<amqp-port>`, e.g. `amqp://rabbit-service.cpq-rabbit:5672`  
    Field `"default": true` sets default RabbitMQ instance to this one.     
    Default instance will be used for virtual hosts registration if another RabbitMQ instance is not specified in virtual host registration request.   
* **Success Response:**  
    `200` - Rabbit instance was registered successfully.  
* **Error Response:**   
    *Http code*: `409`
    *Response body:* 
    ```json
    {
      "error": "can't register the instance. An instance with specified id/address already exists"
    }
    ```
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X POST \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/instance \
      -H 'Content-Type: application/json' \
      -d '{
      "id" : "maas-rabbitmq-code-dev",
      "ApiUrl": "http://rabbitmq.maas-rabbitmq-code-dev:15672",
      "User": "guest",
      "Password": "guest"
    }'
    ```
    Response:
    ```json
    200
    ```

### Get Rabbit instances
This API allows to get Rabbit instances in Maas.  
* **URI:**  `{maas_host}/api/v2/rabbit/instances`  
* **Method:** `GET`
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    No body
* **Success Response:**  
    `200` - Rabbit instances were requested successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  
    
    Request:
    ```bash
    curl --location --request GET 'http://localhost:8080/api/v2/rabbit/instances' \
    --header 'Authorization: Basic bWFuYWdlcjoyNDAwMmVhZGJj' 
    ```
    Response: `200`
    ```json
    [
        {
            "Id": "fd02de58-da04-4842-a258-37ebe4e5ac1e",
            "ApiUrl": "http://127.0.0.1:15672/api",
            "AmqpUrl": "amqp://127.0.0.1:5672",
            "User": "guest",
            "Password": "guest",
            "Default": true
        }
    ]
    ```

### Update Rabbit instance registration
This API allows to update connection properties for RabbitMQ instance in Maas.  
* **URI:**  `{maas_host}/api/v2/rabbit/instance`  
* **Method:** `PUT`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with connection properties for Rabbit instance: 
    ```json
    {
      "id" : "<id for rabbit instance. For example namespace name>",
      "apiUrl": "<url to Rabbit instance>",
      "user": "<username to access Rabbit>",
      "password": "<password to access Rabbit>",
      "default": "<[optional] bool flag that indicates whether this RabbitMQ instnace should be used by default>"
    }
    ```
    Field `"default": true` sets default RabbitMQ instance to this one. 
    Default instance will be used for virtual hosts registration if another RabbitMQ instance is not specified in virtual host registration request. 
* **Success Response:**  
    `200` - Rabbit instance was updated successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X PUT \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/instance \
      -H 'Content-Type: application/json' \
      -d '{
      "id" : "maas-rabbitmq-code-dev",
      "ApiUrl": "http://rabbitmq.maas-rabbitmq-code-dev:15672",
      "User": "guest",
      "Password": "guest"
    }'
    ```
    Response:
    ```json
    200
    ```

### Remove Rabbit instance registration
This API allows to remove RabbitMQ instance registration from Maas.  
* **URI:**  `{maas_host}/api/v2/rabbit/instance`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with connection properties for Rabbit instance: 
    ```json
    {
      "id" : "<id for rabbit instance. For example namespace name>"
    }
    ```
* **Success Response:**  
    `200` - Rabbit instance was removed successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X DELETE \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/instance \
      -H 'Content-Type: application/json' \
      -d '{
      "id" : "maas-rabbitmq-code-dev"
    }'
    ```
    Response:
    ```json
    200
    ```

### Create Rabbit virtual host

This API allows to create and get a Rabbit MQ virtual host connection.   
* **URI:**  `{maas_host}/api/v2/rabbit/vhost?extended=true`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Openshift namespace (project name) where a virtual host will be used;  
* **Parameters:**
    `instance` - rabbitmq instance for virtual host. If not specified default instance will be used  
* **Query params:**
  `extended` - values can be either `true` or `false`.
    * `true` - will return extended response with additional parameters like `apiUrl`

* **Authorization:**
    Basic type
* **Request body:**
    ```json
    {
      "classifier": {
        "name": "<your any custom name>",
        "tenantId": "[optional]<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }, 
      "instance": "<[optional] rabbitmq instance for virtual host. if not specified default instance will be used>"
    }
    ```
    The request body contains a composite identification key that is called `virtual host classifier`. You need to know it for other APIs. More about classifier and its mandatory and optional fields you can find in [README.md](../README.md#classifier)
* **Success Response:**  
    `201` - If a virtual host was created on your request.  
    or  
    `200` - If a virtual host was created early.  
     Response body:  
    ```json
    {
      "cnn": "ampq://<rabbit_host>:<rabbit_port>/<virtual_host_name>",
      "apiUrl": "http://localhost:15672/api",
      "username": "<virtual_host_username>",
      "password": "[plain|km]:<password or password id>"
    }
    ```
  
    Classifier is used to generate vhost name by next contract: 
    In case if you have "tenantId" field:
    ```
    "maas.<namespace>.<tenantId>.<name>" 
    ```
    
    In case if you don't have "tenantId" field:
    ```
    "maas.<namespace>.<name>" 
    ```
  
* **Error Response:**
  
    *Http code*: `500`  
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X POST \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/vhost \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: cloudbss311-platform-core-support-dev3' \
      -d '{
        "namespace": "cloudbss311-platform-core-support-dev3",
        "name": "tenant-manager",
        "tenantId": "123456"
    }'
    ```
    Response:
    ```json
    201
    {
      "cnn": "ampq://rabbitmq.maas-rabbitmq-code-dev:5672/maas.cloudbss311-platform-core-support-dev3.123456.tenant-manager",
      "username": "d7c7bb4a87af433fb8a22c96a0720097",
      "password": "km:6a2d8ed5-268f-4409-8a77-f2e52ff518fa"
    }
    ```
  
### Get Rabbit virtual host with config

This API allows getting a Rabbit MQ virtual host connection with additional config info about rabbit entities.   
* **URI:**  `{maas_host}/api/v2/rabbit/vhost/get-by-classifier`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Kubernetes namespace (project name) where a virtual host will be used;
* **Query params:**
  `extended` - values can be either `true` or `false`.
    * `true` - will return extended response with additional parameters like `apiUrl`
* **Authorization:**
    Basic type
* **Request body:**
    ```json
      {
        "name": "<your any custom name>",
        "tenantId": "[optional]<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }
    ```
    The request body contains a composite identification key that is called `virtual host classifier`.
* **Success Response:**  
    `200` - If a virtual host was created early.  
     Response body:  
    ```json
    {
      "vhost" : {
          "cnn": "ampq://<rabbit_host>:<rabbit_port>/<virtual_host_name>",
          "apiUrl": "http://localhost:15672/api",
          "username": "<virtual_host_username>",
          "password": "[plain|km]:<password or password id>"
          },
      "entities": {
          "exchanges": [
                 {..}, {..}
          ],           
          "queues": [
                 {..}, {..}
          ],           
          "bindings": [
                 {..}, {..}
          ]
       } 
    }
    ```
* **Error Response:**
  
    *Http code*: `500`  
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
  * **Sample call**  

      Request:
      ```bash
        curl --location --request POST 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/vhost/get-by-classifier' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json' \
        --data-raw '{
              "namespace": "namespace",
              "name": "test12"
        }
        '
      ```
      Response: 200
      ```json
    {
        "vhost": {
            "cnn": "amqp://127.0.0.1:5672/maas.namespace.test12",
            "username": "9757343a78a04057a07ee4215f1b1355",
            "password": "plain:504a449065924332b062c4a84e830cbe"
        },
        "entities": {
            "exchanges": [
                {
                    "arguments": {
                        "seconf-arg": "second",
                        "some-argument": "smth"
                    },
                    "auto_delete": false,
                    "durable": true,
                    "internal": false,
                    "name": "test-exchange-2",
                    "type": "topic",
                    "user_who_performed_action": "9757343a78a04057a07ee4215f1b1355",
                    "vhost": "namespace.test12"
                }
            ],
            "queues": [
                {
                    "arguments": {
                        "x-dead-letter-exchange": "cpq-quote-modify-dlx1"
                    },
                    "auto_delete": false,
                    "backing_queue_status": {
                        "avg_ack_egress_rate": 0,
                        "avg_ack_ingress_rate": 0,
                        "avg_egress_rate": 0,
                        "avg_ingress_rate": 0,
                        "delta": [
                            "delta",
                            "undefined",
                            0,
                            0,
                            "undefined"
                        ],
                        "len": 0,
                        "mode": "default",
                        "next_seq_id": 0,
                        "q1": 0,
                        "q2": 0,
                        "q3": 0,
                        "q4": 0,
                        "target_ram_count": "infinity"
                    },
                    "consumer_utilisation": null,
                    "consumers": 0,
                    "durable": false,
                    "effective_policy_definition": {},
                    "exclusive": false,
                    "exclusive_consumer_tag": null,
                    "garbage_collection": {
                        "fullsweep_after": 65535,
                        "max_heap_size": 0,
                        "min_bin_vheap_size": 46422,
                        "min_heap_size": 233,
                        "minor_gcs": 330
                    },
                    "head_message_timestamp": null,
                    "memory": 42664,
                    "message_bytes": 0,
                    "message_bytes_paged_out": 0,
                    "message_bytes_persistent": 0,
                    "message_bytes_ram": 0,
                    "message_bytes_ready": 0,
                    "message_bytes_unacknowledged": 0,
                    "messages": 0,
                    "messages_details": {
                        "rate": 0
                    },
                    "messages_paged_out": 0,
                    "messages_persistent": 0,
                    "messages_ram": 0,
                    "messages_ready": 0,
                    "messages_ready_details": {
                        "rate": 0
                    },
                    "messages_ready_ram": 0,
                    "messages_unacknowledged": 0,
                    "messages_unacknowledged_details": {
                        "rate": 0
                    },
                    "messages_unacknowledged_ram": 0,
                    "name": "test-queue-2",
                    "node": "rabbit@localhost",
                    "operator_policy": null,
                    "policy": null,
                    "recoverable_slaves": null,
                    "reductions": 2814757,
                    "reductions_details": {
                        "rate": 214.4
                    },
                    "single_active_consumer_tag": null,
                    "state": "running",
                    "type": "classic",
                    "vhost": "namespace.test12"
                }
            ],
            "bindings": [
                {
                    "arguments": {},
                    "destination": "test-queue-2",
                    "destination_type": "queue",
                    "properties_key": "test-queue-2",
                    "routing_key": "test-queue-2",
                    "source": "",
                    "vhost": "namespace.test12"
                },
            ]
        }
    }
      ```

### Search Rabbit vhosts
This API allows to search Rabbit vhosts.
* **URI:**  `{maas_host}/api/v2/rabbit/vhost/search`
* **Method:** `POST`
* **Headers:**  
  `Content-Type: application/json`  
  `X-Origin-Namespace` - Namespace (project name) where a vhost will be used;
* **Authorization:**
  Basic type
* **Request body:**
  Body can contain any combination of the following fields (all of them are optional).

  Search will find results by exact match with the provided field values (so e.g. you cannot search topics by classifier part).
    ```json
    {
        "classifier": {
            "name": "<your any custom name>",
            "tenantId": "<external tenant identifier>",
            ...
        }, 
        "vhost": "<exact vhost name in rabbit>",
        "namespace": "<namespace to which vhost belong>"
        "instance": "<id of rabbit instance where vhost was created>"
    }
    ```
  * **Success Response:**

    *HTTP code:* `200`

    *Response body:*
      ```json
      [
          {
           "Vhost": ...,
           "User": ...       ,
            "Namespace": ... ,
           "InstanceId": ... ,
           "Classifier": ... ,
          },
          ...
      ]
      ```

* **Error Response:**

  *HTTP code*: `500` in case of internal server errors; `400` in case of invalid request data

  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**

  Request:
    ```bash
    curl -X POST \
    http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/vhost/search \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: local' \
      -d '{
        "namespace": "cloudbss311-platform-core-support-dev3"
    }'
    ```

### Delete virtual host
This API allows to delete virtual hosts by search parameters.  
* **URI:**  `{maas_host}/api/v2/rabbit/vhost`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Openshift namespace (project name) for which this virtual host was created;  
* **Authorization:**
    Basic type
* **Request body:**
    You should pass body with parameters (all of them are optional), so vhosts that matched these parameters will be removed. In case of classifier you are able to provide only a part of it.
    ```json
    {
      "vhost": "<virtual host id>",
      "user": "<virtual host owner>",   
      "namespace": "<core namespace where maas-agent installed>",
      "instance": "<rabbitmq instance id>",
      "classifier": "<partly or fully described virtual host classifier>"
    }
    ```
* **Success Response:**  
    `204` - Virtual hosts were deleted successfully.  
* **Error Response:**   

    *Http code*: `404`  - In case when there was no virtual hosts that matched search parameters   
    *Response body:* 
    ```json
    {
      "error": "No virtual hosts matched search parameters were found"
    }
    ```
    ---
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X DELETE \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/rabbit/vhost \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: cloudbss311-platform-core-support-dev3' \
      -d '{
      "namespace": "cloudbss311-platform-core-support-dev3",
      "classifier": {"tenantId": "123456"}
    }'
    ```
    Response:
    ```json
    204
    ```

### Validate Rabbit Configs

This API allows rabbit configs bg validation. It helps getting lazy bindings - versioned bindings in rabbit v2 configuration, which yet have not exchange source and expected to appear during app update or new app deployment with exchange declaration    

* **URI:**  `{maas_host}/api/v2/rabbit/validations`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Openshift/kubernetes namespace (project name) of your application  
* **Authorization:**
    Basic type with credentials with 'agent' role
* **Success Response:**  
    `200` 
     Response body:  
    ```json
    {
        "bindings": [
        {
            "vhost": <vhost name>,
            "entity": {
                <entity declaration by user>
            },
            "exchangeVersion": <exchange version>,
            "queueVersion": <queue version>
        }
      ] 
    }   

    ```
* **Error Response:**
    *Http code*: `500`  
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl --location --request GET 'http://localhost:8080/api/v2/rabbit/validations' \
    --header 'X-Origin-Namespace: core-dev' \
    --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' 
     ```
    Response: 200
    ```json
    {
        "bindings": [
            {
                "vhost": "maas.core-dev.test",
                "entity": {
                    "destination": "q1",
                    "routing_key": "key",
                    "source": "e1"
                },
                "exchangeVersion": "v1",
                "queueVersion": "v1"
            }
        ]
}
    ```

### Rabbit namespace recovery

This API allows recovering of all RabbitMQ entities, stored in MaaS registry. It includes both versioned and non-versioned entites.   

* **URI:**  `{maas_host}/api/v2/rabbit/recovery/{namespace}`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Openshift/kubernetes namespace (project name) of your application  
* **Authorization:**
    Basic type with credentials with 'agent' role
* **Success Response:**  
    `200`
* 
* **Error Response:**
    *Http code*: `500`  
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl --location --request POST 'http://localhost:8080/api/v2/rabbit/recovery/core-dev' \
    --header 'X-Origin-Namespace: core-dev' \
    --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' 
     ```
    Response: 200

### Rotate vhosts passwords

This API allows password rotation to all vhosts by search form. The response shows all affected vhosts. New password you can get via GET vhost operation.

* **URI:**  `{maas_host}/api/v2/rabbit/password-rotation`
* **Method:** `POST`
* **Headers:**  
  `Content-Type: application/json`  
  `X-Origin-Namespace` - Openshift/kubernetes namespace (project name) of your application
* **Authorization:**
  Basic type with credentials
* **Request body:**
  You should pass body with parameters (all of them are optional), so vhosts that matched these parameters will be affected.
    ```json
    {
      "vhost": "<virtual host id>",
      "user": "<virtual host owner>",   
      "namespace": "<core namespace where maas-agent installed>",
      "instance": "<rabbitmq instance id>",
      "classifier": "<virtual host classifier>"
    }
    ```
* **Success Response:**  
  `200`
  Response body:
    ```json
    [
        {
            "vhost": <name>,
            "user": <user>,
            "password": "***",
            "namespace": <namespace>,
            "instanceId": <id>,
            "classifier": <classifier>
        },
    
       ...
    ]
    ```

* **Error Response:**
  *Http code*: `500`  
  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**

  Request:
    ```bash
    curl --location 'localhost:8080/api/v2/rabbit/password-rotation' \
    --header 'X-Origin-Namespace: my-namespace' \
    --header 'Content-Type: text/plain' \
    --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
    --data '{
    "namespace": "my-namespace"
    }' 
     ```
  Response: 200
    ```json
     [
        {
            "vhost": "maas.namespace.tenant.name2",
            "user": "maas.namespace.a846db65f3",
            "password": "***",
            "namespace": "namespace",
            "instanceId": "344d94cf-a5b3-400f-912a-fcb7ce9429b7",
            "classifier": "{\"name\":\"name\",\"namespace\":\"namespace\",\"tenantId\":\"tenant\"}"
        },
        {
            "vhost": "maas.namespace.tenant.name1",
            "user": "maas.namespace.6e495493f3",
            "password": "***",
            "namespace": "namespace",
            "instanceId": "344d94cf-a5b3-400f-912a-fcb7ce9429b7",
            "classifier": "{\"name\":\"name1\",\"namespace\":\"namespace\",\"tenantId\":\"tenant\"}"
        }
    ]
    ```


## Kafka API
### Register Kafka instance
This API allows to register Kafka instance in Maas.  
* **URI:**  `{maas_host}/api/v2/kafka/instance`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with connection properties for Kafka instance: 
    ```json
    {
      "id" : "<[optional] id for kafka instance, e.g. 'my-namespace-kafka'>",
      "addresses": "<map protocol: addresses list>", 
      "maasProtocol": "<[optional] procol used by MaaS for communcating with this kafka; default - PLAINTEXT",
      "default": "<[optional] bool flag that indicates whether this kafka instnace should be used by default>",
      "caCert": "<[optional] base64 CA certificate of this Kafka instance>",
      "credentials": { // [required] Admin and client credentials for this kafka instance
        "admin": [ "<[required] List of Kafka Auth DTOs containing admin credentials>" ],
        "client": [ "<[required] List of Kafka Auth DTOs containing client credentials>" ]
    }
    ```
    Field addresses contains map where the key is protocol and value is a list of broker addresses. Allowed protocol values: `PLAINTEXT`, `SSL`, `SASL_PLAINTEXT`, `SASL_SSL`. 

    Field `"default": true` sets default Kafka instance to this one.
    Default instance will be used for new topics registration if another kafka instance is not specified in topic registration request.

    See [Kafka Auth DTO](#kafka-auth-dto) description for more details on Kafka client credentials format. 
* **Success Response:**  
    `200` - Kafka instance was registered successfully.  
* **Error Response:**   
    *Http code*: `409`
    *Response body:* 
    ```json
    {
      "error": "can't register the instance. An instance with specified id/address already exists"
    }
    ```
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X POST \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/instance \
      -H 'Content-Type: application/json' \
      -d '{
        "id": "localkafka",
        "addresses": { "PLAINTEXT": ["localk8s:9092"] }, 
        "default": true,
        "maasProtocol": "PLAINTEXT",
        "caCert": "MIIDPjCCAiYCCQCNmVmmEXs5XjANBgkqhkiG9w0BAQsFADBVMQswCQYDVQQGEwJYWDEVMBMGA1UEBwwMRGVmYXVsdCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMREwDwYDVQQDDAhsb2NhbGs4czAeFw0yMDA4MTgxMjUzMDFaFw0yMDExMjUxMjUzMDFaMG0xEDAOBgNVBAYTB1Vua25vd24xEDAOBgNVBAgTB1Vua25vd24xEDAOBgNVBAcTB1Vua25vd24xEDAOBgNVBAoTB1Vua25vd24xEDAOBgNVBAsTB1Vua25vd24xETAPBgNVBAMTCGxvY2FsazhzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjcTaRTo7E7UI9jO9gtRAi5au57elqRX2YMj/OOGcwQdzP6JfMFZsKFNUoIoF8bJ51JXhbDxVgB+GHvEMmQ0jqGnMjSTsdxEQUCRTnINMAIAYLBKm5FGi5pJodZRzhNKoWhloRO9/2p2AYB+T39MxXFch3fwMdghVKbSqOCo0nsqCZwyB5CcZgLi69qifZPQAIFPUPDHG5Z6oGUjE/p+45RnOcAdCOgO0QllxO+fioCMPizRqIiim88UuZU7EjhaIwSTjOIohcPQStNU6vAp0ZGIgr8BhAZHiL8JRDto37ayo7ltDYtLg4Ojo3e9ue8Dwo5PSs+N6Od8Z//Xq6V8zwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCDsV/jDojj4t977V5BMSTDeELEvNX8VMMtqAGpB1jtVNVXRLfG2SAcv6ZdOUbyuBagF8D0dsV5VvcqPw8YHNpHMKCTSdcida28rV2C31M+XRvvh90eoPtXfE60wo4Ky4UbKiJERiBIXMFLrg8PZ51PukT4fD0DioNpIxzRFb4VkypYv4srADX5shSvJN9Zxdj2EywR+S1k2F4TIDdOnWY9xGMftJz1fc58dMFMwGi7Evr+pR/w7yWDcvRgdgAYUpGaehmnYhzuw4XKWmVX1D6aVFRhonHaxN0rUemPZYSqjHp+lWOiUsYqatxB5MGQqj1/QV3XTAWglFbk1BB2stQg",
        "credentials": {      
            "admin": [{
                "type": "SCRAM",
                "username": "admin",
                "password": "plain:admin-secret"
            }],   
            "client": [{
                "type": "sslCert+SCRAM",
                "clientKey": "MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCnOnudBE9HbwLT+0xurpJylZO3A+jCMtGSUsSiYbv3YNASk14HJhcVAnWvdZkOpU5vM/HyokrS4ag9qx4dLHZVImulqxXjV394rwK8LK9FDlrxcyakx2iuWXURj0mWe+S+1US+KPo4HHJ0pRzA+aYTKQ3fkBVBNNZE+6dBHaqY7AejZsa/oYrK+7oZKxMmQZePoyWhldAputGsRbJ7V7bt+0LU0GNGaEq2KwhIIOLxxpI+DaUZM6vnxFise2A7w6dM/mA5QrZt/cyENbX28DHIWKz428XhP1NS5oIgs++DcWWJB/SuPaQo7t212WDSuXAtei9EBmQ5o5LxK6+x1k23AgMBAAECggEBAJrqqqkC3Q6n4E7QdjXysuhDeNwKWw3MVijfVBm5wI+iuB79NhYZSzjDVpJ8tpXCva52yKSlg/tn4JuAch1Emzqy8FZA773zyLrcJgR8wMGQUt9qdVohAPDDHphtzRYDIB1JJK2k+GgSslUswD5lB1yoo5l/uLD3aafuGTtRaEDQp57dM8n4W/QMzvmiPHr1jEMlx9WDAuS1Nw53nBg5Uf4mp6nZ/sCFyR7l09VIWrMueqt7qUSsH0l5UGW04UO6eFBPcWyEtFDiJNwKso/Y2lrUbf/3AM6fotnKhcXtuFkEJ8s5cwpm7+OA01zr/uW5L0wGVO0rw9Kv+w7TYIA0TkECgYEAyPvSz0SYD3NXsOAmuGpks7oadAHY1cgV1xu6dAyOooOLqnEjrrr5/wrqu6eK+rVrno1NZEQGEC6b260NQg3v5AJHITzoiNYcRjDLm4aZjnx3dwNGXvcylOkidI8XG4KgR1veM85dgObFKTMyQJ+0eUYJL/UjhZAIC/n8lM5VgosCgYEA1QE4Viw4HA/JQ7jtdSicDvEnF3M7eahH7VwG90G6ofY5DuQwxH1g9s+ExBAW3l7tOjfKltm+PSVfBtXIBVCxADO1/ZgsTDMGkraorxbudgLaU2PEsUWZcCWAUaPhlVUhZipTBCfafqGF757BUftkrXqg7Yy/6wyQcESs5frtYwUCgYEAknqWReFkQb56prp2/ejsw+Ba7zl5YzWUVVYsKfAM9HyTCgGzU+GJ2kuGkIWnUNlwOfoZ8X1yPdD6Xrxc8UtfDvpqBNtzTmdd6/ocKpmKyMIF/4Mvgn7/KnBPYEv5N1+YmOlnpLI+i3elMkXR1i+PROO6Rm2PGgTDGJd1cq5+u8kCgYBQUiJ1VD1gT4+civp4CHU4qTUNgbV2vb9JwT8bM9z1wAxqEiBVp9XNnBk7ebm15uPb5TfuxHMZSaNYx3qijngAVH+W/jAOF9utrVVUmPgY5iB/+4orMsyWXn3Ry1OAZVav2NvvIDwjLjN8VUge6wZe6HQQv9eLAfThcPQl0QZ9JQKBgBz+L3HfvHLGghZaAfIUzwOryRRmkjJNFErZEntLVqGYj3FZiEEwWMm2BZ9hm9nIWBPyIBJ1rZ5aGGjtNz8Lwb97LjrlsgVjI6QIGfuFfw9SgGiFo9f8vEQwzLPoceze2MAJLJo7p6RIOCwxqKzYTJXjXhjQPg6crgqNWWODus4H",
                "clientCert": "MIID4DCCAsigAwIBAgIRAK7nAdh5AeZOKCmqPCVEpFAwDQYJKoZIhvcNAQELBQAwgYkxFTATBgNVBAYTDEFua2gtTW9ycG9yazEaMBgGA1UEChMRVW5zZWVuIFVuaXZlcnNpdHkxEDAOBgNVBAsTB0xpYnJhcnkxQjBABgNVBAMTOUxhbmRvb3AncyBGYXN0IERhdGEgRGV2IFNlbGYgU2lnbmVkIENlcnRpZmljYXRlIEF1dGhvcml0eTAeFw0yMDA4MTMwODM1MjdaFw0zMDA4MTEwODM1MjdaMFYxFTATBgNVBAYTDEFua2gtTW9ycG9yazEaMBgGA1UEChMRVW5zZWVuIFVuaXZlcnNpdHkxEDAOBgNVBAsTB0xpYnJhcnkxDzANBgNVBAMTBmNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKc6e50ET0dvAtP7TG6uknKVk7cD6MIy0ZJSxKJhu/dg0BKTXgcmFxUCda91mQ6lTm8z8fKiStLhqD2rHh0sdlUia6WrFeNXf3ivArwsr0UOWvFzJqTHaK5ZdRGPSZZ75L7VRL4o+jgccnSlHMD5phMpDd+QFUE01kT7p0EdqpjsB6Nmxr+hisr7uhkrEyZBl4+jJaGV0Cm60axFsntXtu37QtTQY0ZoSrYrCEgg4vHGkj4NpRkzq+fEWKx7YDvDp0z+YDlCtm39zIQ1tfbwMchYrPjbxeE/U1LmgiCz74NxZYkH9K49pCju3bXZYNK5cC16L0QGZDmjkvErr7HWTbcCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMDQGA1UdEQQtMCuCCWxvY2FsaG9zdIIMYTk3Yzg3YmFiZDk4hwR/AAABhwTAqGNkhwR/AAABMA0GCSqGSIb3DQEBCwUAA4IBAQB35sqpr7S82PWJz9GFmYJioMsEo/vMBuG75Q1UyQJrlhNMX7CvnHJAfIeQoWiWJjpOKVW8vZeEVWc5pHwhIsuCd3VmKESkhk/e8HUqzwcXAo38f54NSLsmgNlJTDmIiQP+L2DZi91tqemN+OFHJv0qpj+RyjWtRYs2hNQ0my4WwLjZLSbOaTaCs3YXgvktaPE7MAOe4+GvcQ/d+OB9uqGmQu0nCUWx8NX93e17Zlus69i1O8H2/NfU+dTkVyEkcGPoe4UjkJwLUvHPUwtyvdhEn1FnNM21nY54Z/ohTtds6tJuC0y7CmbGKza5zvtBTfM0nOCmQD8erubKv7Gh/OFE",
                "username": "alice",
                "password": "plain:alice-secret"
            }]
        }
    }'
    ```
    Response: `200`
    ```json
    {
        "id": "localkafka",
        "addresses": { "PLAINTEXT": ["localk8s:9092"] }, 
        "default": true,
        "maasProtocol": "PLAINTEXT",
        "caCert": "MIIDPjCCAiYCCQCNmVmmEXs5XjANBgkqhkiG9w0BAQsFADBVMQswCQYDVQQGEwJYWDEVMBMGA1UEBwwMRGVmYXVsdCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMREwDwYDVQQDDAhsb2NhbGs4czAeFw0yMDA4MTgxMjUzMDFaFw0yMDExMjUxMjUzMDFaMG0xEDAOBgNVBAYTB1Vua25vd24xEDAOBgNVBAgTB1Vua25vd24xEDAOBgNVBAcTB1Vua25vd24xEDAOBgNVBAoTB1Vua25vd24xEDAOBgNVBAsTB1Vua25vd24xETAPBgNVBAMTCGxvY2FsazhzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjcTaRTo7E7UI9jO9gtRAi5au57elqRX2YMj/OOGcwQdzP6JfMFZsKFNUoIoF8bJ51JXhbDxVgB+GHvEMmQ0jqGnMjSTsdxEQUCRTnINMAIAYLBKm5FGi5pJodZRzhNKoWhloRO9/2p2AYB+T39MxXFch3fwMdghVKbSqOCo0nsqCZwyB5CcZgLi69qifZPQAIFPUPDHG5Z6oGUjE/p+45RnOcAdCOgO0QllxO+fioCMPizRqIiim88UuZU7EjhaIwSTjOIohcPQStNU6vAp0ZGIgr8BhAZHiL8JRDto37ayo7ltDYtLg4Ojo3e9ue8Dwo5PSs+N6Od8Z//Xq6V8zwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCDsV/jDojj4t977V5BMSTDeELEvNX8VMMtqAGpB1jtVNVXRLfG2SAcv6ZdOUbyuBagF8D0dsV5VvcqPw8YHNpHMKCTSdcida28rV2C31M+XRvvh90eoPtXfE60wo4Ky4UbKiJERiBIXMFLrg8PZ51PukT4fD0DioNpIxzRFb4VkypYv4srADX5shSvJN9Zxdj2EywR+S1k2F4TIDdOnWY9xGMftJz1fc58dMFMwGi7Evr+pR/w7yWDcvRgdgAYUpGaehmnYhzuw4XKWmVX1D6aVFRhonHaxN0rUemPZYSqjHp+lWOiUsYqatxB5MGQqj1/QV3XTAWglFbk1BB2stQg",
        "credentials": {      
            "admin": [{
                "type": "SCRAM",
                "username": "admin",
                "password": "plain:admin-secret"
            }],   
            "client": [{
                "type": "sslCert+SCRAM",
                "clientKey": "MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQCnOnudBE9HbwLT+0xurpJylZO3A+jCMtGSUsSiYbv3YNASk14HJhcVAnWvdZkOpU5vM/HyokrS4ag9qx4dLHZVImulqxXjV394rwK8LK9FDlrxcyakx2iuWXURj0mWe+S+1US+KPo4HHJ0pRzA+aYTKQ3fkBVBNNZE+6dBHaqY7AejZsa/oYrK+7oZKxMmQZePoyWhldAputGsRbJ7V7bt+0LU0GNGaEq2KwhIIOLxxpI+DaUZM6vnxFise2A7w6dM/mA5QrZt/cyENbX28DHIWKz428XhP1NS5oIgs++DcWWJB/SuPaQo7t212WDSuXAtei9EBmQ5o5LxK6+x1k23AgMBAAECggEBAJrqqqkC3Q6n4E7QdjXysuhDeNwKWw3MVijfVBm5wI+iuB79NhYZSzjDVpJ8tpXCva52yKSlg/tn4JuAch1Emzqy8FZA773zyLrcJgR8wMGQUt9qdVohAPDDHphtzRYDIB1JJK2k+GgSslUswD5lB1yoo5l/uLD3aafuGTtRaEDQp57dM8n4W/QMzvmiPHr1jEMlx9WDAuS1Nw53nBg5Uf4mp6nZ/sCFyR7l09VIWrMueqt7qUSsH0l5UGW04UO6eFBPcWyEtFDiJNwKso/Y2lrUbf/3AM6fotnKhcXtuFkEJ8s5cwpm7+OA01zr/uW5L0wGVO0rw9Kv+w7TYIA0TkECgYEAyPvSz0SYD3NXsOAmuGpks7oadAHY1cgV1xu6dAyOooOLqnEjrrr5/wrqu6eK+rVrno1NZEQGEC6b260NQg3v5AJHITzoiNYcRjDLm4aZjnx3dwNGXvcylOkidI8XG4KgR1veM85dgObFKTMyQJ+0eUYJL/UjhZAIC/n8lM5VgosCgYEA1QE4Viw4HA/JQ7jtdSicDvEnF3M7eahH7VwG90G6ofY5DuQwxH1g9s+ExBAW3l7tOjfKltm+PSVfBtXIBVCxADO1/ZgsTDMGkraorxbudgLaU2PEsUWZcCWAUaPhlVUhZipTBCfafqGF757BUftkrXqg7Yy/6wyQcESs5frtYwUCgYEAknqWReFkQb56prp2/ejsw+Ba7zl5YzWUVVYsKfAM9HyTCgGzU+GJ2kuGkIWnUNlwOfoZ8X1yPdD6Xrxc8UtfDvpqBNtzTmdd6/ocKpmKyMIF/4Mvgn7/KnBPYEv5N1+YmOlnpLI+i3elMkXR1i+PROO6Rm2PGgTDGJd1cq5+u8kCgYBQUiJ1VD1gT4+civp4CHU4qTUNgbV2vb9JwT8bM9z1wAxqEiBVp9XNnBk7ebm15uPb5TfuxHMZSaNYx3qijngAVH+W/jAOF9utrVVUmPgY5iB/+4orMsyWXn3Ry1OAZVav2NvvIDwjLjN8VUge6wZe6HQQv9eLAfThcPQl0QZ9JQKBgBz+L3HfvHLGghZaAfIUzwOryRRmkjJNFErZEntLVqGYj3FZiEEwWMm2BZ9hm9nIWBPyIBJ1rZ5aGGjtNz8Lwb97LjrlsgVjI6QIGfuFfw9SgGiFo9f8vEQwzLPoceze2MAJLJo7p6RIOCwxqKzYTJXjXhjQPg6crgqNWWODus4H",
                "clientCert": "MIID4DCCAsigAwIBAgIRAK7nAdh5AeZOKCmqPCVEpFAwDQYJKoZIhvcNAQELBQAwgYkxFTATBgNVBAYTDEFua2gtTW9ycG9yazEaMBgGA1UEChMRVW5zZWVuIFVuaXZlcnNpdHkxEDAOBgNVBAsTB0xpYnJhcnkxQjBABgNVBAMTOUxhbmRvb3AncyBGYXN0IERhdGEgRGV2IFNlbGYgU2lnbmVkIENlcnRpZmljYXRlIEF1dGhvcml0eTAeFw0yMDA4MTMwODM1MjdaFw0zMDA4MTEwODM1MjdaMFYxFTATBgNVBAYTDEFua2gtTW9ycG9yazEaMBgGA1UEChMRVW5zZWVuIFVuaXZlcnNpdHkxEDAOBgNVBAsTB0xpYnJhcnkxDzANBgNVBAMTBmNsaWVudDCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAKc6e50ET0dvAtP7TG6uknKVk7cD6MIy0ZJSxKJhu/dg0BKTXgcmFxUCda91mQ6lTm8z8fKiStLhqD2rHh0sdlUia6WrFeNXf3ivArwsr0UOWvFzJqTHaK5ZdRGPSZZ75L7VRL4o+jgccnSlHMD5phMpDd+QFUE01kT7p0EdqpjsB6Nmxr+hisr7uhkrEyZBl4+jJaGV0Cm60axFsntXtu37QtTQY0ZoSrYrCEgg4vHGkj4NpRkzq+fEWKx7YDvDp0z+YDlCtm39zIQ1tfbwMchYrPjbxeE/U1LmgiCz74NxZYkH9K49pCju3bXZYNK5cC16L0QGZDmjkvErr7HWTbcCAwEAAaN1MHMwDgYDVR0PAQH/BAQDAgWgMB0GA1UdJQQWMBQGCCsGAQUFBwMBBggrBgEFBQcDAjAMBgNVHRMBAf8EAjAAMDQGA1UdEQQtMCuCCWxvY2FsaG9zdIIMYTk3Yzg3YmFiZDk4hwR/AAABhwTAqGNkhwR/AAABMA0GCSqGSIb3DQEBCwUAA4IBAQB35sqpr7S82PWJz9GFmYJioMsEo/vMBuG75Q1UyQJrlhNMX7CvnHJAfIeQoWiWJjpOKVW8vZeEVWc5pHwhIsuCd3VmKESkhk/e8HUqzwcXAo38f54NSLsmgNlJTDmIiQP+L2DZi91tqemN+OFHJv0qpj+RyjWtRYs2hNQ0my4WwLjZLSbOaTaCs3YXgvktaPE7MAOe4+GvcQ/d+OB9uqGmQu0nCUWx8NX93e17Zlus69i1O8H2/NfU+dTkVyEkcGPoe4UjkJwLUvHPUwtyvdhEn1FnNM21nY54Z/ohTtds6tJuC0y7CmbGKza5zvtBTfM0nOCmQD8erubKv7Gh/OFE",
                "username": "alice",
                "password": "plain:alice-secret"
            }]
        }
    }
    ```

### Get Kafka instances
This API allows to get Kafka instances in Maas.  
* **URI:**  `{maas_host}/api/v2/kafka/instances`  
* **Method:** `GET`
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    No body
* **Success Response:**  
    `200` - Kafka instances were requested successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/instances' \
    --header 'Authorization: Basic bWFuYWdlcjoyNDAwMmVhZGJj' \
    ```
    Response: `200`
    ```json
    [
        {
            "id": "cpq-kafka-maas-test",
            "addresses": {
                "SASL_PLAINTEXT": [
                    "kafka.cpq-kafka:9092"
                ]
            },
            "default": true,
            "maasProtocol": "SASL_PLAINTEXT",
            "credentials": {
                "admin": [
                    {
                        "password": "plain:admin",
                        "type": "SCRAM",
                        "username": "admin"
                    }
                ],
                "client": [
                    {
                        "password": "plain:client",
                        "type": "SCRAM",
                        "username": "client"
                    }
                ]
            }
        }
    ]
    ```


### Update Kafka instance registration
This API allows to update connection properties for Kafka instance in Maas.  
* **URI:**  `{maas_host}/api/v2/kafka/instance`  
* **Method:** `PUT`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with connection properties for Kafka instance: 
    ```json
    {
      "id" : "<[optional] id for kafka instance, e.g. 'my-namespace-kafka'>",
      "addresses": "<map protocol: addresses list>", 
      "maasProtocol": "<[optional] procol used by MaaS for communcating with this kafka; default - PLAINTEXT",
      "default": "<[optional] bool flag that indicates whether this kafka instnace should be used by default>",
      "caCert": "<[optional] base64 CA certificate of this Kafka instance>",
      "credentials": { // [optional] Admin and client credentials for this kafka instance
        "admin": [ "<[optional] List of Kafka Auth DTOs containing admin credentials>" ],
        "client": [ "<[optional] List of Kafka Auth DTOs containing client credentials>" ]
    }
    ```
    Field addresses contains map where the key is protocol and value is a list of broker addresses. Allowed protocol values: `PLAINTEXT`, `SSL`, `SASL_PLAINTEXT`, `SASL_SSL`. 

    Field `"default": true` sets default Kafka instance to this one.
    Default instance will be used for new topics registration if another kafka instance is not specified in topic registration request.

    See [Kafka Auth DTO](#kafka-auth-dto) description for more details on Kafka client credentials format. 
* **Success Response:**  
    `200` - Kafka instance was updated successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X PUT \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/instance \
      -H 'Content-Type: application/json' \
      -d '{
        "id": "localkafka",
        "addresses": { "PLAINTEXT": ["localk8s:9092"] }, 
        "default": true,
        "maasProtocol": "PLAINTEXT",
        "caCert": "MIIDPjCCAiYCCQCNmVmmEXs5XjANBgkqhkiG9w0BAQsFADBVMQswCQYDVQQGEwJYWDEVMBMGA1UEBwwMRGVmYXVsdCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMREwDwYDVQQDDAhsb2NhbGs4czAeFw0yMDA4MTgxMjUzMDFaFw0yMDExMjUxMjUzMDFaMG0xEDAOBgNVBAYTB1Vua25vd24xEDAOBgNVBAgTB1Vua25vd24xEDAOBgNVBAcTB1Vua25vd24xEDAOBgNVBAoTB1Vua25vd24xEDAOBgNVBAsTB1Vua25vd24xETAPBgNVBAMTCGxvY2FsazhzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjcTaRTo7E7UI9jO9gtRAi5au57elqRX2YMj/OOGcwQdzP6JfMFZsKFNUoIoF8bJ51JXhbDxVgB+GHvEMmQ0jqGnMjSTsdxEQUCRTnINMAIAYLBKm5FGi5pJodZRzhNKoWhloRO9/2p2AYB+T39MxXFch3fwMdghVKbSqOCo0nsqCZwyB5CcZgLi69qifZPQAIFPUPDHG5Z6oGUjE/p+45RnOcAdCOgO0QllxO+fioCMPizRqIiim88UuZU7EjhaIwSTjOIohcPQStNU6vAp0ZGIgr8BhAZHiL8JRDto37ayo7ltDYtLg4Ojo3e9ue8Dwo5PSs+N6Od8Z//Xq6V8zwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCDsV/jDojj4t977V5BMSTDeELEvNX8VMMtqAGpB1jtVNVXRLfG2SAcv6ZdOUbyuBagF8D0dsV5VvcqPw8YHNpHMKCTSdcida28rV2C31M+XRvvh90eoPtXfE60wo4Ky4UbKiJERiBIXMFLrg8PZ51PukT4fD0DioNpIxzRFb4VkypYv4srADX5shSvJN9Zxdj2EywR+S1k2F4TIDdOnWY9xGMftJz1fc58dMFMwGi7Evr+pR/w7yWDcvRgdgAYUpGaehmnYhzuw4XKWmVX1D6aVFRhonHaxN0rUemPZYSqjHp+lWOiUsYqatxB5MGQqj1/QV3XTAWglFbk1BB2stQg",
        "credentials": {      
            "admin": [{
                "type": "SCRAM",
                "username": "admin",
                "password": "plain:admin-secret"
            }],   
            "client": [{
                "type": "SCRAM",
                "username": "alice",
                "password": "plain:alice-secret"
            }]
        }
    }'
    ```
    Response: `200 OK`
    ```json
    {
        "id": "localkafka",
        "addresses": {
            "PLAINTEXT": [
                "localk8s:9092"
            ]
        },
        "default": true,
        "maasProtocol": "PLAINTEXT",
        "caCert": "MIIDPjCCAiYCCQCNmVmmEXs5XjANBgkqhkiG9w0BAQsFADBVMQswCQYDVQQGEwJYWDEVMBMGA1UEBwwMRGVmYXVsdCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMREwDwYDVQQDDAhsb2NhbGs4czAeFw0yMDA4MTgxMjUzMDFaFw0yMDExMjUxMjUzMDFaMG0xEDAOBgNVBAYTB1Vua25vd24xEDAOBgNVBAgTB1Vua25vd24xEDAOBgNVBAcTB1Vua25vd24xEDAOBgNVBAoTB1Vua25vd24xEDAOBgNVBAsTB1Vua25vd24xETAPBgNVBAMTCGxvY2FsazhzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjcTaRTo7E7UI9jO9gtRAi5au57elqRX2YMj/OOGcwQdzP6JfMFZsKFNUoIoF8bJ51JXhbDxVgB+GHvEMmQ0jqGnMjSTsdxEQUCRTnINMAIAYLBKm5FGi5pJodZRzhNKoWhloRO9/2p2AYB+T39MxXFch3fwMdghVKbSqOCo0nsqCZwyB5CcZgLi69qifZPQAIFPUPDHG5Z6oGUjE/p+45RnOcAdCOgO0QllxO+fioCMPizRqIiim88UuZU7EjhaIwSTjOIohcPQStNU6vAp0ZGIgr8BhAZHiL8JRDto37ayo7ltDYtLg4Ojo3e9ue8Dwo5PSs+N6Od8Z//Xq6V8zwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCDsV/jDojj4t977V5BMSTDeELEvNX8VMMtqAGpB1jtVNVXRLfG2SAcv6ZdOUbyuBagF8D0dsV5VvcqPw8YHNpHMKCTSdcida28rV2C31M+XRvvh90eoPtXfE60wo4Ky4UbKiJERiBIXMFLrg8PZ51PukT4fD0DioNpIxzRFb4VkypYv4srADX5shSvJN9Zxdj2EywR+S1k2F4TIDdOnWY9xGMftJz1fc58dMFMwGi7Evr+pR/w7yWDcvRgdgAYUpGaehmnYhzuw4XKWmVX1D6aVFRhonHaxN0rUemPZYSqjHp+lWOiUsYqatxB5MGQqj1/QV3XTAWglFbk1BB2stQg",
        "credentials": {
            "admin": [
                {
                    "password": "plain:admin-secret",
                    "type": "SCRAM",
                    "username": "admin"
                }
            ],
            "client": [
                {
                    "password": "plain:alice-secret",
                    "type": "SCRAM",
                    "username": "alice"
                }
            ]
        }
    }
    ```


### Remove Kafka instance registration
This API allows remove Kafka instance from Maas.  
* **URI:**  `{maas_host}/api/v2/kafka/instance`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with Kafka instance id: 
    ```json
    {
      "id" : "<id of kafka instance, e.g. 'my-namespace-kafka'>"
    }
    ```
* **Success Response:**  
    `200` - Kafka instance was deleted successfully.  
* **Error Response:**   
    *Http code*: `400` - in case of removing kafka instance registration which has registered topics  
    *Response body:*  
    ```json
    {
      "error": "can't delete instance registration due of registered topics. Delete topics first"
    }
    ```     
    ---
    *Http code*: `500` - in case of internal server errors  
    *Response body:*  
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X DELETE \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/instance \
      -H 'Content-Type: application/json' \
      -d '{ "id": "14708d42-4344-4212-bf15-5a036a3e2437" }'
    ```
    Response: `200`
    ```json
    {
        "id": "14708d42-4344-4212-bf15-5a036a3e2437",
        "addresses": null,
        "default": false,
        "maasProtocol": "PLAINTEXT",
        "caCert": "",
        "credentials": null
    }
    ```

### Get or Create Kafka topic

This API allows to get or create Kafka topic.   
* **URI:**  `{maas_host}/api/v2/kafka/topic`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Query parameters:**
  * `onTopicExists` - This flag controls how MaaS should behave in following case: classifier specified in request is new to MaaS, but 
  physical topic with specified name is already presented in Kafka. 
      * `fail` - (defaults) fail applying configuration on such collision. MaaS assumes that topic shouldn't be exists in Kafka. This default behavior should prevents
        consuming toppic by typo or misconfiguration.
      * `merge` - ignore collision if it happens. Take existing topic if it exists (create new if not) and continue processing configuration. This option helps migrate existing topics to MaaS. Use this option with caution.
* **Request body:**
    ```json
    {
      "name": "<[optional] actual name of the topic that will be created in kafka. It could have a template value like 'maas.{{namespace}}.{{tenantId}}.{{name}}'. If not specified, name will be generated based on classifier info, more about classifier and its mandatory and optional fields you can find in README.md>",
      "classifier": {
        "name": "<your any custom name, it will be used as a part of generated topic name if no concrete name is specified in request>",
        "tenantId": "[optional]<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }, 
      "instance": "<[optional] id of kafka instance in which topic will be created. If not specified, default instance will be used>",
      "numPartitions": "<[optional] number of partitions for topic>; mutually exclusive to minNumPartitions", 
      "minNumPartitions": "<[optional] minimum number of partitions for topic>; mutually exclusive to numPartitions", 
      "replicationFactor": "<[optional] replication factor for topic. Value can be either positive integer or `inherit' (if Kafka version 2.4+) for server defaults>",
      "externallyManaged": "<[optional] property turns off all operations on Kafka instance. In case set true, topic name have to be set>", 
      "versioned": "<[optional] if set to true new topic will be created for each bg version", 
      "replicaAssignment": { //[optional] replica assignment for topic
          "0": [0, 1, 2]
      }, 
      "configs": { //[optional] topic configs
        "retention.ms": "1000"
      }
    }
    ```
    The request body contains a composite identification key that is called `classifier`. You need to know it for other APIs.  
* **Success Response:**  
    `201` - If the topic was created on your request.  
    or  
    `200` - If the topic was created earlier.  
     Response body:  
    ```json
    {
      "addresses": <map protocol: addresses list>, 
      "name": "<actual name of the topic that was created in kafka and can be used to access the topic>"
      "classifier": {
        "name": "<your any custom name>",
        "tenantId": "<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }, 
      "namespace": "my-namespace",
      "instance": "my-kafka-instance",
      "externallyManaged": "false",
      "versioned": "false",
      "caCert": "<[optional] base64 CA certificate of Kafka instance>", 
      "credentials": // [optional] client credentials if present
        {
           "client": [] // <List of Kafka Auth DTO objects (described below)>
        },
      "requestedSettings": { // topic settings that were specified in creation request
          "numPartitions": 1,
          "replicationFactor": 1,
          "replicaAssignment": null,
          "configs": null
      },
      "actualSettings": { // effective topic settings from Kafka (can differ from "requestedSettings" due to kafka server defaults)
          "numPartitions": 1,
          "replicationFactor": 1,
          "replicaAssignment": {
                "0": [
                    0
                ]
          },
          "configs": {
              "cleanup.policy": "delete",
              "compression.type": "producer",
              ...
          }
      },
    }
    ```
    For Kafka topic in case if you don't put "name" field in request (outside of classifier), then classifier is used to generate Kafka topic name by next contract: 
    In case if you have "tenantId" field:
    ```
    "maas.<namespace>.<tenantId>.<name>" 
    ```
    
    In case if you don't have "tenantId" field:
    ```
    "maas.<namespace>.<name>" 
    ```
    Please note, that overall topic name length (including namespace, tenantId and randomly generated part of the name) is limited to 249 symbols. 

    See [Kafka Auth DTO](#kafka-auth-dto) description for more details on Kafka client credentials format. 
* **Error Response:**
  
    *Http code*: `500` in case of internal server errors; `400` in case of invalid request data
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X POST \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: local' \
      -d '{
        "classifier": {
            "name": "sampleTopic7",
            "namespace": "local"
        },
        "numPartitions": 1,
        "replicationFactor": 1,
        "instance": "localkafka"
    }'
    ```
    Response:
    `201 CREATED`

    ```json
    {
        "addresses": {
            "PLAINTEXT": [
                "localkafka.kafka-cluster:9092"
            ],
            "SASL_SSL": [
                "localkafka.kafka-cluster:9093"
            ]
        },
        "name": "local.sampleTopic7.e083a1816e8f4547a7a0eaaecaf33645",
        "classifier": {
            "name": "sampleTopic7",
            "namespace": "local"
        },
        "namespace": "local",
        "instance": "localkafka",
        "caCert": "MIIDPjCCAiYCCQCNmVmmEXs5XjANBgkqhkiG9w0BAQsFADBVMQswCQYDVQQGEwJYWDEVMBMGA1UEBwwMRGVmYXVsdCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMREwDwYDVQQDDAhsb2NhbGs4czAeFw0yMDA4MTgxMjUzMDFaFw0yMDExMjUxMjUzMDFaMG0xEDAOBgNVBAYTB1Vua25vd24xEDAOBgNVBAgTB1Vua25vd24xEDAOBgNVBAcTB1Vua25vd24xEDAOBgNVBAoTB1Vua25vd24xEDAOBgNVBAsTB1Vua25vd24xETAPBgNVBAMTCGxvY2FsazhzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjcTaRTo7E7UI9jO9gtRAi5au57elqRX2YMj/OOGcwQdzP6JfMFZsKFNUoIoF8bJ51JXhbDxVgB+GHvEMmQ0jqGnMjSTsdxEQUCRTnINMAIAYLBKm5FGi5pJodZRzhNKoWhloRO9/2p2AYB+T39MxXFch3fwMdghVKbSqOCo0nsqCZwyB5CcZgLi69qifZPQAIFPUPDHG5Z6oGUjE/p+45RnOcAdCOgO0QllxO+fioCMPizRqIiim88UuZU7EjhaIwSTjOIohcPQStNU6vAp0ZGIgr8BhAZHiL8JRDto37ayo7ltDYtLg4Ojo3e9ue8Dwo5PSs+N6Od8Z//Xq6V8zwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCDsV/jDojj4t977V5BMSTDeELEvNX8VMMtqAGpB1jtVNVXRLfG2SAcv6ZdOUbyuBagF8D0dsV5VvcqPw8YHNpHMKCTSdcida28rV2C31M+XRvvh90eoPtXfE60wo4Ky4UbKiJERiBIXMFLrg8PZ51PukT4fD0DioNpIxzRFb4VkypYv4srADX5shSvJN9Zxdj2EywR+S1k2F4TIDdOnWY9xGMftJz1fc58dMFMwGi7Evr+pR/w7yWDcvRgdgAYUpGaehmnYhzuw4XKWmVX1D6aVFRhonHaxN0rUemPZYSqjHp+lWOiUsYqatxB5MGQqj1/QV3XTAWglFbk1BB2stQg",
        "credentials": {
            "client": [
                {
                    "password": "plain:alice-secret",
                    "type": "SCRAM",
                    "username": "alice"
                }
            ]
        },
        "requestedSettings": {
            "numPartitions": 1,
            "replicationFactor": 1,
            "replicaAssignment": null,
            "configs": null
        },
        "actualSettings": {
            "numPartitions": 1,
            "replicationFactor": 1,
            "replicaAssignment": {
                "0": [
                    0
                ]
            },
            "configs": {
                "cleanup.policy": "delete",
                "compression.type": "producer",
                "delete.retention.ms": "86400000",
                "file.delete.delay.ms": "60000",
                "flush.messages": "9223372036854775807",
                "flush.ms": "9223372036854775807",
                "follower.replication.throttled.replicas": "",
                "index.interval.bytes": "4096",
                "leader.replication.throttled.replicas": "",
                "max.compaction.lag.ms": "9223372036854775807",
                "max.message.bytes": "1048588",
                "message.downconversion.enable": "true",
                "message.format.version": "2.5-IV0",
                "message.timestamp.difference.max.ms": "9223372036854775807",
                "message.timestamp.type": "CreateTime",
                "min.cleanable.dirty.ratio": "0.5",
                "min.compaction.lag.ms": "0",
                "min.insync.replicas": "1",
                "preallocate": "false",
                "retention.bytes": "-1",
                "retention.ms": "604800000",
                "segment.bytes": "1073741824",
                "segment.index.bytes": "10485760",
                "segment.jitter.ms": "0",
                "segment.ms": "604800000",
                "unclean.leader.election.enable": "false"
            }
        }
    }
    ```

### Delete Kafka topic
This API allows to bulk delete Kafka topics by search parameters.  
* **URI:**  `{maas_host}/api/v2/kafka/topic`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Project namespace (project name) for which this topic was created;  
* **Authorization:**
    Basic type
* **Request body:**
    You should pass body with parameters (all of them are optional), so topics that matched these parameters will be removed. 
    In case `classifier` field is specified, MaaS will search for exact match (only `namespace` field of the classifier can be ommitted).
    ```json
    {
      "topic": "<actual topic name generated by MaaS on topic registration>",
      "namespace": "<core namespace where maas-agent installed>",
      "instance": "<kafka instance id>",
      "classifier": "<fully described topic classifier>",
      "leaveRealTopicIntact": "<true, if you want topic to be deleted only in MaaS db, not in Kafka broker>"
    }
    ```
* **Response:**  
    `200` - all matched topics were deleted successfully.  
    `500` - deletion of some topics failed.  
    `405` - topic deletion is not allowed by kafka server configuration. 
* **Response body:**
    ```json
    {
      "deletedSuccessfully": [ { <succesfully deleted topics DTOs (same as in Create Topic response) } ],
      "failedToDelete": [
        {
          "message": "<error message>"
          "topic": { <topic DTO (same as in Create Topic response) that was not deleted due to error> }
        }
      ]
    }
    ```

* **Error Response:**   

    **Http code**: `500`  - In case not a single topic was deleted  
    **Response body:** 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample calls**  

    **Delete all topics in namespace (useful for deployer)**

    Request:
    ```bash
    curl -X DELETE \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: local' \
      -d '{"namespace": "local"}'
    ```
    Response: `200`
    ```json
    {
        "deletedSuccessfully": [
            {
                "addresses": {
                    "PLAINTEXT": [
                        "localk8s:9092"
                    ]
                },
                "name": "local.sampleTopic1.690a32b2f9144d74ab62e7bafca949c5",
                "classifier": {
                    "name": "sampleTopic1",
                    "namespace": "local"
                },
                "namespace": "local",
                "instance": "localkafka",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": null,
                    "configs": null
                },
                "actualSettings": {
                    "numPartitions": 1,
                   "replicationFactor": 1,
                   "replicaAssignment": {
                       "0": [ 0 ]
                    },
                    "configs": {
                        "cleanup.policy": "delete"
                    }
                }
            },
            {
                "addresses": {
                    "PLAINTEXT": [
                        "localk8s:9092"
                    ]
                },
                "name": "local.sampleTopic2.aad0e967b41b4502a7bd-decde5e5bb06",
                "classifier": {
                    "name": "sampleTopic2",
                    "namespace": "local"
                },
                "namespace": "local",
                "instance": "localkafka",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": null,
                    "configs": null
                },
                "actualSettings": {
                    "numPartitions": 1,
                   "replicationFactor": 1,
                   "replicaAssignment": {
                       "0": [ 0 ]
                    },
                    "configs": {
                        "cleanup.policy": "delete"
                    }
                }
            }
        ],
        "failedToDelete": []
    }
    ```

    **Delete single topic by it's classifier and namespace**

    Request:
    ```bash
    curl -X DELETE \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: local' \
      -d '{
      "namespace": "local",
      "classifier": {"name": "sampleTopic"}
    }'
    ```
    Response: `200`
    ```json
    {
        "deletedSuccessfully": [
            {
                "addresses": {
                    "PLAINTEXT": [
                        "localk8s:9092"
                    ]
                },
                "name": "local.sampleTopic.690a32b2f9144d74ab62e7bafca949c5",
                "classifier": {
                    "name": "sampleTopic",
                    "namespace": "local"
                },
                "namespace": "local",
                "instance": "localkafka",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": null,
                    "configs": null
                },
                "actualSettings": {
                    "numPartitions": 1,
                   "replicationFactor": 1,
                   "replicaAssignment": {
                       "0": [ 0 ]
                    },
                    "configs": {
                        "cleanup.policy": "delete"
                    }
                }
            }
        ],
        "failedToDelete": []
    }
    ```

### Get Kafka topic by classifier
This API allows to get Kafka topic by exact match to given classifier. If you need to search topics by partial matching use [Search Kafka topics](#search-kafka-topics).
* **URI:**  `{maas_host}/api/v2/kafka/topic/get-by-classifier`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:** 
    Body must contain topic classifier fields. 
    ```json
    {
        "name": "<your any custom name>",
        "tenantId": "[optional]<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
    }
    ```
    The request body contains a composite identification key that is called `classifier`. You need to know it for other APIs.  
* **Success Response:**  
    `200`
     Response body:  
    ```json
    {
      "addresses": {
          "PLAINTEXT": [
              "my-kafka.kafka-cluster:9092"
           ]
      }, 
      "name": "<actual name of the topic that was created in kafka and can be used to access the topic>"
      "classifier": {
        "name": "<your any custom name, it will be used as a part of actual topic name>",
        "tenantId": "<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }, 
      "namespace": "my-namespace",
      "instance": "my-kafka-instance",
      "versioned": "false",
      "requestedSettings": {
        "numPartitions": 1,
        "replicationFactor": 1,
        "replicaAssignment": null,
        "configs": null
      },
      "actualSettings": {
        "numPartitions": 1,
        "replicationFactor": 1,
        "replicaAssignment": {
          "0": [ 0 ]
        },
        "configs": {
          "cleanup.policy": "delete"
          }
        } 
    }
    ```

Null value means that it was not specified during creation.

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors; `400` in case of invalid request data
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl -X POST \
      http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic/get-by-classifier \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: local' \
      -d '{
        "name": "sampleTopic2",
        "namespace": "local"
    }'
    ```
    Response:
    ```json
    200
    {
        "addresses": {
            "PLAINTEXT": [
                "localkafka.default:9092"
            ]
        },
        "name": "local.sampleTopic2.430d243ba9c74f25bf05f492a52804fd",
        "classifier": {
            "name": "sampleTopic2",
            "namespace": "local"
        },
        "namespace": "local",
        "instance": "localkafka",
        "caCert": "",
        "credentials": null,
        "requestedSettings": {
            "numPartitions": 1,
            "replicationFactor": 1,
            "replicaAssignment": null,
            "configs": null
        },
        "actualSettings": {
            "numPartitions": 1,
            "replicationFactor": 1,
            "replicaAssignment": {
                "0": [
                    0
                ]
            },
            "configs": {
                "cleanup.policy": "delete",
                "compression.type": "producer",
                "delete.retention.ms": "86400000",
                "file.delete.delay.ms": "60000",
                "flush.messages": "10000",
                "flush.ms": "1000",
                "follower.replication.throttled.replicas": "",
                "index.interval.bytes": "4096",
                "leader.replication.throttled.replicas": "",
                "max.compaction.lag.ms": "9223372036854775807",
                "max.message.bytes": "1000012",
                "message.downconversion.enable": "true",
                "message.format.version": "2.5-IV0",
                "message.timestamp.difference.max.ms": "9223372036854775807",
                "message.timestamp.type": "CreateTime",
                "min.cleanable.dirty.ratio": "0.5",
                "min.compaction.lag.ms": "0",
                "min.insync.replicas": "1",
                "preallocate": "false",
                "retention.bytes": "1073741824",
                "retention.ms": "604800000",
                "segment.bytes": "1073741824",
                "segment.index.bytes": "10485760",
                "segment.jitter.ms": "0",
                "segment.ms": "604800000",
                "unclean.leader.election.enable": "false"
            }
        }
    }
    ```

### Search Kafka topics
This API allows to search Kafka topics. 
* **URI:**  `{maas_host}/api/v2/kafka/topic/search`  
* **Method:** `POST`
* **Headers:**  
`Content-Type: application/json`  
`X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
Basic type
* **Request body:** 
    Body can contain any combination of the following fields (all of them are optional). 
    
    Search will find results by exact match with the provided field values (so e.g. you cannot search topics by classifier part). 
    ```json
    {
        "classifier": {
            "name": "<your any custom name>",
            "tenantId": "<external tenant identifier>",
            ...
        }, 
        "topic": "<exact topic name in kafka>",
        "namespace": "<namespace to which topics belong>"
        "instance": "<id of kafka instance where topics are created>"
    }
    ```
* **Success Response:** 

    *HTTP code:* `200`
    
    *Response body:*
    ```json
    [
        {
            "addresses": {
                "PLAINTEXT": [ "my-kafka.kafka-cluster:9092" ]
            }, 
            "name": "<actual name of the topic that was created in kafka and can be used to access the topic>"
            "classifier": {
                "name": "<your any custom name, it will be used as a part of actual topic name>",
                "tenantId": "<external tenant identifier>",
                "namespace": "<namespace where service is deployed>"
            }, 
            "namespace": "my-namespace"
            "instance": "my-kafka-instance",
            "requestedSettings": {
                "numPartitions": 1,
                "replicationFactor": 1,
                "replicaAssignment": null,
                "configs": null
            },
            "actualSettings": {
                "numPartitions": 1,
                "replicationFactor": 1,
                "replicaAssignment": {
                    "0": [ 0 ]
                },
                "configs": {
                    "cleanup.policy": "delete"
                }
            } 
        },
        ...
    ]
    ```

* **Error Response:**
    
    *HTTP code*: `500` in case of internal server errors; `400` in case of invalid request data
    
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  
    
    Request:
    ```bash
    curl -X POST \
    http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic/search \
      -u maas:maas \
      -H 'Content-Type: application/json' \
      -H 'X-Origin-Namespace: local' \
      -d '{
        "namespace": "cloudbss311-platform-core-support-dev3"
    }'
    ```
    Response:
    ```json
    200
    [
        {
            "addresses": {
                "SASL_PLAINTEXT": [
                    "kafka.cpq-kafka:9092"
                ]
            },
            "name": "cloudbss311-platform-core-support-dev3.sampleTopic2.0981cc8d4e904d93a3aa3d47db12c1c1",
            "classifier": {
                "name": "sampleTopic2",
                "namespace": "cloudbss311-platform-core-support-dev3"
            },
            "namespace": "cloudbss311-platform-core-support-dev3",
            "instance": "cpq-kafka",
            "caCert": "",
            "credentials": {
                "client": [
                    {
                        "password": "plain:client",
                        "type": "SCRAM",
                        "username": "client"
                    }
                ]
            },
            "requestedSettings": {
                "numPartitions": 1,
                "replicationFactor": 1,
                "replicaAssignment": null,
                "configs": null
            },
            "actualSettings": {
                "numPartitions": 1,
                "replicationFactor": 1,
                "replicaAssignment": {
                    "0": [ 0 ]
                },
                "configs": {
                    "cleanup.policy": "delete"
                }
            }
        },
        {
            "addresses": {
                "SASL_PLAINTEXT": [
                    "kafka.cpq-kafka:9092"
                ]
            },
            "name": "cloudbss311-platform-core-support-dev3.sampleTopic1.41ef3e94a76d4af593b46b0a9641ace0",
            "classifier": {
                "name": "sampleTopic1",
                "namespace": "cloudbss311-platform-core-support-dev3"
            },
            "namespace": "cloudbss311-platform-core-support-dev3",
            "instance": "cpq-kafka",
            "caCert": "",
            "credentials": {
                "client": [
                    {
                        "password": "plain:client",
                        "type": "SCRAM",
                        "username": "client"
                    }
                ]
            },
            "requestedSettings": {
                "numPartitions": 1,
                "replicationFactor": 1,
                "replicaAssignment": null,
                "configs": null
            },
            "actualSettings": {
                "numPartitions": 1,
                "replicationFactor": 1,
                "replicaAssignment": {
                    "0": [ 0 ]
                },
                "configs": {
                    "cleanup.policy": "delete"
                }
            }
        }
    ]
    ```

### Kafka Auth DTO
| Field name | Data type | Required | Description |
|------------|-----------|----------|-------------|
|  **type** | string | true | Authorization mechanism type. Possible values: `plain`, `sslCert`, `SCRAM` (which stands for SASL-SCRAM-SHA512), `sslCert+plain`, `sslCert+SCRAM` |
| **username** | string | false | Holds username for SASL Plain or SASL-SCRAM-SHA512 authorization. Must not be empty in case `plain`, `SCRAM`, `sslCert+plain`, or `sslCert+SCRAM` auth is used. |
| **password** | string | false | Holds password for SASL Plain or SASL-SCRAM-SHA512 authorization. Must not be empty in case `plain`, `SCRAM`, `sslCert+plain`, or `sslCert+SCRAM` auth is used. Password is specified in format `"<password storage type>:<value>"`, e.g. `"plain:alice-secret"`. Currently only plaintext password passing is supported. |
| **clientKey** | string | false | Holds Base64-encoded client SSL private key for `sslCert` authorization. Must not be empty in case `sslCert`, `sslCert+plain`, or `sslCert+SCRAM` auth is used. |
| **clientCert** | string | false | Holds Base64-encoded client SSL certificate for `sslCert` authorization. Must not be empty in case `sslCert`, `sslCert+plain`, or `sslCert+SCRAM` auth is used. |


### Get templates by namespace

This API allows getting templates with namespace header
* **URI:**  `{maas_host}/api/v2/kafka/topic-templates`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    None
* **Success Response:**  
    `200`
     Response body:  
    ```json
    [
     {
         "Name": "my-template44",
         "numPartitions": 1,
         "replicationFactor": 1,
         "configs": {
             "flush.ms": "1088"
         }
     },
     {
         "Name": "my-template444",
         "numPartitions": 1,
         "replicationFactor": 1,
         "configs": {
             "flush.ms": "1088"
         }
     }
    ]
    ```

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
      curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic-templates' \
      --header 'X-Origin-Namespace: namespace' \
      --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
      --header 'Content-Type: application/json' 
    ```
    Response:
    `200 OK`

     ```json
     [
      {
          "Name": "my-template44",
          "numPartitions": 1,
          "replicationFactor": 1,
          "configs": {
              "flush.ms": "1088"
          }
      },
      {
          "Name": "my-template444",
          "numPartitions": 1,
          "replicationFactor": 1,
          "configs": {
              "flush.ms": "1088"
          }
      }
     ]
     ```

### Delete template

This API allows deleting template
* **URI:**  `{maas_host}/api/v2/kafka/topic-template`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    ```json
  {
      "name": "<template name>"
  }
  ```
* **Success Response:**  
    `200`
     Response body:  
    ```json
    {
        "Name": "my-template",
        "numPartitions": 1,
        "replicationFactor": 1,
        "configs": {
            "flush.ms": "1088"
        }
    }
    ```

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request DELETE 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/topic-template' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json' \
        --data-raw '{
            "name": "my-template"
        }'
    ```
    Response:
    `200 OK`

     ```json
    {
        "Name": "my-template",
        "numPartitions": 1,
        "replicationFactor": 1,
        "configs": {
            "flush.ms": "1088"
        }
    }
     ```

### Define lazy topic

This API allows defining lazy topic 
Lazy topic is an entity stored in register of maas, defining lazy topic does not create anything in Kafka  
Main feature of lazy topic is that you can define topic with partially-filled classifier (e.g. `{ "name" : "my-topic", "namespace" : "my-namespace" }` ) with all proper configurations  
Then you can create topic by lazy topic (see api below) only with fully-filled classifier, adding for example tenantId (e.g. `{ "name" : "my-topic",  "namespace" : "my-namespace", "tenantId" : "123" }` )  
And topic will be created with fully-filled classifier but with configs from lazy topics  

Proper lazy topic is chosen by the rule, that all fields with their values of defined lazy topic should be present in requested classifier  
Please notice, that if more than one lazy topic is compatible with requested topic it will be a conflict  

* **URI:**  `{maas_host}/api/v2/kafka/lazy-topic/definition`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    Same as for topic
    
* **Success Response:**  
    `201` - if created
    `200` - if exists or updated
     Response body:  
    ```json
    {
        "classifier": {
            "name": "my-lazy-topic"
        },
        "numPartitions": 1,
        "replicationFactor": "1",
        "configs": {
            "flush.ms": "1009"
        }
    }
    ```

* **Error Response:**
    *Http code*: `500` in case of internal server errors; `400` in case of bad request
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request POST 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/lazy-topic/definition' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json' \
        --data-raw '{
            "classifier": {
                "name": "my-lazy-topic"
            },
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1019"
            }
        }'
    ```
    Response:
    `200 OK`

     ```json
    {
        "classifier": {
            "name": "my-lazy-topic"
        },
        "numPartitions": 1,
        "replicationFactor": "1",
        "configs": {
            "flush.ms": "1009"
        }
    }
     ```
  
### Create topic by lazy topic

This API allows creating topic by lazy topic  
More info in `Define lazy topic` section
* **URI:**  `{maas_host}/api/v2/kafka/lazy-topic`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    ```json
      {
        "name": "<your any custom name, it will be used as a part of generated topic name if no concrete name is specified in request>",
        "tenantId": "[optional]<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }
  
  ```
* **Success Response:**  
    same as for topic

* **Error Response:**
    *Http code*: `500` in case of internal server errors; `400` in case of bad request; `404` in case if any compatible lazy topic was not found; `409` in casi if more than one topic is compatible with your classifier
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request POST 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/lazy-topic' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json' \
        --data-raw '{
                "name": "my-lazy-topic"
        }'
    ```
    Response:
    `200 OK`

     ```json
    {
        "addresses": {
            "PLAINTEXT": [
                "localhost:9092"
            ]
        },
        "name": "maas.namespace.my-lazy-topic12.c3b9b7e8d5be4c4e9396d674b7293190",
        "classifier": {
            "name": "my-lazy-topic"
        },
        "namespace": "namespace",
        "instance": "cpq-kafka-maas-test",
        "requestedSettings": {
            "numPartitions": 1,
            "replicationFactor": 1,
            "configs": {
                "flush.ms": "1088"
            }
        },
        "actualSettings": {
            "numPartitions": 1,
            "replicationFactor": 1,
            "replicaAssignment": {
                "0": [
                    0
                ]
            },
            "configs": {
                "cleanup.policy": "delete",
                "compression.type": "producer",
                "delete.retention.ms": "86400000",
                "file.delete.delay.ms": "60000",
                "flush.messages": "9223372036854775807",
                "flush.ms": "1088",
                "follower.replication.throttled.replicas": "",
                "index.interval.bytes": "4096",
                "leader.replication.throttled.replicas": "",
                "max.compaction.lag.ms": "9223372036854775807",
                "max.message.bytes": "1048588",
                "message.downconversion.enable": "true",
                "message.format.version": "2.6-IV0",
                "message.timestamp.difference.max.ms": "9223372036854775807",
                "message.timestamp.type": "CreateTime",
                "min.cleanable.dirty.ratio": "0.5",
                "min.compaction.lag.ms": "0",
                "min.insync.replicas": "1",
                "preallocate": "false",
                "retention.bytes": "-1",
                "retention.ms": "604800000",
                "segment.bytes": "1073741824",
                "segment.index.bytes": "10485760",
                "segment.jitter.ms": "0",
                "segment.ms": "604800000",
                "unclean.leader.election.enable": "false"
            }
        },
        "template": "my-template444"
    }
     ```
  
  
  
### Get lazy topic definitions

This API allows getting lazy topics with namespace header  
More info in `Define lazy topic` section
* **URI:**  `{maas_host}/api/v2/kafka/lazy-topics/definitions`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    None
* **Success Response:**  
    `200`
     Response body:  
    ```json
    [
        {
            "Classifier": "{\"name\":\"a233\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        },
        {
            "Classifier": "{\"name\":\"b1\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        }
    ]
    ```

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/lazy-topics' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json'
    ```
    Response:
    `200 OK`

     ```json
    [
        {
            "Classifier": "{\"name\":\"a233\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        },
        {
            "Classifier": "{\"name\":\"b1\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        }
    ]
     ```

### Delete lazy topic

This API allows deleting lazy topic definition. All topics that has been created by lazy topic definition stay
intact and can be deleted by using its classifier by separate calls.
* **URI:**  `{maas_host}/api/v2/kafka/lazy-topic`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    ```json
      {
        "name": "<your any custom name, it will be used as a part of generated topic name if no concrete name is specified in request>",
        "tenantId": "[optional]<external tenant identifier>",
        "namespace": "<namespace where service is deployed>"
      }
  ```
* **Success Response:**  
    `200`
     Response body:  
    ```json
        {
            "Classifier": "{\"name\":\"my-lazy-topic\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1019"
            }
        }
    ```

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request DELETE 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/lazy-topic' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json' \
        --data-raw '{
           
                "name": "my-lazy-topic"
            
        }'
    ```
    Response:
    `200 OK`

     ```json
    {
        "Classifier": "{\"name\":\"my-lazy-topic\"}",
        "Namespace": "namespace",
        "numPartitions": 1,
        "replicationFactor": "1",
        "configs": {
            "flush.ms": "1019"
        }
    }
     ```

### Get tenant topics declarations by namespace

Get list of tenant topic definitions filtered by namespace. 
Creation of tenant topics could be done using declarative approach 
(more info in `declarative_approach.md` ). More info about tenant-topics in [Readme](../README.md)

* **URI:**  `{maas_host}/api/v2/kafka/tenant-topics`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Success Response:**  
    `200`
     Response body:  
    ```json
    [
        {
            "Classifier": "{\"name\":\"a233\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        },
        {
            "Classifier": "{\"name\":\"b1\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        }
    ]
    ```

* **Error Response:**

    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/kafka/tenant-topics' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json'
    ```
    Response:
    `200 OK`

     ```json
    [
        {
            "Classifier": "{\"name\":\"a233\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        },
        {
            "Classifier": "{\"name\":\"b1\"}",
            "Namespace": "namespace",
            "numPartitions": 1,
            "replicationFactor": "1",
            "configs": {
                "flush.ms": "1007"
            }
        }
    ]
     ```
### Delete tenant topic definition

Deletes tenant-topic definition by given classifier. Note that tenant topics already produced by this defintion 
stay intact. You can delete these tenant topics by using [Delete topic](rest_api.md#delete-kafka-topic) API 
* **URI:**  `{maas_host}/api/v2/kafka/tenant-topic`
* **Method:** `DELETE`
* **Headers:**  
  `Content-Type: application/json`  
  `X-Origin-Namespace` - Namespace (project name) where a topic will be used;
* **Authorization:**
  Basic type
* **Request body:**
    ```json
      {
        "name": "<tenant topic name part>",
        "namespace": "<namespace where service is deployed>"
      }
  ```
* **Success Response:**  
  `200`
  Response body:
    ```json
        {
            "Classifier": { 
              "name": "my-lazy-topic",
               "namespace": "namespace"
            },
            "numPartitions": 1,
            "replicationFactor": "1"
        }
    ```

* **Error Response:**
  * Http code*: `404` tenant topic definition with given classifier is not found 
  * Http code*: `500` internal server error
  
  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**

  Request:
    ```bash
        curl --request DELETE 'localhost:8080/api/v2/kafka/tenant-topic' \
            --header 'X-Origin-Namespace: cloud-dev' \
            --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
            --header 'Content-Type: application/json' \
            --header 'Cookie: JSESSIONID=EA6AF99A3517E9BB0B3F3B1C319C9C35' \
            --data-raw '{
                "name": "abc",
                "namespace": "cloud-dev"
            }'
    ```
  Response:
  `200 OK`

     ```json
    {
        "classifier": {
            "name": "abc",
            "namespace": "cloud-dev"
        },
        "numPartitions": 1,
        "replicationFactor": 1
    }
     ```

### Kafka Topics Reconciliation

Allows to recreate kafka topics that exists in maas, but not in kafka
* **URI:**  `{maas_host}/api/v2/kafka/recovery/{namespace}`
* **Method:** `POST`
* **Headers:**  
  None
* **Authorization:**
  Basic type
* **Request body:**
  None
* **Success Response:**  
  `200`  
  Response body:  
  see sample

* **Sample call**

  Request:
    ```bash
        curl --location --request POST 'localhost:8080/api/v2/kafka/recovery/local'
    ```
  Response:
  `200 OK`

    ```json
    [
        {
            "name": "maas.local.test-topic-0",
            "classifier": {
                "name": "test-topic-0",
                "namespace": "local"
            },
            "status": "added"
        },
        {
            "name": "maas.local.test-topic-1",
            "classifier": {
                "name": "test-topic-1",
                "namespace": "local"
            },
            "status": "exists"
        },
        {
            "name": "maas.local.test-topic-2",
            "classifier": {
                "name": "test-topic-2",
                "namespace": "local"
            },
            "status": "error",
            "errMsg": "can not get topic info"
        }
    ]
    ```

### Kafka Single Topic Recovery

Allows to recreate kafka topics that exists in maas, but not in kafka
* **URI:**  `{maas_host}/api/v2/kafka/recover-topic`
* **Method:** `POST`
* **Headers:**  
  None
* **Authorization:**
  Basic type
* **Request body:**
  Classifier of required topic (see example)
* **Success Response:**  
  `200`  
  Response body:  
  see sample
* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```

  * **Sample call**

    Request:
      ```bash
          curl --location --request POST 'http://localhost:8080/api/v2/kafka/recover-topic' \
      --header 'X-Origin-Namespace: namespace' \
      --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
      --header 'Content-Type: application/json' \
      --data-raw ' {
      "name": "topic-recovery-test",
      "namespace": "namespace"
      }'
      ```
    Response:
    `200 OK`

      ```json
     {
    "name": "maas.namespace.topic-recovery-test",
    "classifier": {
        "name": "topic-recovery-test",
        "namespace": "namespace"
    },
    "status": "exists"
    }
      ```
    
Possible statuses: `"exists", "added", "not_found"`

### Discrepancy Report

Allows to get discrepancy between Kafka topics from MaaS database and Kafka
* **URI:**  `{maas_host}/api/v2/kafka/discrepancy-report/{namespace}?status=ok|absent`
* **Method:** `GET`
* **Headers:**  
  None
* **Authorization:**
  None
* **Request body:**
  None
* **Query params:**
  `status` - values can be either `ok` or `absent`. 
  * `ok` - existing kafka topic we have valid registration record in MaaS
  * `absent` - MaaS has registration record in its database, but real topic is absent or missed in kafka 

* **Success Response:**  
  `200`  
  Response body:  
  see example
* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**

  Request:
    ```bash
        curl --location --request GET 'http://localhost:8080/api/v2/kafka/discrepancy-report/my-namespace'
    ```
  Response:
  `200 OK`

     ```json
     [
        {
            "name": "maas.test.81.my-awesome-test-topic",
            "classifier": {
                "name": "my-awesome-test-topic",
                "namespace": "my-namespace",
                "tenantId": "81"
            },
            "status": "ok"
        },
        {
            "name": "maas.test.82.my-awesome-test-topic",
            "classifier": {
                "name": "my-awesome-test-topic",
                "namespace": "my-namespace",
                "tenantId": "82"
            },
            "status": "absent"
        }
    ]
     ```

## Tenants API
### Get tenants

This API allows getting tenants stored in maas
It is used for tenant-topics, more info about tenant-topics in README.md
* **URI:**  `{maas_host}/api/v2/tenants`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    Empty
* **Success Response:**  
    `200`
     Response body:  
    ```json
  [
      {
          "namespace": "namespace",
          "externalId": "123",
          "tenantPresentation": {
              "externalId": "123"
          }
      },
      {
          "namespace": "namespace",
          "externalId": "456",
          "tenantPresentation": {
              "externalId": "456"
          }
      }
  ]
    ```

* **Error Response:**

    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/tenants' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json'
    ```
    Response:
    `200 OK`

     ```json
  [
      {
          "namespace": "namespace",
          "externalId": "123",
          "tenantPresentation": {
              "externalId": "123"
          }
      },
      {
          "namespace": "namespace",
          "externalId": "456",
          "tenantPresentation": {
              "externalId": "456"
          }
      }
  ]
     ```

### Synchronize tenants
This API allows sychronizing tenant in maas
It means that list of tenants you send will be compared with already stored in maas and new tenants will be added to register and all tenant-topics of your namespace will be created for new tenants
It is used for tenant-topics, more info about tenant-topics in README.md
* **URI:**  `{maas_host}/api/v2/synchronize-tenants`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
```json
    [
        {
            "externalId": <id>,
            "tenantPresentation": {
                <tenant info, as described in tenant manager>
            }
        },
        ...
    ]
```
* **Success Response:**  
    `200`
     Response body:  
    ```json
    [
    {
        "tenant": {
            "namespace": "us1",
            "externalId": "101",
            "tenantPresentation": {
                "externalId": "101"
            }
        },
        "topics": [
            {
                 <kafka topic entity>
            },
            ...
        ]
    }
    ]
    ```

* **Error Response:**

    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/synchronize-tenants' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json'
      --data-raw '
          [
              {
                "externalId": "101",
                "tenantPresentation": {
                    "externalId": "101"
                }
              } 
          ]
      '
    ```
    Response:
    `200 OK`

     ```json
    [
    {
        "tenant": {
            "namespace": "us1",
            "externalId": "101",
            "tenantPresentation": {
                "externalId": "101"
            }
        },
        "topics": [
            {
                "addresses": {
                    "PLAINTEXT": [
                        "localhost:9092"
                    ]
                },
                "name": "maas.us1.us1.b9c3addeade24f199652e3fd315d199b",
                "classifier": {
                    "name": "us1",
                    "tenantId": "101"
                },
                "namespace": "us1",
                "externallyManaged": false,
                "instance": "cpq-kafka-maas-test",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "configs": {
                        "flush.ms": "1004"
                    }
                },
                "actualSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": {
                        "0": [
                            0
                        ]
                    },
                    "configs": {
                        "cleanup.policy": "delete",
                        "compression.type": "producer",
                        "delete.retention.ms": "86400000",
                        "file.delete.delay.ms": "60000",
                        "flush.messages": "9223372036854775807",
                        "flush.ms": "1004",
                        "follower.replication.throttled.replicas": "",
                        "index.interval.bytes": "4096",
                        "leader.replication.throttled.replicas": "",
                        "max.compaction.lag.ms": "9223372036854775807",
                        "max.message.bytes": "1048588",
                        "message.downconversion.enable": "true",
                        "message.format.version": "2.6-IV0",
                        "message.timestamp.difference.max.ms": "9223372036854775807",
                        "message.timestamp.type": "CreateTime",
                        "min.cleanable.dirty.ratio": "0.5",
                        "min.compaction.lag.ms": "0",
                        "min.insync.replicas": "1",
                        "preallocate": "false",
                        "retention.bytes": "-1",
                        "retention.ms": "604800000",
                        "segment.bytes": "1073741824",
                        "segment.index.bytes": "10485760",
                        "segment.jitter.ms": "0",
                        "segment.ms": "604800000",
                        "unclean.leader.election.enable": "false"
                    }
                }
            },
            {
                "addresses": {
                    "PLAINTEXT": [
                        "localhost:9092"
                    ]
                },
                "name": "maas.us1.us12.4d8ae6bff264402e9a059534cc427b80",
                "classifier": {
                    "name": "us12",
                    "tenantId": "101"
                },
                "namespace": "us1",
                "externallyManaged": false,
                "instance": "cpq-kafka-maas-test",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "configs": {
                        "flush.ms": "1004"
                    }
                },
                "actualSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": {
                        "0": [
                            0
                        ]
                    },
                    "configs": {
                        "cleanup.policy": "delete",
                        "compression.type": "producer",
                        "delete.retention.ms": "86400000",
                        "file.delete.delay.ms": "60000",
                        "flush.messages": "9223372036854775807",
                        "flush.ms": "1004",
                        "follower.replication.throttled.replicas": "",
                        "index.interval.bytes": "4096",
                        "leader.replication.throttled.replicas": "",
                        "max.compaction.lag.ms": "9223372036854775807",
                        "max.message.bytes": "1048588",
                        "message.downconversion.enable": "true",
                        "message.format.version": "2.6-IV0",
                        "message.timestamp.difference.max.ms": "9223372036854775807",
                        "message.timestamp.type": "CreateTime",
                        "min.cleanable.dirty.ratio": "0.5",
                        "min.compaction.lag.ms": "0",
                        "min.insync.replicas": "1",
                        "preallocate": "false",
                        "retention.bytes": "-1",
                        "retention.ms": "604800000",
                        "segment.bytes": "1073741824",
                        "segment.index.bytes": "10485760",
                        "segment.jitter.ms": "0",
                        "segment.ms": "604800000",
                        "unclean.leader.election.enable": "false"
                    }
                }
            },
            {
                "addresses": {
                    "PLAINTEXT": [
                        "localhost:9092"
                    ]
                },
                "name": "maas.us1.us123.0a7519c7d82f4fb2928ba29e6fe96814",
                "classifier": {
                    "name": "us123",
                    "tenantId": "101"
                },
                "namespace": "us1",
                "externallyManaged": false,
                "instance": "cpq-kafka-maas-test",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "configs": {
                        "flush.ms": "1004"
                    }
                },
                "actualSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": {
                        "0": [
                            0
                        ]
                    },
                    "configs": {
                        "cleanup.policy": "delete",
                        "compression.type": "producer",
                        "delete.retention.ms": "86400000",
                        "file.delete.delay.ms": "60000",
                        "flush.messages": "9223372036854775807",
                        "flush.ms": "1004",
                        "follower.replication.throttled.replicas": "",
                        "index.interval.bytes": "4096",
                        "leader.replication.throttled.replicas": "",
                        "max.compaction.lag.ms": "9223372036854775807",
                        "max.message.bytes": "1048588",
                        "message.downconversion.enable": "true",
                        "message.format.version": "2.6-IV0",
                        "message.timestamp.difference.max.ms": "9223372036854775807",
                        "message.timestamp.type": "CreateTime",
                        "min.cleanable.dirty.ratio": "0.5",
                        "min.compaction.lag.ms": "0",
                        "min.insync.replicas": "1",
                        "preallocate": "false",
                        "retention.bytes": "-1",
                        "retention.ms": "604800000",
                        "segment.bytes": "1073741824",
                        "segment.index.bytes": "10485760",
                        "segment.jitter.ms": "0",
                        "segment.ms": "604800000",
                        "unclean.leader.election.enable": "false"
                    }
                }
            }
        ]
    }
    ]
     ```

## Blue green API
### Get blue green status

This API allows getting control plane messages stored in maas by namespace
* **URI:**  `{maas_host}/api/v2/bg-status`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    Empty
* **Success Response:**  
    `200`
     Response body:  
    ```json
    [
        {
            "namespace": "<namespace>",
            "timestamp": "<timestamp>",
            "active": "<string with active version>",
            "legacy": "<string with legacy version>",
            "candidates": "<array of candidates strings>"
        },
        ...
    ]
    ```

* **Error Response:**

    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/bg-status' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json'
    ```
    Response:
    `200 OK`

     ```json
    [
        {
            "namespace": "namespace",
            "timestamp": "2021-08-18T17:29:06.82837Z",
            "active": "v2",
            "legacy": "v1",
            "candidates": ["v3", "v4"]
        },
        {
            "namespace": "namespace",
            "timestamp": "2021-07-25T22:30:40.586303Z",
            "active": "v1",
            "legacy": "",
            "candidates": null
        }
    ]
     ```

### Apply control plane message 
This API allows apply cp message with blue-green status to maas
You MUST NOT do it manually, it is done via control-plane and maas-agent in cloud-core
* **URI:**  `{maas_host}/api/v2/bg-status`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
```json
    [
      {
        "version": "<version>",
        "stage": "<ACTIVE, LEGACY, CANDIDATE, ARCHIVE>",
        "createdWhen": "<timestamp>",
        "updatedWhen": "<timestamp>"
      },
      ...
    ] 
```
* **Success Response:**  
    `200`   
     Response body:  no body

* **Error Response:**

    *Http code*: `500` in case of internal server errors;   
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/bg-status' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json'
      --data-raw '
    [
      {
        "version": "v1",
        "stage": "LEGACY",
        "createdWhen": "2021-08-18T14:42:26.072479Z",
        "updatedWhen": "2021-08-18T14:42:26.072479Z"
      },
      {
        "version": "v2",
        "stage": "ACTIVE",
        "createdWhen": "2021-08-18T16:33:15.142354459Z",
        "updatedWhen": "2021-08-18T16:33:15.14235456Z"
      }
    ] 
      '
    ```
    Response:   
    `200 OK`
 

## Accounts API
### Get accounts

This API allows getting accounts
* **URI:**  `{maas_host}/api/v2/auth/accounts`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    None
* **Success Response:**  
    `200`
     Response body:  
    ```json
    [
        {
            "Username": "manager",
            "Roles": [
                "manager"
            ],
            "Salt": "df9234aba8",
            "Password": "42vZKOFp88OBCDenIEDnR2E/tekv9YkgUV+pxL1LrE22YOaX9afok0WfCyr8MpnySbvhaeAaFfp+ng73DHjjKw==",
            "Namespace": "_GLOBAL"
        },
        {
            "Username": "client",
            "Roles": [
                "agent",
                "manager"
            ],
            "Salt": "3929c98db9",
            "Password": "epsxkPJvEcPJ5Y35P5l0KaM1auTor5E4vkCOgH5ZtJk0mDzV+mfvFDl/UC46dhZ0CDqj3hLRf40JrUjANvvD2A==",
            "Namespace": "_GLOBAL"
        }
    ]
    ```

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
      curl --location --request GET 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/accounts' \
      --header 'X-Origin-Namespace: namespace' \
      --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
      --header 'Content-Type: application/json' 
    ```
    Response:
    `200 OK`

     ```json
    [
        {
            "Username": "manager",
            "Roles": [
                "manager"
            ],
            "Salt": "df9234aba8",
            "Password": "42vZKOFp88OBCDenIEDnR2E/tekv9YkgUV+pxL1LrE22YOaX9afok0WfCyr8MpnySbvhaeAaFfp+ng73DHjjKw==",
            "Namespace": "_GLOBAL"
        },
        {
            "Username": "client",
            "Roles": [
                "agent",
                "manager"
            ],
            "Salt": "3929c98db9",
            "Password": "epsxkPJvEcPJ5Y35P5l0KaM1auTor5E4vkCOgH5ZtJk0mDzV+mfvFDl/UC46dhZ0CDqj3hLRf40JrUjANvvD2A==",
            "Namespace": "_GLOBAL"
        }
     ]
     ```


### Create manager account
Manager account has a role `manager` who is able to manage broker instances.

* **URI:**  `{maas_host}/api/v2/auth/account/manager`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    No auth for first manager creation. Basic type with credentials with `manager` role for all except first. Specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    You need to pass body with connection properties for Rabbit instance: 
    ```json
    {
      "username": "<manager username>",
      "password": "<manager password>"
    }
    ```
* **Success Response:**  
    `201` - Manager account was created successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl --location --request POST 'http://localhost:8080/api/v2/auth/account/manager' \
    --header 'Content-Type: application/json' \
    --data-raw '{
      "username": "manager",
      "password": "24002eadbc",
    }
    '
    ```
    Response:
    ```json
    201
    ```
### Update manager's password
Update password for manager account.

* **URI:**  `{maas_host}/api/v2/auth/account/manager/{name}/password`
* **Method:** `PUT`
* **Headers:**  
  _None_
* **Authorization:**
  Basic type with credentials for `manager` role.
* **Request body:**
  New password as _plain text_
    ```text
    new-password
    ```
* **Success Response:**  
  `200` - Manager account password was changed successfully.
* **Error Response:**   
  *Http code*: `500`
  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**

  Request:
    ```bash
    curl --location --request PUT 'http://localhost:8080/api/v2/auth/account/manager/admin/password' \
    --header 'Authorization: Basic bWFuYWdlcjoyNDAwMmVhZGJj' \
    --data new-password
    ```
  Response:
    ```text
    200
    ```


### Create client account
Client account has a role `agent` who is able to manage broker entities of given instance.

* **URI:**  `{maas_host}/api/v2/auth/account/client`  
* **Method:** `POST`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    ```json
    {
      "username": "<client username>",
      "password": "<client password>",
      "namespace": "<client namespace>",
      "roles": [
            "<client roles>"
        ]
    }
    ```
* **Success Response:**  
    `201` - Client account was created successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl --location --request POST 'http://localhost:8080/api/v2/auth/account/client' \
    --header 'Authorization: Basic bWFuYWdlcjoyNDAwMmVhZGJj' \
    --header 'Content-Type: application/json' \
    --data-raw '{
        "username": "client",
        "password": "client",
        "roles": [
            "agent"
        ],
        "namespace": "namespace"
    }'
    '
    ```
    Response:
    ```json
    201
    ```


### Delete client account

* **URI:**  `{maas_host}/api/v2/auth/account/client`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
* **Authorization:**
    Basic type with credentials with `manager` role specified as `MAAS_ACCOUNT_MANAGER_USERNAME` and `MAAS_ACCOUNT_MANAGER_PASSWORD` deployment parameters.  
* **Request body:**
    ```json
    {
      "username": "<client username>",
      "password": "<client password>",
      "namespace": "<client namespace>",
      "roles": [
            "<client roles>"
        ]
    }
    ```
* **Success Response:**  
    `204` - Client account was deleted successfully.  
* **Error Response:**   
    *Http code*: `500`
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
    curl --location --request DELETE 'http://localhost:8080/api/v2/auth/account/client' \
    --header 'Authorization: Basic bWFuYWdlcjoyNDAwMmVhZGJj' \
    --header 'Content-Type: application/json' \
    --data-raw '{
        "username": "client",
        "password": "client",
        "roles": [
            "agent"
        ],
        "namespace": "namespace"
    }'
    '
    ```
    Response:
    ```json
    200
    ```
  
## Instance designators API

Note, that same API exists for RabbitMQ, but URI should be `{maas_host}/api/v2/rabbit/instance-designator`  

### Get kafka instance designators
Get kafka instance designator. You can have only one instance designator of broker per namespace.

* **URI:**  `{maas_host}/api/v2/kafka/instance-designator`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name);  
* **Authorization:**
    Basic type
* **Request body:**
    None
* **Success Response:**  
    `200`
     Response body:  
    ```json
     {
         "namespace": "...",
         "defaultInstance": "...",
         "selectors": [
             {
                 "classifierMatch": {
                     "name": "..."
                 },
                 "instance": "..."
             },
             {
                 "classifierMatch": {
                     "tenantId": "..."
                 },
                 "instance": "..."
             }
         ]
     }
    ```

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'localhost:8080/api/v2/kafka/instance-designator' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' 
    ```
    Response:
    `200 OK`

     ```json
    {
        "namespace": "namespace",
        "defaultInstance": "",
        "selectors": [
            {
                "classifierMatch": {
                    "name": "smoke-test-4"
                },
                "instance": "cpq-kafka-maas-test"
            },
            {
                "classifierMatch": {
                    "tenantId": "82133ba8-4bf9-4659-9e02-62e608bab645"
                },
                "instance": "cpq-kafka-maas-test"
            }
        ]
    }
     ```


### Delete kafka instance designators

Delete Kafka instance designator

* **URI:**  `{maas_host}/api/v2/kafka/instance-designator`  
* **Method:** `GET`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name);  
* **Authorization:**
    Basic type
* **Request body:**
    None
* **Success Response:**  
    `200`
     Response body:  
     None


* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request DELETE 'localhost:8080/api/v2/kafka/instance-designator' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' 
    ```
    Response:
    `200 OK`

  
  
  

### Get rabbit instance designators
Get rabbit instance designator. You can have only one instance designator of broker per namespace.

* **URI:**  `{maas_host}/api/v2/rabbit/instance-designator`  
* **Method:** `GET`
* **Headers:**  
  `Content-Type: application/json`  
  `X-Origin-Namespace` - Namespace (project name);  
* **Authorization:**
  Basic type
* **Request body:**
  None
* **Success Response:**  
  `200`
   Response body:  
  ```json
   {
       "namespace": "...",
       "defaultInstance": "...",
       "selectors": [
           {
               "classifierMatch": {
                   "name": "..."
               },
               "instance": "..."
           },
           {
               "classifierMatch": {
                   "tenantId": "..."
               },
               "instance": "..."
           }
       ]
   }
  ```

* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:* 
  ```json
  {
    "error": "<error message>"
  }
  ```
* **Sample call**  

  Request:
  ```bash
      curl --location --request GET 'localhost:8080/api/v2/rabbit/instance-designator' \
      --header 'X-Origin-Namespace: namespace' \
      --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' 
  ```
  Response:
  `200 OK`

   ```json
  {
      "namespace": "namespace",
      "defaultInstance": "",
      "selectors": [
          {
              "classifierMatch": {
                  "name": "smoke-test-4"
              },
              "instance": "cpq-kafka-maas-test"
          },
          {
              "classifierMatch": {
                  "tenantId": "82133ba8-4bf9-4659-9e02-62e608bab645"
              },
              "instance": "cpq-kafka-maas-test"
          }
      ]
  }
   ```


### Delete rabbit instance designators

Delete rabbit instance designator

* **URI:**  `{maas_host}/api/v2/rabbit/instance-designator`  
* **Method:** `GET`
* **Headers:**  
  `Content-Type: application/json`  
  `X-Origin-Namespace` - Namespace (project name);  
* **Authorization:**
  Basic type
* **Request body:**
  None
* **Success Response:**  
  `200`
   Response body:  
   None


* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:* 
  ```json
  {
    "error": "<error message>"
  }
  ```
* **Sample call**  

  Request:
  ```bash
      curl --location --request DELETE 'localhost:8080/api/v2/rabbit/instance-designator' \
      --header 'X-Origin-Namespace: namespace' \
      --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' 
  ```
  Response:
  `200 OK`



    
## Apply Configuration v1

Note, that this endpoint can't be used for nc.maas.rabbit/v2 config, it could be used only with aggregated config which is sent by deployer, see [Apply Configuration v2](#apply-configuration-v2)

***Configuration format***

MaaS service has a universal endpoint to apply set of configurations by one call. Given configurations set can be encoded either in JSON or 
YAML format. Each configuration should contain a preamble with `apiVersion`, `kind` and `spec` mandatory fields and optional `pragma`:
```yaml
apiVersion: nc.maas.kafka/v1 
kind: topic
pragma:
  kube-secret: topic-acccess-secret
  on-entity-exists: merge
spec: 
  ...
```
or in JSON:
```json
{
  "apiVersion": "nc.maas.kafka/v1",
  "kind": "topic",
  "pragma": {"kube-secret": "topic-acccess-secret", "on-entity-exists": "merge"},
  "spec": { 
    ...
  }
}
```
In case of passing multiple configurations at the time you should separate it by `---` in YAML: 
```yaml
apiVersion: nc.maas.kafka/v1 
kind: topic
spec: 
  classifier: 
    name: events
    namespace: ${ENV_NAMESPACE}

---
apiVersion: nc.maas.kafka/v1 
kind: topic
spec: 
  classifier: 
    name: orders
    namespace: ${ENV_NAMESPACE}
```

or form as array in JSON format:
```json
[
  {
    "apiVersion": "nc.maas.kafka/v1",
    "kind": "topic",
    "spec": {
      "classifier": {
        "name": "events",
        "namespace": "${ENV_NAMESPACE}"
      }
    }
  },
  {
    "apiVersion": "nc.maas.kafka/v1",
    "kind": "topic",
    "spec": {
      "classifier": {
        "name": "orders",
        "namespace": "${ENV_NAMESPACE}"
      }
    }
  }
]
```
Pragma section intended to contain some sort of instructions or options to correctly process configuration. Available fields are:
* `kube-secret` - string. Not processed by MaaS itself but used by platform cloud deployer. Deployer saves response 
data to secret with given name in project namespace. So microservice can attach this secret in its deployment configuration 
and access to topic at start time not need to request maas-agent/maas.
* `on-entity-exists` - flag with available `merge` or `fail` values. If topic in Kafka with `name` already exists and:
  * `on-entity-exists` set to `fail`, MaaS processing will fail with collision error: topic in Kafka with such name already exists. 
  * `on-entity-exists` set to `merge`, MaaS skip error and continue processing, going to second phase: apply requested topic properties on existsing topic. This phase can fail if, for example, requested `numPartitions` value is less than found on existing topic in Kafka and can't be downscaled automatically by MaaS.


***Supported configuration types***

* apiVersion: "nc.maas.kafka/v1", kind: "topic" 
* apiVersion: "nc.maas.kafka/v1", kind: "topic-template" 
* apiVersion: "nc.maas.kafka/v1", kind: "lazy-topic" 
* apiVersion: "nc.maas.kafka/v1", kind: "tenant-topic" 

* apiVersion: "nc.maas.kafka/v2", kind: "instance-designator" 

* apiVersion: "nc.maas.rabbit/v1", kind: "vhost" 

***Endpoint details*** 
* **URI:**  `{maas_host}/api/v1/config`  
* **Method:** `POST`
* **Headers:**  
    * `Content-Type: application/json`
    * `X-Origin-Namespace: <ns>`
* **Authorization:**
    Basic type with agent credentials  
* **Request body:**
    Text in JSON/YAML format. See details in topics this chapter above

***Responses:***
***OK 200***
All set of configurations is applied without errors. Example output: 
```json
[
  {
    "request": <...>,
    "result": {
       "status": "ok",
       "data": { ... } 
    }
  },
  ...
  {
      "request": <...>,
      "result": {
         "status": "error",
         "error": "error message" 
      }
    }

]
```

***BadRequest 400*** 
Invalid user input. Server can't parse request body neither as JSON nor as YAML. Response is plain text with error 

***ServerError 500***
Errors exists during configuration apply. Returns list of configurations with corresponding processing status. Note that 
some of configurations can be applied and some can be failed. For results with failed result field `status` has `error` value:
```json
[
  {
    "request": <...>,
    "response": {
       "status": "ok",
       "data": { ... } 
    }
  },
  ...
  {
      "request": <...>,
      "response": {
         "status": "error",
         "error": "error message" 
     }
  }
  ...
]
```

***Sample request:***
**Request**
```bash
curl --location --request POST 'localhost:8080/api/v1/config' \
    --header 'X-Origin-Namespace: cloud-dev' \
    --user 'client:secret' \
    --data-raw '---
apiVersion: nc.maas.kafka/v1
kind: topic
spec: 
  classifier:
    name: events
    namespace: my-namespace

---
apiVersion: nc.maas.kafka/v1
kind: topic
spec: 
  classifier:
    name: orders
    namespace: my-namespace
' 
```

**Response**
```json
[
    {
        "request": {
            "apiVersion": "nc.maas.kafka/v1",
            "kind": "topic",
            "spec": {
                "classifier": {
                    "name": "events",
                    "namespace": "cloud-dev"
                }
            }
        },
        "result": {
            "status": "ok",
            "data": {
                "addresses": {
                    "PLAINTEXT": [
                        "localhost:9092"
                    ]
                },
                "name": "maas.cloud-dev.events.b5c241b1298549618d28fde09f643b41",
                "classifier": {
                    "name": "events",
                    "namespace": "cloud-dev"
                },
                "namespace": "cloud-dev",
                "instance": "localkafka",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1
                },
                "actualSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": {
                        "0": [
                            0
                        ]
                    },
                    "configs": {
                        "cleanup.policy": "delete",
                        "compression.type": "producer",
                        "delete.retention.ms": "86400000",
                        "file.delete.delay.ms": "60000",
                        "flush.messages": "9223372036854775807",
                        "flush.ms": "9223372036854775807",
                        "follower.replication.throttled.replicas": "",
                        "index.interval.bytes": "4096",
                        "leader.replication.throttled.replicas": "",
                        "max.compaction.lag.ms": "9223372036854775807",
                        "max.message.bytes": "1048588",
                        "message.downconversion.enable": "true",
                        "message.format.version": "2.5-IV0",
                        "message.timestamp.difference.max.ms": "9223372036854775807",
                        "message.timestamp.type": "CreateTime",
                        "min.cleanable.dirty.ratio": "0.5",
                        "min.compaction.lag.ms": "0",
                        "min.insync.replicas": "1",
                        "preallocate": "false",
                        "retention.bytes": "-1",
                        "retention.ms": "604800000",
                        "segment.bytes": "1073741824",
                        "segment.index.bytes": "10485760",
                        "segment.jitter.ms": "0",
                        "segment.ms": "604800000",
                        "unclean.leader.election.enable": "false"
                    }
                }
            }
        }
    },
    {
        "request": {
            "apiVersion": "nc.maas.kafka/v1",
            "kind": "topic",
            "spec": {
                "classifier": {
                    "name": "orders",
                    "namespace": "cloud-dev"
                }
            }
        },
        "result": {
            "status": "ok",
            "data": {
                "addresses": {
                    "PLAINTEXT": [
                        "localhost:9092"
                    ]
                },
                "name": "maas.cloud-dev.orders.56b283e2461b42eb93ee53d5c915d269",
                "classifier": {
                    "name": "orders",
                    "namespace": "cloud-dev"
                },
                "namespace": "cloud-dev",
                "instance": "localkafka",
                "requestedSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1
                },
                "actualSettings": {
                    "numPartitions": 1,
                    "replicationFactor": 1,
                    "replicaAssignment": {
                        "0": [
                            0
                        ]
                    },
                    "configs": {
                        "cleanup.policy": "delete",
                        "compression.type": "producer",
                        "delete.retention.ms": "86400000",
                        "file.delete.delay.ms": "60000",
                        "flush.messages": "9223372036854775807",
                        "flush.ms": "9223372036854775807",
                        "follower.replication.throttled.replicas": "",
                        "index.interval.bytes": "4096",
                        "leader.replication.throttled.replicas": "",
                        "max.compaction.lag.ms": "9223372036854775807",
                        "max.message.bytes": "1048588",
                        "message.downconversion.enable": "true",
                        "message.format.version": "2.5-IV0",
                        "message.timestamp.difference.max.ms": "9223372036854775807",
                        "message.timestamp.type": "CreateTime",
                        "min.cleanable.dirty.ratio": "0.5",
                        "min.compaction.lag.ms": "0",
                        "min.insync.replicas": "1",
                        "preallocate": "false",
                        "retention.bytes": "-1",
                        "retention.ms": "604800000",
                        "segment.bytes": "1073741824",
                        "segment.index.bytes": "10485760",
                        "segment.jitter.ms": "0",
                        "segment.ms": "604800000",
                        "unclean.leader.election.enable": "false"
                    }
                }
            }
        }
    }
]
```

## Apply Configuration v2

This endpoint should be used mainly by deployer, because it aggregates all configs from every microservice and put its name both with its config. To send config manually use [Apply Configuration v1](#apply-configuration-v1)

***Configuration format***

```yaml
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: <version>
  namespace: <namespace>
  services: 
    - serviceName: <service-1-name>
      config: |+
          <configs of particular type body>
      
    - serviceName: <service-2-name>
      config: |+
          <configs of particular type body>

    - ...
```

***Supported configuration types***

* apiVersion: "nc.maas.kafka/v1", kind: "topic" 
* apiVersion: "nc.maas.kafka/v1", kind: "topic-template" 
* apiVersion: "nc.maas.kafka/v1", kind: "lazy-topic" 
* apiVersion: "nc.maas.kafka/v1", kind: "tenant-topic" 


* apiVersion: "nc.maas.kafka/v2", kind: "instance-designator" 


* apiVersion: "nc.maas.rabbit/v1", kind: "vhost"
* apiVersion: "nc.maas.rabbit/v2", kind: "vhost" 


***Endpoint details*** 
* **URI:**  `{maas_host}/api/v2/config`  
* **Method:** `POST`
* **Headers:**  
    * `Content-Type: application/json`
    * `X-Origin-Namespace: <ns>`
* **Authorization:**
    Basic type with credentials with 'agent' role
* **Request body:**
    Text in YAML format - see format above (JSON also can be used)

***Responses:***
***OK***

```json
{
    "status": "ok",
    "msResponses": [
        {
            "request": {
                "serviceName": <service-1-name>,
                "config": {
                    ...
                }
            },
            "result": {
                "status": "ok",
                "data": {
...
                }
            }
        },
        {
            "request": {
                "serviceName": <service-2-name>,
                "config": {
                    ...
                }
            },
            "result": {
                "status": "ok",
                "data": {
                    ...
                }
            }
        },
        ...       
    ]
}
```

***Error***
There could be mistakes of two main types: config parsing and internal server error in the particular config. In both cases you'll get overall field "status": "error".  

We save first error to return it on outer level of response, inner errors are described for any specific ms inside it anyway.  
So if no inner errors, but outer - user will see outer overall error (as for rabbit validation).  
If there is inner error, then user will see first inner error for kafka and we continue to process other configs if mistake is in one of kafka's configs.  
Or if it is rabbit error, then user will see first inner error for rabbit and no more rabbit configs are processed.  
 
In case you have overall error in your config, you will get no msResponses, but error body, for example for config with bad kafka kind in second microservice:

```yaml
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: core-dev
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: abc, namespace: namespace}
      
    - serviceName: order-executor
      config: |+
          ---
          apiVersion: nc.maas.kafka/v1
          kind: bad-kind
          spec:
             classifier: { name: foo-3, namespace: namespace}

```

**Response**

```json
{
    "status": "error",
    "error": "Error during applying aggregated config, error: 'configurator_service: bad input, check correctness of your YAML', message: (Unsupported object kind '{ApiVersion:nc.maas.kafka/v1 Kind:bad-kind}' in inner config 'map[apiVersion:nc.maas.kafka/v1 kind:bad-kind spec:map[classifier:map[name:foo-3 namespace:namespace]]]')",
    "msResponses": null
}
```

If you got some error during applying config in broker, you'll get overall error, but some configs could have "ok" status inside of array of inner configs. For example for second microservice config with wrong namespace:

```yaml
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: core-dev
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: abc, namespace: namespace}
      
    - serviceName: order-executor
      config: |+
          ---
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: foo-4, namespace: bad-namespace}

```

**Response**

```json
{
    "status": "error",
    "error": "Error during applying aggregated config, error: 'configurator_service: server error for internal config of ms', message: (Error during applying inner config for microservice with name 'order-executor', err: request: Classifier must be not null and with 'name' and 'namespace' fields. Namespace should be equal to namespace client can access. Error: bad namespace. Classifier namespace: 'bad-namespace'. Required namespace: 'core-dev'))",
    "msResponses": [
        {
            "request": {
                "serviceName": "order-processor",
                "config": {
                    "apiVersion": "nc.maas.kafka/v1",
                    "kind": "topic",
                    "spec": {
                        "classifier": {
                            "name": "abc",
                            "namespace": "namespace"
                        }
                    }
                }
            },
            "result": {
                "status": "ok",
                "data": {
                    "addresses": {
                        "PLAINTEXT": [
                            "localhost:9092"
                        ]
                    },
                    "name": "maas.namespace.abc",
                    "classifier": {
                        "name": "abc",
                        "namespace": "namespace"
                    },
                    "namespace": "namespace",
                    "externallyManaged": false,
                    "instance": "cpq-kafka-maas-test",
                    "requestedSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1
                    },
                    "actualSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1,
                        "replicaAssignment": {
                            "0": [
                                0
                            ]
                        },
                        "configs": {
                            "cleanup.policy": "delete",
                            "compression.type": "producer",
                            "delete.retention.ms": "86400000",
                            "file.delete.delay.ms": "60000",
                            "flush.messages": "9223372036854775807",
                            "flush.ms": "9223372036854775807",
                            "follower.replication.throttled.replicas": "",
                            "index.interval.bytes": "4096",
                            "leader.replication.throttled.replicas": "",
                            "max.compaction.lag.ms": "9223372036854775807",
                            "max.message.bytes": "1048588",
                            "message.downconversion.enable": "true",
                            "message.format.version": "2.8-IV1",
                            "message.timestamp.difference.max.ms": "9223372036854775807",
                            "message.timestamp.type": "CreateTime",
                            "min.cleanable.dirty.ratio": "0.5",
                            "min.compaction.lag.ms": "0",
                            "min.insync.replicas": "1",
                            "preallocate": "false",
                            "retention.bytes": "-1",
                            "retention.ms": "604800000",
                            "segment.bytes": "1073741824",
                            "segment.index.bytes": "10485760",
                            "segment.jitter.ms": "0",
                            "segment.ms": "604800000",
                            "unclean.leader.election.enable": "false"
                        }
                    }
                }
            }
        },
        {
            "request": {
                "serviceName": "order-executor",
                "config": {
                    "apiVersion": "nc.maas.kafka/v1",
                    "kind": "topic",
                    "spec": {
                        "classifier": {
                            "name": "foo-4",
                            "namespace": "bad-namespace"
                        }
                    }
                }
            },
            "result": {
                "status": "error",
                "error": "Error during applying aggregated config, error: 'configurator_service: server error for internal config of ms', message: (Error during applying inner config for microservice with name 'order-executor', err: request: Classifier must be not null and with 'name' and 'namespace' fields. Namespace should be equal to namespace client can access. Error: bad namespace. Classifier namespace: 'bad-namespace'. Required namespace: 'core-dev'))"
            }
        }
    ]
}
```

***Sample request:***
**Request**:

```bash
curl --location --request POST 'localhost:8080/api/v2/config' \
    --user 'client:secret' \
    --data-raw '---
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: core-dev
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: abc, namespace: namespace}
       
          ---
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: cde, namespace: namespace}
      
    - serviceName: order-executor
      config: |+
          ---
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: foo, namespace: namespace}
          
          ---
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: bar1, namespace: namespace}
' 
```

**Response**

```json
{
    "status": "ok",
    "msResponses": [
        {
            "request": {
                "serviceName": "order-processor",
                "config": {
                    "apiVersion": "nc.maas.kafka/v1",
                    "kind": "topic",
                    "spec": {
                        "classifier": {
                            "name": "abc",
                            "namespace": "namespace"
                        }
                    }
                }
            },
            "result": {
                "status": "ok",
                "data": {
                    "addresses": {
                        "PLAINTEXT": [
                            "localhost:9092"
                        ]
                    },
                    "name": "maas.namespace.abc",
                    "classifier": {
                        "name": "abc",
                        "namespace": "namespace"
                    },
                    "namespace": "namespace",
                    "externallyManaged": false,
                    "instance": "cpq-kafka-maas-test",
                    "requestedSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1
                    },
                    "actualSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1,
                        "replicaAssignment": {
                            "0": [
                                0
                            ]
                        },
                        "configs": {
                            "cleanup.policy": "delete",
                            "compression.type": "producer",
                            "delete.retention.ms": "86400000",
                            "file.delete.delay.ms": "60000",
                            "flush.messages": "9223372036854775807",
                            "flush.ms": "9223372036854775807",
                            "follower.replication.throttled.replicas": "",
                            "index.interval.bytes": "4096",
                            "leader.replication.throttled.replicas": "",
                            "max.compaction.lag.ms": "9223372036854775807",
                            "max.message.bytes": "1048588",
                            "message.downconversion.enable": "true",
                            "message.format.version": "2.8-IV1",
                            "message.timestamp.difference.max.ms": "9223372036854775807",
                            "message.timestamp.type": "CreateTime",
                            "min.cleanable.dirty.ratio": "0.5",
                            "min.compaction.lag.ms": "0",
                            "min.insync.replicas": "1",
                            "preallocate": "false",
                            "retention.bytes": "-1",
                            "retention.ms": "604800000",
                            "segment.bytes": "1073741824",
                            "segment.index.bytes": "10485760",
                            "segment.jitter.ms": "0",
                            "segment.ms": "604800000",
                            "unclean.leader.election.enable": "false"
                        }
                    }
                }
            }
        },
        {
            "request": {
                "serviceName": "order-processor",
                "config": {
                    "apiVersion": "nc.maas.kafka/v1",
                    "kind": "topic",
                    "spec": {
                        "classifier": {
                            "name": "cde",
                            "namespace": "namespace"
                        }
                    }
                }
            },
            "result": {
                "status": "ok",
                "data": {
                    "addresses": {
                        "PLAINTEXT": [
                            "localhost:9092"
                        ]
                    },
                    "name": "maas.namespace.cde",
                    "classifier": {
                        "name": "cde",
                        "namespace": "namespace"
                    },
                    "namespace": "namespace",
                    "externallyManaged": false,
                    "instance": "cpq-kafka-maas-test",
                    "requestedSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1
                    },
                    "actualSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1,
                        "replicaAssignment": {
                            "0": [
                                0
                            ]
                        },
                        "configs": {
                            "cleanup.policy": "delete",
                            "compression.type": "producer",
                            "delete.retention.ms": "86400000",
                            "file.delete.delay.ms": "60000",
                            "flush.messages": "9223372036854775807",
                            "flush.ms": "9223372036854775807",
                            "follower.replication.throttled.replicas": "",
                            "index.interval.bytes": "4096",
                            "leader.replication.throttled.replicas": "",
                            "max.compaction.lag.ms": "9223372036854775807",
                            "max.message.bytes": "1048588",
                            "message.downconversion.enable": "true",
                            "message.format.version": "2.8-IV1",
                            "message.timestamp.difference.max.ms": "9223372036854775807",
                            "message.timestamp.type": "CreateTime",
                            "min.cleanable.dirty.ratio": "0.5",
                            "min.compaction.lag.ms": "0",
                            "min.insync.replicas": "1",
                            "preallocate": "false",
                            "retention.bytes": "-1",
                            "retention.ms": "604800000",
                            "segment.bytes": "1073741824",
                            "segment.index.bytes": "10485760",
                            "segment.jitter.ms": "0",
                            "segment.ms": "604800000",
                            "unclean.leader.election.enable": "false"
                        }
                    }
                }
            }
        },
        {
            "request": {
                "serviceName": "order-executor",
                "config": {
                    "apiVersion": "nc.maas.kafka/v1",
                    "kind": "topic",
                    "spec": {
                        "classifier": {
                            "name": "foo",
                            "namespace": "namespace"
                        }
                    }
                }
            },
            "result": {
                "status": "ok",
                "data": {
                    "addresses": {
                        "PLAINTEXT": [
                            "localhost:9092"
                        ]
                    },
                    "name": "maas.namespace.foo",
                    "classifier": {
                        "name": "foo",
                        "namespace": "namespace"
                    },
                    "namespace": "namespace",
                    "externallyManaged": false,
                    "instance": "cpq-kafka-maas-test",
                    "requestedSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1
                    },
                    "actualSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1,
                        "replicaAssignment": {
                            "0": [
                                0
                            ]
                        },
                        "configs": {
                            "cleanup.policy": "delete",
                            "compression.type": "producer",
                            "delete.retention.ms": "86400000",
                            "file.delete.delay.ms": "60000",
                            "flush.messages": "9223372036854775807",
                            "flush.ms": "9223372036854775807",
                            "follower.replication.throttled.replicas": "",
                            "index.interval.bytes": "4096",
                            "leader.replication.throttled.replicas": "",
                            "max.compaction.lag.ms": "9223372036854775807",
                            "max.message.bytes": "1048588",
                            "message.downconversion.enable": "true",
                            "message.format.version": "2.8-IV1",
                            "message.timestamp.difference.max.ms": "9223372036854775807",
                            "message.timestamp.type": "CreateTime",
                            "min.cleanable.dirty.ratio": "0.5",
                            "min.compaction.lag.ms": "0",
                            "min.insync.replicas": "1",
                            "preallocate": "false",
                            "retention.bytes": "-1",
                            "retention.ms": "604800000",
                            "segment.bytes": "1073741824",
                            "segment.index.bytes": "10485760",
                            "segment.jitter.ms": "0",
                            "segment.ms": "604800000",
                            "unclean.leader.election.enable": "false"
                        }
                    }
                }
            }
        },
        {
            "request": {
                "serviceName": "order-executor",
                "config": {
                    "apiVersion": "nc.maas.kafka/v1",
                    "kind": "topic",
                    "spec": {
                        "classifier": {
                            "name": "bar1",
                            "namespace": "namespace"
                        }
                    }
                }
            },
            "result": {
                "status": "ok",
                "data": {
                    "addresses": {
                        "PLAINTEXT": [
                            "localhost:9092"
                        ]
                    },
                    "name": "maas.namespace.bar1",
                    "classifier": {
                        "name": "bar1",
                        "namespace": "namespace"
                    },
                    "namespace": "namespace",
                    "externallyManaged": false,
                    "instance": "cpq-kafka-maas-test",
                    "requestedSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1
                    },
                    "actualSettings": {
                        "numPartitions": 1,
                        "replicationFactor": 1,
                        "replicaAssignment": {
                            "0": [
                                0
                            ]
                        },
                        "configs": {
                            "cleanup.policy": "delete",
                            "compression.type": "producer",
                            "delete.retention.ms": "86400000",
                            "file.delete.delay.ms": "60000",
                            "flush.messages": "9223372036854775807",
                            "flush.ms": "9223372036854775807",
                            "follower.replication.throttled.replicas": "",
                            "index.interval.bytes": "4096",
                            "leader.replication.throttled.replicas": "",
                            "max.compaction.lag.ms": "9223372036854775807",
                            "max.message.bytes": "1048588",
                            "message.downconversion.enable": "true",
                            "message.format.version": "2.8-IV1",
                            "message.timestamp.difference.max.ms": "9223372036854775807",
                            "message.timestamp.type": "CreateTime",
                            "min.cleanable.dirty.ratio": "0.5",
                            "min.compaction.lag.ms": "0",
                            "min.insync.replicas": "1",
                            "preallocate": "false",
                            "retention.bytes": "-1",
                            "retention.ms": "604800000",
                            "segment.bytes": "1073741824",
                            "segment.index.bytes": "10485760",
                            "segment.jitter.ms": "0",
                            "segment.ms": "604800000",
                            "unclean.leader.election.enable": "false"
                        }
                    }
                }
            }
        }
    ]
}
```

Another example request:

```bash
curl --location --request POST 'localhost:8080/api/v2/config' \
    --user 'client:secret' \
    --data-raw '---
apiVersion: nc.maas.config/v2
kind: config
spec:
version: v1
namespace: cloudbss-kube-csrd-dev2
services:
{serviceName: security-scripts-cli, config: ''}
{serviceName: wp-runtime, config: ''}
' 
```


## Delete namespace

This API allows deleting ALL entities in particular namespace (topic, template, lazy topic, tenant topic, tenant, vhost)
* **URI:**  `{maas_host}/api/v2/namespace`  
* **Method:** `DELETE`
* **Headers:**  
    `Content-Type: application/json`  
    `X-Origin-Namespace` - Namespace (project name) where a topic will be used;  
* **Authorization:**
    Basic type
* **Request body:**
    ```json
      {
        "namespace": "<your namespace>"
      }
  ```
* **Success Response:**  
    `200`

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request DELETE 'http://maas-service-maas-core-dev.paas-apps8.openshift.sdntest.qubership.org/api/v2/namespace' \
        --header 'X-Origin-Namespace: namespace' \
        --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
        --header 'Content-Type: application/json' 
    ```
    Response:
    `200 OK`


## Monitoring

This API allows getting accounts
* **URI:**  `{maas_host}/api/v2/monitoring/entity-distribution`  
* **Method:** `GET`
* **Headers:**  
    None
* **Authorization:**
    Basic type with account with 'manager' role
* **Request body:**
    None
* **Success Response:**  
    `200`  
     Response body:  
     see example

* **Error Response:**
  
    *Http code*: `500` in case of internal server errors;
    *Response body:* 
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**  

    Request:
    ```bash
        curl --location --request GET 'http://localhost:8080/api/v2/monitoring/entity-distribution' \
        --data-raw ''
    ```
    Response:
    `200 OK`

     ```text
     maas_rabbit{namespace="maas-it-test", microservice="order-processor", broker_host="amqps://rabbitmq.maas-rabbitmq-2:5671", vhost="maas.maas-it-test.vers-test", broker_status="UP"}
     maas_rabbit{namespace="maas-it-test", microservice="", broker_host="amqps://rabbitmq.maas-rabbitmq-2:5671", vhost="maas.maas-it-test.it-test.VirtualHostBasicOperationsIT", broker_status="UP"}
     maas_kafka{namespace="maas-it-test", broker_host="{'SASL_PLAINTEXT': ['kafka.maas-kafka-2:9092']}", topic="maas.maas-it-test.it-test.KafkaTopicBasicOperationsIT", broker_status="UP"}
     ```

## vhost/topic request audit

This API allows getting vhost/topic request metrics in Prometheus format
* **URI:**  `{maas_host}/api/v2/monitoring/entity-request-audit`
* **Method:** `GET`
* **Headers:**  
  None
* **Authorization:**
  None
* **Request body:**
  None
* **Success Response:**  
  `200`  
  Response body:  
  see example

* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "error": "<error message>"
    }
    ```
* **Sample call**

  Request:
    ```bash
        curl --location --request GET 'http://localhost:8080/api/v2/monitoring/entity-request-audit'
    ```
  Response:
  `200 OK`

     ```text
    maas_kafka_entity_requests{namespace="maas-it-test", microservice="maas-it-test-microservice", instance="kafka-main", topic="maas.maas-it-test.it-test.KafkaTopicBasicOperationsIT"} 56
    maas_kafka_entity_requests{namespace="another-maas-it-test", microservice="maas-it-test-microservice", instance="kafka-main", topic="maas.another-maas-it-test.it-test.AnotherKafkaTopicBasicOperationsIT"} 72
    maas_rabbit_entity_requests{namespace="maas-it-test", microservice="maas-it-test-microservice", instance="rabbit-main", vhost="maas.maas-it-test.it-test.VirtualHostBasicOperationsIT"} 42
    maas_rabbit_entity_requests{namespace="another-maas-it-test", microservice="maas-it-test-microservice", instance="kafka-main", vhost="maas.another-maas-it-test.it-test.AnotherVirtualHostBasicOperationsIT"} 24
     ```
## Composite
### List composite structures

Get list of all registered composites. This method is useful for debug purposes
* **URI:**  `{maas_host}/api/composite/v1/structures`
* **Method:** `GET`
* **Headers:**  
  None
* **Authorization:**
  Basic type
* **Request body:**
  None
* **Success Response:**  
  `200`  
  Response body:  
  see example

* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "id": "47f79f65-82a0-4401-8321-d31abb3bd07d",
      "status": "500",
      "code": "MAAS-0600",
      "message": "text",
      "reason": "text",
      "@type": "NC.TMFErrorResponse.v1.0"
    }
    ```
* **Sample call**

  Request:
    ```bash
        curl --location --request GET 'http://localhost:8080/api/composite/v1/structures' \
          --header 'Authorization: Basic Y2xpZW50OmNsaWVudA=='
    ```
  Response:
  `200 OK`

     ```json
    [
      {
        "id": "baseNamespace2",
        "namespaces": [
          "baseNamespace2",
          "satelliteNamespace21"
        ]
      },
      {
        "id": "baseNamespace1",
        "namespaces": [
          "baseNamespace1",
          "satelliteNamespace11",
          "satelliteNamespace12"
        ]
      }
    ]
     ```

### Get composite structure by id

Get structure by id
* **URI:**  `{maas_host}/api/composite/v1/structure/{id}`
* **Method:** `GET`
* **Headers:**  
  None
* **Authorization:**
  Basic type
* **Request body:**
  None
* **Success Response:**  
  `200`  
  Response body:  
  see example

* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "id": "47f79f65-82a0-4401-8321-d31abb3bd07d",
      "status": "500",
      "code": "MAAS-0600",
      "message": "text",
      "reason": "text",
      "@type": "NC.TMFErrorResponse.v1.0"
    }
    ```
* **Sample call**

  Request:
    ```bash
        curl --location  --request GET 'http://localhost:8080/api/composite/v1/structure/mybaseline' \
          --header 'Authorization: Basic Y2xpZW50OmNsaWVudA=='
    ```
  Response:
  `200 OK`

     ```json
      {
        "id": "mybaseline",
        "namespaces": [
          "mybaseline",
          "satelliteNamespace"
        ]
      }
     ```

### Initialize/Update Composite Structure

Insert or update composite structure in XaaS. Validation should be performed on XaaS side that neither of namespaces is used in other composite
* **URI:**  `{maas_host}/api/composite/v1/structure`
* **Method:** `POST`
* **Headers:**  
  `X-Origin-Namespace` - Namespace (project name)
* **Authorization:**
  Basic type
* **Request body:**
  ```json
  {
     "id": "base",
     "namespaces": [
       "base",
       "satelliteNamespace"
     ]
  }
  ```
* **Success Response:**  
  `204`  
  Response body:  
  None

* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "id": "47f79f65-82a0-4401-8321-d31abb3bd07d",
      "status": "500",
      "code": "MAAS-0600",
      "message": "text",
      "reason": "text",
      "@type": "NC.TMFErrorResponse.v1.0"
    }
    ```
* **Sample call**

  Request:
    ```bash
    curl --location --request POST 'http://localhost:8080/api/composite/v1/structure' \
      --header 'X-Origin-Namespace: namespace' \
      --header 'Authorization: Basic Y2xpZW50OmNsaWVudA==' \
      --header 'Content-Type: application/json' \
      --data-raw ' {
         "id": "base",
         "namespaces": [
           "base",
           "satelliteNamespace"
         ]
      }'
    ```
  Response:
  `204 NoContent`


### Destroy composite structure registration

This method intended to destroy composite and remove its registration from MaaS
* **URI:**  `{maas_host}/api/composite/v1/structure/{id}`
* **Method:** `DELETE`
* **Headers:**  
  None
* **Authorization:**
  Basic type
* **Request body:**
  None
* **Success Response:**  
  `204`  
  Response body:  
  see example

* **Error Response:**

  *Http code*: `500` in case of internal server errors;
  *Response body:*
    ```json
    {
      "id": "47f79f65-82a0-4401-8321-d31abb3bd07d",
      "status": "500",
      "code": "MAAS-0600",
      "message": "text",
      "reason": "text",
      "@type": "NC.TMFErrorResponse.v1.0"
    }
    ```
* **Sample call**

  Request:
    ```bash
        curl --location  --request DELETE 'http://localhost:8080/api/composite/v1/structure/mybaseline' \
          --header 'Authorization: Basic Y2xpZW50OmNsaWVudA=='
    ```
  Response:
  `204 NoContent`
