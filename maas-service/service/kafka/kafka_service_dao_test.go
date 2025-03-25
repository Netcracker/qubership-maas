package kafka

import (
	"context"
	"database/sql"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"maas/maas-service/dao"
	"maas/maas-service/model"
	"maas/maas-service/monitoring"
	mock_auth "maas/maas-service/service/auth/mock"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/service/instance"
	mock_instance "maas/maas-service/service/instance/mock"
	mock_kafka_helper "maas/maas-service/service/kafka/helper/mock"
	"testing"
)

func TestInsertNewRegistration(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		instance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) {
			return &domain.BGNamespaces{"controller", "primary", "secondary"}, nil
		})

		reg := model.TopicRegistration{
			Classifier:  &model.Classifier{Name: "a", Namespace: "primary"},
			Instance:    "default",
			Namespace:   "primary",
			InstanceRef: instance,
		}

		_, err = sd.InsertTopicRegistration(ctx, &reg, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
			return reg.ToResponseDto(), nil
		})

		assert.NoError(t, err)
	})
}

func TestFindTopicsBySearchRequest(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		instance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)

		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		reg1 := model.TopicRegistration{
			Topic:       "test-topic1",
			Classifier:  &model.Classifier{Name: "b1", Namespace: "primary"},
			Instance:    "default",
			Namespace:   "primary",
			InstanceRef: instance,
		}
		reg2 := model.TopicRegistration{
			Topic:       "test-topic2",
			Classifier:  &model.Classifier{Name: "b2", Namespace: "primary"},
			Instance:    "default",
			Namespace:   "primary",
			InstanceRef: instance,
		}

		_, err = sd.InsertTopicRegistration(ctx, &reg1, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
			return reg.ToResponseDto(), nil
		})
		assert.NoError(t, err)
		_, err = sd.InsertTopicRegistration(ctx, &reg2, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
			return reg.ToResponseDto(), nil
		})
		assert.NoError(t, err)

		topicByClassifier, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "b1",
				Namespace: "primary",
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, topicByClassifier)
		assert.Len(t, topicByClassifier, 1)
		assert.Equal(t, reg1.Topic, topicByClassifier[0].Topic)

		topicByClassifierAndNamespace, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "b2",
				Namespace: "primary",
			},
			Namespace: "primary",
		})
		assert.NoError(t, err)
		assert.NotNil(t, topicByClassifierAndNamespace)
		assert.Len(t, topicByClassifierAndNamespace, 1)
		assert.Equal(t, reg2.Topic, topicByClassifierAndNamespace[0].Topic)

		topicByClassifierAndNamespaceAndTopicName, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "b2",
				Namespace: "primary",
			},
			Namespace: "primary",
			Topic:     "test-topic2",
		})
		assert.NoError(t, err)
		assert.NotNil(t, topicByClassifierAndNamespaceAndTopicName)
		assert.Len(t, topicByClassifierAndNamespaceAndTopicName, 1)
		assert.Equal(t, reg2.Topic, topicByClassifierAndNamespaceAndTopicName[0].Topic)

		topicsByNamespace, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Namespace: "primary"})
		assert.NoError(t, err)
		assert.NotNil(t, topicsByNamespace)
		assert.Len(t, topicsByNamespace, 2)
	})
}

