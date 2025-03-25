package helper

import (
	"maas/maas-service/model"
	"maas/maas-service/utils"
)

var healthCtx = utils.CreateContextFromString("kafka_health")

const dummyNonExistenceTopic = "maas-health-topic"

type instanceHealthChecker struct {
	instance model.KafkaInstance
	helper   Helper
}

func NewInstanceHealthChecker(instance model.KafkaInstance, helper Helper) *instanceHealthChecker {
	return &instanceHealthChecker{instance: instance, helper: helper}
}

func (healthChecker instanceHealthChecker) IsAvailable() error {
	return healthChecker.helper.CheckHealth(healthCtx, &healthChecker.instance)
}
