package postdeploy

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/model"
	"maas/maas-service/service/instance"
	"os"
)

var log logging.Logger

var EtcMaaSRoot = "/etc/maas"

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

func readFile(ctx context.Context, path string, processor func(body []byte) error) error {
	log.InfoC(ctx, "Check file: `%v'", path)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.InfoC(ctx, "File `%s` is not exists, skip postdeploy action", path)
		return nil
	}
	log.InfoC(ctx, "Load file: `%v'", path)
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading `%s': %w", path, err)
	}

	if len(contents) == 0 {
		log.InfoC(ctx, "File `%s' has empty body, skip processing", path)
		return nil
	}

	return processor(contents)
}
