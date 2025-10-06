package postdeploy

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/netcracker/qubership-maas/utils"
	"path/filepath"
	"reflect"
)

type brokerInstance interface {
	GetId() string
	SetDefault(val bool)
}

type brokerInstanceService[A brokerInstance] interface {
	GetById(ctx context.Context, id string) (A, error)
	Update(ctx context.Context, instance A) (A, error)
	Register(ctx context.Context, instance A) (A, error)
	GetDefault(ctx context.Context) (A, error)
}

func processInstanceRegistrationFiles[T brokerInstance](ctx context.Context, name string, service brokerInstanceService[T]) error {
	//instancePath := fmt.Sprintf("/etc/maas/maas-instance-registrations/maas-%s-instance-body", name)
	fileName := fmt.Sprintf("maas-%s-instance-body", name)
	instancePath := filepath.Join(EtcMaaSRoot, "maas-instance-registrations", fileName)
	err := registerInstance(ctx, name, service, instancePath)
	if err != nil {
		return err
	}

	//backward compatibility, property is no longer supported
	instancePath = fmt.Sprintf("maas-%s-instance-body", name)
	return registerInstance(ctx, name, service, instancePath+"-2")
}

func registerInstance[T brokerInstance](ctx context.Context, name string, service brokerInstanceService[T], path string) error {
	return readInstance(ctx, path, func(instances []T) error {
		// we should handle default flag by special way: user can change default instance in database for instances that are not exists in secret files
		defaultInstance, err := service.GetDefault(ctx)
		if err != nil {
			return utils.LogError(log, ctx, "error getting %s instance marked as default", name)
		}
		// instance marked in database as default have higher priority than settings in secret files
		if !reflect.ValueOf(defaultInstance).IsNil() {
			for _, inst := range instances {
				inst.SetDefault(inst.GetId() == defaultInstance.GetId())
			}
		}

		for _, inst := range instances {
			instanceId := inst.GetId()
			log.InfoC(ctx, "Process %s instance setup for `%s`", name, instanceId)
			existingInstance, err := service.GetById(ctx, instanceId)
			if err != nil {
				return utils.LogError(log, ctx, "couldn't find %s instance by id: %s: %w", name, instanceId, err)
			}

			// coerce type to `comparable`
			if reflect.ValueOf(existingInstance).IsNil() {
				log.InfoC(ctx, "Register new %s instance: %v", name, instanceId)
				if _, err = service.Register(ctx, inst); err != nil {
					return utils.LogError(log, ctx, "error register %s instance: %s: %w", name, instanceId, err)
				}
			} else {
				log.InfoC(ctx, "Update existing %s instance: %v", name, instanceId)
				if _, err = service.Update(ctx, inst); err != nil {
					return utils.LogError(log, ctx, "error update %s instance: %s: %w", name, instanceId, err)
				}
			}
		}
		return nil
	})
}

func readInstance[T any](ctx context.Context, path string, processor func(body []T) error) error {
	return readFile(ctx, path, func(body []byte) error {
		// try array of T instances
		var objects []T
		err := json.Unmarshal(body, &objects)
		if err != nil {
			// try single instance for backward compatibility
			var object T
			err = json.Unmarshal(body, &object)
			if err != nil {
				return fmt.Errorf("error unmarshalling to single/multiple objects: %w", err)
			}
			objects = append(objects, object)
		}

		return processor(objects)
	})
}