func TestWarmup(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		instance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		reg := model.TopicRegistration{
			Topic:       "test-topic",
			Classifier:  &model.Classifier{Name: "b", Namespace: "primary"},
			Instance:    "default",
			Namespace:   "primary",
			InstanceRef: instance,
		}

		_, err = sd.InsertTopicRegistration(ctx, &reg, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
			return reg.ToResponseDto(), nil
		})
		assert.NoError(t, err)

		topicByNs1, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "b",
				Namespace: "primary",
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, topicByNs1)
		assert.Len(t, topicByNs1, 1)
		assert.Equal(t, "test-topic", topicByNs1[0].Topic)

		topicByNs2, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "b",
				Namespace: "secondary",
			},
		})
		assert.NoError(t, err)
		assert.Nil(t, topicByNs2)

		topicByNs1, err = sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Namespace: "primary"})
		assert.NoError(t, err)
		assert.NotNil(t, topicByNs1)
		assert.Len(t, topicByNs1, 1)
		assert.Equal(t, "test-topic", topicByNs1[0].Topic)

		topicByNs2, err = sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Namespace: "secondary"})
		assert.NoError(t, err)
		assert.Nil(t, topicByNs2)

		err = sd.Warmup(ctx, "primary", "secondary")
		assert.NoError(t, err)

		topicByNs2, err = sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "b",
				Namespace: "secondary",
			},
		})
		assert.NoError(t, err)
		assert.NotNil(t, topicByNs2)
		assert.Len(t, topicByNs2, 1)
		assert.Equal(t, "test-topic", topicByNs2[0].Topic)

		topicByNs2, err = sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Namespace: "secondary"})
		assert.NoError(t, err)
		assert.NotNil(t, topicByNs2)
		assert.Len(t, topicByNs2, 1)
		assert.Equal(t, "test-topic", topicByNs2[0].Topic)
	})
}

func TestKafkaBg2Composite(t *testing.T) {

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = model.WithRequestContext(context.Background(), requestContext)

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		instance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		mockCtrl := gomock.NewController(t)
		instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
		instanceService.EXPECT().GetKafkaInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		instanceService.EXPECT().GetDefault(gomock.Any()).Return(instance, nil).AnyTimes()
		instanceService.EXPECT().GetById(gomock.Any(), gomock.Any()).Return(instance, nil).AnyTimes()

		kafkaHelperMock := mock_kafka_helper.NewMockHelper(mockCtrl)

		kafkaHelperMock.EXPECT().DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
		kafkaHelperMock.EXPECT().CreateTopic(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		kafkaHelperMock.EXPECT().GetTopicSettings(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		eventBus := NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).AnyTimes()
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		auditService = monitoring.NewMockAuditor(mockCtrl)
		auditService.EXPECT().AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("error")).Times(1)

		domainService := domain.NewBGDomainService(domainDao)
		kafkaService := NewKafkaService(sd, instanceService, kafkaHelperMock, auditService, domainService, eventBus, authService)

		err = domainService.Bind(ctx, "origin", "peer", "")
		assert.NoError(t, err)

		reg := model.TopicRegistration{
			Topic:       "test-topic-composite",
			Classifier:  &model.Classifier{Name: "composite", Namespace: "origin"},
			Instance:    "default",
			Namespace:   "origin",
			InstanceRef: instance,
		}

		exists, _, err := kafkaService.GetOrCreateTopic(ctx, &reg, model.Fail)
		assert.NoError(t, err)
		assert.Equal(t, false, exists)

		topic, err := kafkaService.GetTopicByClassifierWithBgDomain(ctx, model.Classifier{Name: "composite", Namespace: "origin"})
		assert.NoError(t, err)
		assert.NotNil(t, topic)

		topic, err = kafkaService.GetTopicByClassifierWithBgDomain(ctx, model.Classifier{Name: "composite", Namespace: "controller"})
		assert.NoError(t, err)
		assert.Nil(t, topic)

		err = domainService.Bind(ctx, "origin", "peer", "controller")
		assert.NoError(t, err)

		topic, err = kafkaService.GetTopicByClassifierWithBgDomain(ctx, model.Classifier{Name: "composite", Namespace: "controller"})
		assert.NoError(t, err)
		assert.NotNil(t, topic)

		regContr := model.TopicRegistration{
			Topic:       "test-topic-composite",
			Classifier:  &model.Classifier{Name: "composite", Namespace: "controller"},
			Instance:    "default",
			Namespace:   "controller",
			InstanceRef: instance,
		}

		_, topic, err = kafkaService.GetOrCreateTopic(ctx, &regContr, model.Fail)
		assert.Error(t, err)
	})
}

