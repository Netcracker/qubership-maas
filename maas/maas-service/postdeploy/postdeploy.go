package postdeploy

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/instance"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("postdeploy")
}

func RunPostdeployScripts(ctx context.Context, auth AuthService, rabbit instance.RabbitInstanceService, kafka instance.KafkaInstanceService) error {
	err := createManagerAccount(ctx, auth)
	if err != nil {
		return err
	}

	err = createDeployerAccount(ctx, auth)
	if err != nil {
		return err
	}

	rabbitCoer := brokerInstanceService[*model.RabbitInstance](rabbit)
	err = processInstanceRegistrationFiles(ctx, "rabbit", rabbitCoer)
	if err != nil {
		return err
	}

	kafkaCoer := brokerInstanceService[*model.KafkaInstance](kafka)
	err = processInstanceRegistrationFiles(ctx, "kafka", kafkaCoer)
	if err != nil {
		return err
	}

	return nil
}
