package cr

import (
	"context"
	"encoding/json"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/exp/slices"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/service/configurator_service"
	mockconfiguratorservice "maas/maas-service/service/configurator_service/mock"
	"maas/maas-service/service/kafka"
	mock_rabbit_service "maas/maas-service/service/rabbit_service/mock"
	"math/rand"
	"testing"
	"time"
)

func TestCustomResourceProcessorService_Create_Topic(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaConfiguration(gomock.Any(), &model.TopicRegistrationConfigReqDto{
			Pragma: nil,
			Spec: model.TopicRegistrationReqDto{
				Classifier: model.Classifier{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				ExternallyManaged: false,
				Instance:          "test-instance",
			},
		}, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)
	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
		},
	}, ActionCreate)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_Create_Topic_Unknown_Fields(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
			"wrong":    "field",
		},
	}, ActionCreate)
	assert.ErrorContains(t, err, "has invalid keys: wrong")
	assert.ErrorIs(t, err, msg.BadRequest)
}

func TestCustomResourceProcessorService_Create_Topic_With_Template(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)

	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)
	customResourceRequestTopic := CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
			"template": "test-topic-template",
		},
	}
	customResourceRequestTopicJson, err := json.Marshal(customResourceRequestTopic)
	assert.NoError(t, err)

	gomock.InOrder(
		kafkaService.EXPECT().GetTopicTemplateByNameAndNamespace(gomock.Any(), "test-topic-template", "test-namespace"),

		waitListDao.EXPECT().Create(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, customResourceWaitEntity *CustomResourceWaitEntity) {
			assert.Equal(t, CustomResourceStatusInProgress, customResourceWaitEntity.Status)
			assert.Equal(t, "test-namespace", customResourceWaitEntity.Namespace)
			assert.Equal(t, string(customResourceRequestTopicJson), customResourceWaitEntity.CustomResource)
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-topic-template", "test-namespace"),
	)

	_, err = resourceProcessorService.Apply(ctx, &customResourceRequestTopic, ActionCreate)
	assert.NoError(t, err)

	customResourceRequestTopicTemplate := CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopicTemplate,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-topic-template",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"numPartitions": 2,
		},
	}

	configReqDto := model.TopicTemplateConfigReqDto{
		Spec: model.TopicTemplateReqDto{
			Name: "test-topic-template",
		},
	}
	var two int32 = 2
	configReqDto.Spec.NumPartitions = &two

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaTopicTemplate(gomock.Any(), &configReqDto, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).DoAndReturn(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) ([]CustomResourceWaitEntity, error) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
			return []CustomResourceWaitEntity{{
				TrackingId:     42,
				CustomResource: string(customResourceRequestTopicJson),
				Namespace:      "test-namespace",
				Reason:         "",
				Status:         CustomResourceStatusInProgress,
				CreatedAt:      time.Now(),
			}}, nil
		}),

		kafkaService.EXPECT().
			GetTopicTemplateByNameAndNamespace(gomock.Any(), "test-topic-template", "test-namespace").
			Return(&model.TopicTemplate{
				Name:      "test-topic-template",
				Namespace: "test-namespace",
			}, nil),

		configuratorService.EXPECT().ApplyKafkaConfiguration(gomock.Any(), &model.TopicRegistrationConfigReqDto{
			Spec: model.TopicRegistrationReqDto{
				Classifier: model.Classifier{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				ExternallyManaged: false,
				Instance:          "test-instance",
				Template:          "test-topic-template",
			},
		}, "test-namespace"),

		waitListDao.EXPECT().UpdateStatus(gomock.Any(), gomock.Any(), gomock.Any()).Do(func(ctx context.Context, trackingId int64, status CustomResourceWaitStatus) {
			assert.Equal(t, CustomResourceStatusCompleted, status)
		}),
	)

	_, err = resourceProcessorService.Apply(ctx, &customResourceRequestTopicTemplate, ActionCreate)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_Create_Topic_Multiple_Requests(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)

	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)
	customResourceRequestTopic := CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
			"template": "test-topic-template",
		},
	}
	customResourceRequestTopicJson, err := json.Marshal(customResourceRequestTopic)
	assert.NoError(t, err)

	gomock.InOrder(
		kafkaService.EXPECT().GetTopicTemplateByNameAndNamespace(gomock.Any(), "test-topic-template", "test-namespace"),

		waitListDao.EXPECT().Create(gomock.Any(), gomock.Any()).Do(func(ctx context.Context, customResourceWaitEntity *CustomResourceWaitEntity) {
			assert.Equal(t, CustomResourceStatusInProgress, customResourceWaitEntity.Status)
			assert.Equal(t, "test-namespace", customResourceWaitEntity.Namespace)
			assert.Equal(t, string(customResourceRequestTopicJson), customResourceWaitEntity.CustomResource)
		}),
	)

	_, err = resourceProcessorService.Apply(ctx, &customResourceRequestTopic, ActionCreate)
	assert.NoError(t, err)

	customResourceRequestTopicWithoutTemplate := CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
		},
	}

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaConfiguration(gomock.Any(), &model.TopicRegistrationConfigReqDto{
			Pragma: nil,
			Spec: model.TopicRegistrationReqDto{
				Classifier: model.Classifier{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				ExternallyManaged: false,
				Instance:          "test-instance",
			},
		}, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)

	_, err = resourceProcessorService.Apply(ctx, &customResourceRequestTopicWithoutTemplate, ActionCreate)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_Create_LazyTopic(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaLazyTopic(gomock.Any(), &model.TopicRegistrationConfigReqDto{
			Pragma: nil,
			Spec: model.TopicRegistrationReqDto{
				Classifier: model.Classifier{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				ExternallyManaged: false,
				Instance:          "test-instance",
			},
		}, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)
	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindLazyTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
		},
	}, ActionCreate)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_Create_TenantTopic(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaTenantTopic(gomock.Any(), &model.TopicRegistrationConfigReqDto{
			Pragma: nil,
			Spec: model.TopicRegistrationReqDto{
				Classifier: model.Classifier{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
				ExternallyManaged: false,
				Instance:          "test-instance",
			},
		}, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)
	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTenantTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
		},
	}, ActionCreate)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_CleanupNamespace(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)

	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	waitListDao.EXPECT().DeleteByNamespace(ctx, "test-namespace")
	err := resourceProcessorService.CleanupNamespace(ctx, "test-namespace")
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_GetStatus(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)

	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	trackingId := rand.Int63()
	waitListDao.EXPECT().GetByTrackingId(ctx, trackingId).Return(&CustomResourceWaitEntity{
		Namespace: "test-namespace",
		Reason:    "test-reason",
		Status:    CustomResourceStatusInProgress,
		CreatedAt: time.Now(),
	}, nil)
	status, err := resourceProcessorService.GetStatus(ctx, trackingId)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, "test-namespace", status.Namespace)
	assert.Equal(t, "test-reason", status.Reason)
	assert.Equal(t, CustomResourceStatusInProgress, status.Status)
}

