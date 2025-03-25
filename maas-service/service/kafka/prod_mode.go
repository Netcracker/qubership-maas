package kafka

import (
	"context"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/utils"
)

type ProdMode struct {
	KafkaService

	isProdMode bool
}

func NewProdMode(kafkaService KafkaService, isProdMode bool) KafkaService {
	return &ProdMode{kafkaService, isProdMode}
}

func (pm *ProdMode) DeleteTopics(ctx context.Context, searchReq *model.TopicSearchRequest) (*model.TopicDeletionResp, error) {
	if pm.isProdMode {
		return nil, utils.LogError(log, ctx, "Topic deletion is not allowed in production mode: %w", msg.BadRequest)
	}
	return pm.KafkaService.DeleteTopics(ctx, searchReq)
}

func (pm *ProdMode) DeleteTopic(ctx context.Context, topic *model.TopicRegistration, leaveRealTopicIntact bool) error {
	if pm.isProdMode {
		return utils.LogError(log, ctx, "Topic deletion is not allowed in production mode: %w", msg.BadRequest)
	}
	return pm.KafkaService.DeleteTopic(ctx, topic, leaveRealTopicIntact)
}

func (pm *ProdMode) DeleteTopicDefinition(ctx context.Context, classifier *model.Classifier) (*model.TopicDefinition, error) {
	if pm.isProdMode {
		return nil, utils.LogError(log, ctx, "Topic deletion is not allowed in production mode: %w", msg.BadRequest)
	}
	return pm.KafkaService.DeleteTopicDefinition(ctx, classifier)
}

func (pm *ProdMode) DeleteTopicTemplate(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error) {
	if pm.isProdMode {
		return nil, utils.LogError(log, ctx, "Topic deletion is not allowed in production mode: %w", msg.BadRequest)
	}
	return pm.KafkaService.DeleteTopicTemplate(ctx, name, namespace)
}
