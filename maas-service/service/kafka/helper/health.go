package helper

import (
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
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
