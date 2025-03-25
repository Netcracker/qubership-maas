package kafka

import (
	"maas/maas-service/model"
)

type TopicClassifierEntity struct {
	Id         uint `gorm:"primaryKey"`
	TopicId    uint
	Classifier model.Classifier `gorm:"serializer:json"`
}

func (_ TopicClassifierEntity) TableName() string {
	return "kafka_topic_classifiers"
}

type TopicDefinitionClassifierEntity struct {
	Id                uint `gorm:"primaryKey"`
	TopicDefinitionId uint
	Classifier        *model.Classifier `gorm:"serializer:json"`
}

func (_ TopicDefinitionClassifierEntity) TableName() string {
	return "kafka_topic_definitions_classifiers"
}