func TestKafkaTopicDefinitionsWarmup(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		classifier := model.Classifier{
			Name:      "first",
			Namespace: "primary",
		}
		originalTopicDefinition := model.TopicDefinition{
			Classifier: &classifier,
			Name:       "first-td",
			Kind:       model.TopicDefinitionKindTenant,
		}
		err := sd.InsertTopicDefinition(ctx, &originalTopicDefinition)
		assert.NoError(t, err)

		topicDefinitions, err := sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: "primary", Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.NotEmpty(t, topicDefinitions)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "first-td", topicDefinitions[0].Name)

		topicDefinitions, err = sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Classifier: &classifier, Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "first-td", topicDefinitions[0].Name)

		err = sd.Warmup(ctx, "primary", "secondary")
		assert.NoError(t, err)

		topicDefinitions, err = sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: "secondary", Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.NotEmpty(t, topicDefinitions)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "first-td", topicDefinitions[0].Name)
		assert.Equal(t, originalTopicDefinition, topicDefinitions[0])

		topicDefinitions, err = sd.FindTopicDefinitions(ctx,
			TopicDefinitionSearch{Classifier: &model.Classifier{Name: "first", Namespace: "secondary"}, Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "first-td", topicDefinitions[0].Name)

		// delete definition
		topicDefinition, err := sd.DeleteTopicDefinition(ctx, &classifier)
		assert.NoError(t, err)
		assert.Equal(t, &originalTopicDefinition, topicDefinition)

		// test deletion results (both classifier should be deteled
		topicDefinitions, err = sd.FindTopicDefinitions(ctx,
			TopicDefinitionSearch{Classifier: &model.Classifier{Name: "first", Namespace: "secondary"}, Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 0)
	})
}

func TestKafkaDaoImpl_InsertTopicTemplate(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		err = sd.InsertTopicTemplate(ctx, &model.TopicTemplate{
			Name:      "test-primary",
			Namespace: "primary",
		})
		assert.NoError(t, err)

		template, err := sd.FindTopicTemplateByNameAndNamespace(ctx, "test-primary", "primary")
		assert.NoError(t, err)
		assert.NotNil(t, template)
		assert.Equal(t, "test-primary", template.Name)
		assert.Equal(t, "primary", template.Namespace)
		assert.Contains(t, template.DomainNamespaces, "primary")

		err = bgDomainService.Bind(ctx, "primary", "secondary", "controller")
		assert.NoError(t, err)

		err = sd.InsertTopicTemplate(ctx, &model.TopicTemplate{
			Name:      "test-secondary",
			Namespace: "secondary",
		})
		assert.NoError(t, err)

		template, err = sd.FindTopicTemplateByNameAndNamespace(ctx, "test-secondary", "secondary")
		assert.NoError(t, err)
		assert.NotNil(t, template)
		assert.Equal(t, "test-secondary", template.Name)
		assert.Equal(t, "primary", template.Namespace)
		assert.Contains(t, template.DomainNamespaces, "primary")
		assert.Contains(t, template.DomainNamespaces, "secondary")
	})
}

func TestKafkaDaoImpl_UpdateTopicTemplate(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		topicTemplate := &model.TopicTemplate{
			Name:      "name",
			Namespace: "test-namespace",
		}

		err = sd.InsertTopicTemplate(ctx, topicTemplate)
		assert.NoError(t, err)

		template, err := sd.FindTopicTemplateByNameAndNamespace(ctx, "name", "test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, template)
		assert.Equal(t, "name", template.Name)
		assert.Nil(t, template.Configs["test-prop"])

		topicTemplate.Name = "new-name"
		propVal := "test-prop-val"
		topicTemplate.Configs = map[string]*string{"test-prop": &propVal}
		err = sd.UpdateTopicTemplate(ctx, topicTemplate)
		assert.NoError(t, err)

		template, err = sd.FindTopicTemplateByNameAndNamespace(ctx, "new-name", "test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, template)
		assert.Equal(t, "new-name", template.Name)
		assert.Equal(t, propVal, *template.Configs["test-prop"])
	})
}

