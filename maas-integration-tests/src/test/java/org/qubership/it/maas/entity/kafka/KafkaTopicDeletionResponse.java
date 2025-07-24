package org.qubership.it.maas.entity.kafka;


import lombok.Data;

import java.util.List;

@Data
public class KafkaTopicDeletionResponse {
    private List<KafkaTopicResponse> deletedSuccessfully;
    private List<TopicDeletionError> failedToDelete;

    @Data
    public static class TopicDeletionError {
        private KafkaTopicResponse topic;
        private String message;
    }
}