func TestCustomResourceProcessorService_Terminate(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)

	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	waitListDao.EXPECT().UpdateStatus(ctx, gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, trackingId int64, status CustomResourceWaitStatus) (*CustomResourceWaitEntity, error) {
		assert.Equal(t, CustomResourceStatusTerminated, status)
		return &CustomResourceWaitEntity{
			Namespace: "test-namespace",
			Reason:    "test-reason",
			Status:    CustomResourceStatusTerminated,
			CreatedAt: time.Now(),
		}, nil
	})

	customResourceWaitEntity, err := resourceProcessorService.Terminate(ctx, rand.Int63())
	assert.NoError(t, err)
	assert.NotNil(t, customResourceWaitEntity)
	assert.Equal(t, CustomResourceStatusTerminated, customResourceWaitEntity.Status)
}

func TestCustomResourceProcessorService_Delete_Topic(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaDeleteTopic(gomock.Any(), &model.TopicDeleteConfig{
			Spec: &model.TopicDeleteCriteria{
				Classifier: model.Classifier{
					Name:      "test-name",
					Namespace: "test-namespace",
				},
			},
		}, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)
	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopic,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"instance": "test-instance",
		},
	}, ActionDelete)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_Delete_TopicTemplate(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	gomock.InOrder(
		configuratorService.EXPECT().ApplyKafkaDeleteTopicTemplate(gomock.Any(), &model.TopicTemplateDeleteConfig{
			Spec: &model.TopicTemplateDeleteCriteria{
				Name: "test-name",
			},
		}, "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)
	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindTopicTemplate,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
	}, ActionDelete)
	assert.NoError(t, err)
}

func TestCustomResourceProcessorService_Unknown_Kind(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    "UNKNOWN-SUB-KIND",
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
	}, ActionDelete)
	assert.ErrorIs(t, err, msg.BadRequest)
	assert.ErrorContains(t, err, "unknown kind")
}

func TestCustomResourceProcessorService_Create_Vhost(t *testing.T) {
	ctx := model.WithSecurityContext(
		context.Background(),
		model.NewSecurityContext(&model.Account{
			Username:  "user",
			Namespace: "test-namespace",
		}, false),
	)
	mockCtrl := gomock.NewController(t)
	waitListDao := NewMockWaitListDao(mockCtrl)
	configuratorService := mockconfiguratorservice.NewMockConfiguratorService(mockCtrl)
	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	resourceProcessorService := NewCustomResourceProcessorService(waitListDao, configuratorService, kafkaService, rabbitService)

	gomock.InOrder(
		configuratorService.EXPECT().ApplyRabbitConfiguration(gomock.Any(), gomock.Any(), "test-namespace"),

		waitListDao.EXPECT().FindByNamespaceAndStatus(gomock.Any(), "test-namespace", gomock.Any()).Do(func(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) {
			assert.Len(t, status, 1)
			assert.True(t, slices.Contains(status, CustomResourceStatusInProgress))
		}),

		waitListDao.EXPECT().DeleteByNameAndNamespace(gomock.Any(), "test-name", "test-namespace"),
	)
	_, err := resourceProcessorService.Apply(ctx, &CustomResourceRequest{
		ApiVersion: configurator_service.CoreNcV1ApiVersion,
		Kind:       "MaaS",
		SubKind:    configurator_service.CustomResourceKindVhost,
		Metadata: &CustomResourceMetadataRequest{
			Name:      "test-name",
			Namespace: "test-namespace",
		},
		Spec: &CustomResourceSpecRequest{
			"entities": map[string]interface{}{
				"exchanges": []CustomResourceSpecRequest{
					{"name": "e1"},
				},
				"queues": []CustomResourceSpecRequest{
					{"name": "q1"},
				},
				"bindings": []CustomResourceSpecRequest{
					{"source": "e1", "destination": "q1"},
				},
			},
			"deletions": map[string]interface{}{
				"exchanges": []CustomResourceSpecRequest{
					{"name": "e2"},
				},
			},
		},
	}, ActionCreate)
	assert.NoError(t, err)
}
