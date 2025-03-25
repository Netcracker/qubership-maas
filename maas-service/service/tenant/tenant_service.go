package tenant

import (
	"context"
	"encoding/json"
	"maas/maas-service/dao"
	"maas/maas-service/model"
	"maas/maas-service/utils"
)

const TENANT_EXTERNAL_ID = "externalId"

//go:generate mockgen -source=tenant_service.go -destination=mock/tenant_service.go
type TenantServiceDao interface {
	InsertTenant(ctx context.Context, userTenant *model.Tenant) error
	GetTenantsByNamespace(ctx context.Context, namespace string) ([]model.Tenant, error)
	DeleteTenantsByNamespace(ctx context.Context, namespace string) error
}

type TenantServiceImpl struct {
	dao          TenantServiceDao
	kafkaService KafkaService
}

type KafkaService interface {
	GetTopicDefinitionsByNamespaceAndKind(ctx context.Context, namespace string, kind string) ([]model.TopicDefinition, error)
	CreateTopicByTenantTopic(ctx context.Context, topicDefinition model.TopicDefinition, tenant model.Tenant) (*model.TopicRegistrationRespDto, error)
}

func NewTenantService(dao TenantServiceDao, kafkaService KafkaService) *TenantServiceImpl {
	return &TenantServiceImpl{dao: dao, kafkaService: kafkaService}
}

func (cs *TenantServiceImpl) ApplyTenants(ctx context.Context, reqBody string) ([]model.SyncTenantsResp, error) {
	responses := make([]model.SyncTenantsResp, 0)

	tenantsFromReq, err := cs.getTenantsFromReq(ctx, reqBody)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error proceedTenantsFromReq: %w", err)
	}

	tenantsFromDb, err := cs.dao.GetTenantsByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during getTenantsByNamespace with namespace: %v, %w", model.RequestContextOf(ctx).Namespace, err)
	}
	log.InfoC(ctx, "Already stored tenants for namespace '%v': %v", model.RequestContextOf(ctx).Namespace, tenantsFromDb)

	tenantTopics, err := cs.kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, model.RequestContextOf(ctx).Namespace, model.TopicDefinitionKindTenant)
	if err != nil {
		log.ErrorC(ctx, "error during FindTopicDefinitionsByNamespaceAndKind with namespace: %v, %v", model.RequestContextOf(ctx).Namespace, err)
		return nil, err
	}

	tenantsToCreate := SubstractTenants(tenantsFromReq, tenantsFromDb)
	log.InfoC(ctx, "Tenant topics will be applies for tenants: %v", tenantsToCreate)

	return dao.WithTransactionContextValue(ctx, func(ctx context.Context) ([]model.SyncTenantsResp, error) {
		for _, createdTenant := range tenantsToCreate {
			err := cs.dao.InsertTenant(ctx, &createdTenant)
			if err != nil {
				log.ErrorC(ctx, "error while inserting tenant: %v, %v", tenantsToCreate, err)
				return responses, err
			}

			topicResponses := make([]*model.TopicRegistrationRespDto, 0)
			for _, tenantTopic := range tenantTopics {
				topicResp, err := cs.kafkaService.CreateTopicByTenantTopic(ctx, tenantTopic, createdTenant)
				if err != nil {
					log.ErrorC(ctx, "error during CreateTopicByTenantTopic: %v", err)
					return responses, err
				}

				topicResponses = append(topicResponses, topicResp)
			}
			responses = append(responses, model.SyncTenantsResp{
				Tenant: createdTenant,
				Topics: topicResponses,
			})
		}
		return responses, nil
	})
	//todo deleting tenants not yet implemented
}

// TODO why this code is not on controller side?
func (cs *TenantServiceImpl) getTenantsFromReq(ctx context.Context, reqBody string) ([]model.Tenant, error) {
	var tenantsFromReq []map[string]interface{}
	if err := json.Unmarshal([]byte(reqBody), &tenantsFromReq); err != nil {
		log.ErrorC(ctx, "Error during proceeding tenants from req: %v, err: %v", reqBody, err)
		return nil, err
	}

	var userTenants []model.Tenant
	for _, tenant := range tenantsFromReq {
		if externalId, exists := tenant[TENANT_EXTERNAL_ID]; exists {
			userTenant := model.Tenant{
				Namespace:          model.RequestContextOf(ctx).Namespace,
				ExternalId:         externalId.(string),
				TenantPresentation: tenant,
			}
			userTenants = append(userTenants, userTenant)
		}
	}

	return userTenants, nil
}

func (cs *TenantServiceImpl) GetTenantsByNamespace(ctx context.Context, namespace string) ([]model.Tenant, error) {
	tenantsFromDb, err := cs.dao.GetTenantsByNamespace(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during getTenantsByNamespace with namespace `%v': %w", namespace, err)
	}
	log.InfoC(ctx, "Tenants for namespace '%v': %v", namespace, tenantsFromDb)
	return tenantsFromDb, nil
}

func (cs *TenantServiceImpl) DeleteTenantsByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "remove tenants for namespace: %v", namespace)
	return cs.dao.DeleteTenantsByNamespace(ctx, namespace)
}

// difference returns the elements in `a` that aren't in `b`.
func SubstractTenants(a, b []model.Tenant) []model.Tenant {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x.ExternalId] = struct{}{}
	}
	var diff []model.Tenant
	for _, x := range a {
		if _, found := mb[x.ExternalId]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
