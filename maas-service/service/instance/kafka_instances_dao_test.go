package instance

import (
	"context"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestKafkaInstanceOperations(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		dao := NewKafkaInstancesDao(baseDao, domainDao)

		// test expected result for empty database
		instance, err := dao.GetDefaultInstance(ctx)
		assert.NoError(t, err)
		assert.Nil(t, instance)

		main := &model.KafkaInstance{
			Id:           "main",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {`{"addr":"main"}`}},
			Default:      false,
			MaasProtocol: "PLAINTEXT",
		}
		inserted, err := dao.InsertInstanceRegistration(ctx, main)
		assert.NoError(t, err)
		assert.True(t, inserted.Default)

		{ // test prevent inserting the same instance
			_, err := dao.InsertInstanceRegistration(ctx, main)
			assert.Error(t, err)
		}

		{ // test added instance
			inst, err := dao.GetInstanceById(ctx, "main")
			assert.NoError(t, err)
			assert.Equal(t, "main", inst.GetId())

			// this is only one registered instance, it should be set default ignoring request definition
			assert.True(t, inst.IsDefault())
		}

		// add another registration marked as new default
		backupInstance := &model.KafkaInstance{
			Id:           "backup",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {`{"addr":"backup"}`}},
			Default:      true,
			MaasProtocol: "PLAINTEXT",
		}
		_, err = dao.InsertInstanceRegistration(ctx, backupInstance)
		assert.NoError(t, err)

		{
			inst, err := dao.GetInstanceById(ctx, "main")
			assert.NoError(t, err)
			assert.Equal(t, "main", inst.GetId())
			// first server should lost it default status
			assert.False(t, inst.IsDefault())
		}

		{ // validate result for non existent instance
			inst, err := dao.GetInstanceById(ctx, "absent")
			assert.NoError(t, err)
			assert.Nil(t, inst)
		}

		{
			inst, err := dao.GetInstanceById(ctx, "backup")
			assert.NoError(t, err)
			assert.Equal(t, "backup", inst.GetId())
			// first server should lose its default status
			assert.True(t, inst.IsDefault())

			// test default instance search
			inst2, err := dao.GetDefaultInstance(ctx)
			assert.NoError(t, err)
			assert.Equal(t, inst, inst2)
		}

		{
			// test update instance
			backupInstance.Addresses["PLAINTEXT"][0] = "backup:9092"
			_, err := dao.UpdateInstanceRegistration(ctx, backupInstance)
			assert.NoError(t, err)

			inst, err := dao.GetInstanceById(ctx, "backup")
			assert.NoError(t, err)
			assert.Equal(t, backupInstance, inst)

			backupInstance.Addresses["PLAINTEXT"][0] = "backup-new:9092"
			_, err = dao.UpdateInstanceRegistration(ctx, backupInstance)
			assert.NoError(t, err)

			inst, err = dao.GetInstanceById(ctx, "backup")
			assert.NoError(t, err)
			assert.Equal(t, "backup-new:9092", backupInstance.Addresses["PLAINTEXT"][0])
		}

		{
			// test fail update instance that leads to no default instance in list
			backupInstance.Default = false
			_, err := dao.UpdateInstanceRegistration(ctx, backupInstance)
			assert.Error(t, err)
		}

		nonExistentInstance := &model.KafkaInstance{
			Id:           "non_existent_id",
			Addresses:    nil,
			Default:      false,
			MaasProtocol: "PLAINTEXT",
		}

		{
			//test fail remove non existent instance
			_, err = dao.RemoveInstanceRegistration(ctx, nonExistentInstance.Id)
			assert.Error(t, err)
		}

		{
			//test fail update registration for non existent instance
			_, err = dao.UpdateInstanceRegistration(ctx, nonExistentInstance)
			assert.Error(t, err)
		}

		{
			//test fail set default for non existent instance
			_, err = dao.SetDefaultInstance(ctx, nonExistentInstance)
			assert.Error(t, err)
		}

		// test update
		// check error delete default instance
		_, err = dao.RemoveInstanceRegistration(ctx, "backup")
		assert.Error(t, err)

		// delete non default instance
		inst, err := dao.RemoveInstanceRegistration(ctx, "main")
		assert.NoError(t, err)
		assert.Equal(t, inst.Id, main.Id)

		// because this is last instance registration, its removal should be fine
		_, err = dao.RemoveInstanceRegistration(ctx, "backup")
		assert.NoError(t, err)
	})
}

func TestKafkaInstancesDaoImpl_InsertInstanceDesignatorKafka(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		dao := NewKafkaInstancesDao(baseDao, domainDao)

		defaultInstanceId := "main"

		err := dao.InsertInstanceDesignatorKafka(ctx, &model.InstanceDesignatorKafka{
			Namespace:         "test-ns",
			DefaultInstanceId: &defaultInstanceId,
		})
		assert.ErrorContains(t, err, "check your default instance 'main'")

		main := model.KafkaInstance{
			Id:           "main",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {`{"addr":"main"}`}},
			Default:      false,
			MaasProtocol: "PLAINTEXT",
		}
		_, err = dao.InsertInstanceRegistration(ctx, &main)
		assert.NoError(t, err)

		instanceDesignatorWithSelector := model.InstanceDesignatorKafka{
			Namespace:         "test-ns",
			DefaultInstanceId: &defaultInstanceId,
			InstanceSelectors: []*model.InstanceSelectorKafka{
				{
					ClassifierMatch: model.ClassifierMatch{
						Namespace: "test-ns",
					},
					InstanceId: "second",
				},
			},
		}
		err = dao.InsertInstanceDesignatorKafka(ctx, &instanceDesignatorWithSelector)
		assert.ErrorContains(t, err, "check your selector instance: Key (instance_id)=(second) is not present")

		second := model.KafkaInstance{
			Id:           "second",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {`{"addr":"second"}`}},
			Default:      false,
			MaasProtocol: "PLAINTEXT",
		}
		_, err = dao.InsertInstanceRegistration(ctx, &second)
		assert.NoError(t, err)

		err = dao.InsertInstanceDesignatorKafka(ctx, &instanceDesignatorWithSelector)
		assert.NoError(t, err)
	})
}