func TestKafkaDaoImpl_FindAllTopicTemplatesByNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		tt1 := model.TopicTemplate{
			Name:      "name-1",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &tt1)
		assert.NoError(t, err)

		tt2 := model.TopicTemplate{
			Name:      "name-2",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &tt2)
		assert.NoError(t, err)

		tt3 := model.TopicTemplate{
			Name:      "name-1",
			Namespace: "second-test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &tt3)
		assert.NoError(t, err)

		templates, err := sd.FindAllTopicTemplatesByNamespace(ctx, "test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Len(t, templates, 2)
		assert.Contains(t, templates, tt1)
		assert.Contains(t, templates, tt2)

		templates, err = sd.FindAllTopicTemplatesByNamespace(ctx, "second-test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Len(t, templates, 1)
		assert.Contains(t, templates, tt3)
	})
}

func TestKafkaDaoImpl_DeleteTopicTemplate(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instance, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		topicTemplate := model.TopicTemplate{
			Name:      "name",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &topicTemplate)
		assert.NoError(t, err)

		topicRegistration := model.TopicRegistration{
			Classifier: &model.Classifier{Name: "a", Namespace: "primary"},
			Instance:   "default",
			Namespace:  "primary",
			Template: sql.NullInt64{
				Int64: int64(topicTemplate.Id),
				Valid: true,
			},
			InstanceRef: instance,
		}
		_, err = sd.InsertTopicRegistration(ctx, &topicRegistration, nil)
		assert.NoError(t, err)

		err = sd.DeleteTopicTemplate(ctx, &topicTemplate)
		assert.ErrorContains(t, err, "kafka topic template is used by one or more topics")
		err = sd.DeleteTopicRegistration(ctx, &topicRegistration, nil)
		assert.NoError(t, err)

		err = sd.DeleteTopicTemplate(ctx, &topicTemplate)
		assert.NoError(t, err)
	})
}

func TestKafkaDaoImpl_DeleteTopicTemplatesByNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		tt1 := model.TopicTemplate{
			Name:      "name-1",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &tt1)
		assert.NoError(t, err)

		tt2 := model.TopicTemplate{
			Name:      "name-2",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &tt2)
		assert.NoError(t, err)

		tt3 := model.TopicTemplate{
			Name:      "name-1",
			Namespace: "second-test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &tt3)
		assert.NoError(t, err)

		templates, err := sd.FindAllTopicTemplatesByNamespace(ctx, "test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Len(t, templates, 2)
		assert.Contains(t, templates, tt1)
		assert.Contains(t, templates, tt2)

		templates, err = sd.FindAllTopicTemplatesByNamespace(ctx, "second-test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Len(t, templates, 1)
		assert.Contains(t, templates, tt3)

		err = sd.DeleteTopicTemplatesByNamespace(ctx, "test-namespace")
		assert.NoError(t, err)

		templates, err = sd.FindAllTopicTemplatesByNamespace(ctx, "test-namespace")
		assert.NoError(t, err)
		assert.Empty(t, templates)

		templates, err = sd.FindAllTopicTemplatesByNamespace(ctx, "second-test-namespace")
		assert.NoError(t, err)
		assert.NotNil(t, templates)
		assert.Len(t, templates, 1)
		assert.Contains(t, templates, tt3)
	})
}

func TestKafkaDaoImpl_DeleteTopicDefinitionsByNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		err = sd.InsertTopicDefinition(ctx, &model.TopicDefinition{
			Classifier: &model.Classifier{
				Name:      "first",
				Namespace: "test-namespace",
			},
			Namespace: "test-namespace",
			Name:      "first-td",
			Kind:      model.TopicDefinitionKindTenant,
		})
		assert.NoError(t, err)

		err = sd.InsertTopicDefinition(ctx, &model.TopicDefinition{
			Classifier: &model.Classifier{
				Name:      "second",
				Namespace: "test-namespace",
			},
			Namespace: "test-namespace",
			Name:      "second-td",
			Kind:      model.TopicDefinitionKindTenant,
		})
		assert.NoError(t, err)

		err = sd.InsertTopicDefinition(ctx, &model.TopicDefinition{
			Classifier: &model.Classifier{
				Name:      "first",
				Namespace: "new-test-namespace",
			},
			Namespace: "new-test-namespace",
			Name:      "first-td",
			Kind:      model.TopicDefinitionKindTenant,
		})
		assert.NoError(t, err)

		definitions, err := sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: "test-namespace", Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.NotNil(t, definitions)
		assert.Len(t, definitions, 2)

		definitions, err = sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: "new-test-namespace", Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.NotNil(t, definitions)
		assert.Len(t, definitions, 1)

		err = sd.DeleteTopicDefinitionsByNamespace(ctx, "test-namespace")
		assert.NoError(t, err)

		definitions, err = sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: "test-namespace", Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.Empty(t, definitions)

		definitions, err = sd.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: "new-test-namespace", Kind: model.TopicDefinitionKindTenant})
		assert.NoError(t, err)
		assert.NotEmpty(t, definitions)
		assert.Len(t, definitions, 1)
	})
}

func TestKafkaDaoImpl_FindTopicTemplateById(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		topicTemplate := model.TopicTemplate{
			Name:      "first",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &topicTemplate)
		assert.NoError(t, err)

		err = sd.InsertTopicTemplate(ctx, &model.TopicTemplate{
			Name:      "second",
			Namespace: "test-namespace",
		})
		assert.NoError(t, err)

		templateById, err := sd.FindTopicTemplateById(ctx, int64(topicTemplate.Id))
		assert.NoError(t, err)
		assert.NotNil(t, templateById)
		assert.Equal(t, "first", templateById.Name)
		assert.Equal(t, "test-namespace", templateById.Namespace)

		templateById, err = sd.FindTopicTemplateById(ctx, int64(42))
		assert.Error(t, err)
	})
}

func TestKafkaDaoImpl_FindTopicsByTemplate(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instance, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)

		topicTemplateFirst := model.TopicTemplate{
			Name:      "first",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &topicTemplateFirst)
		assert.NoError(t, err)

		topicTemplateSecond := model.TopicTemplate{
			Name:      "second",
			Namespace: "test-namespace",
		}
		err = sd.InsertTopicTemplate(ctx, &topicTemplateSecond)
		assert.NoError(t, err)

		topicRegistrationFirst := model.TopicRegistration{
			Classifier: &model.Classifier{Name: "a", Namespace: "primary"},
			Topic:      "a",
			Instance:   "default",
			Namespace:  "primary",
			Template: sql.NullInt64{
				Int64: int64(topicTemplateFirst.Id),
				Valid: true,
			},
			InstanceRef: instance,
		}
		_, err = sd.InsertTopicRegistration(ctx, &topicRegistrationFirst, nil)
		assert.NoError(t, err)

		topicRegistrationSecond := model.TopicRegistration{
			Classifier: &model.Classifier{Name: "b", Namespace: "primary"},
			Topic:      "b",
			Instance:   "default",
			Namespace:  "primary",
			Template: sql.NullInt64{
				Int64: int64(topicTemplateFirst.Id),
				Valid: true,
			},
			InstanceRef: instance,
		}
		_, err = sd.InsertTopicRegistration(ctx, &topicRegistrationSecond, nil)
		assert.NoError(t, err)

		topicRegistrationThird := model.TopicRegistration{
			Classifier: &model.Classifier{Name: "c", Namespace: "primary"},
			Topic:      "c",
			Instance:   "default",
			Namespace:  "primary",
			Template: sql.NullInt64{
				Int64: int64(topicRegistrationSecond.Id),
				Valid: true,
			},
			InstanceRef: instance,
		}
		_, err = sd.InsertTopicRegistration(ctx, &topicRegistrationThird, nil)
		assert.NoError(t, err)

		topics, err := sd.FindTopicsByTemplate(ctx, &topicTemplateFirst)
		assert.NoError(t, err)
		assert.NotNil(t, topics)
		assert.Len(t, topics, 2)
		assert.Equal(t, topicRegistrationFirst.Topic, topics[0].Topic)
		assert.Equal(t, topicRegistrationSecond.Topic, topics[1].Topic)
	})
}
