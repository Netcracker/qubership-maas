package monitoring

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/watchdog"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("audit")
}

//go:generate mockgen -source=service.go -destination=auditor_mock.go -package=monitoring
type Auditor interface {
	AddEntityRequestStat(ctx context.Context, entityType EntityType, name string, instance string)
	GetAllEntityRequestsStat(ctx context.Context) (*[]EntityRequestStatDto, error)
	GetRabbitMonitoringEntities(ctx context.Context) (*[]VhostStatLine, error)
	GetKafkaMonitoringEntities(ctx context.Context) (*[]KafkaStatLine, error)
	CleanupData(ctx context.Context, namespace string) error
}

type defaultAuditor struct {
	auditDao       Dao
	drMode         dr.Mode
	statusResolver func(instanceId string) watchdog.Status
}

func NewAuditor(auditDao Dao, drMode dr.Mode, statusResolver func(instanceId string) watchdog.Status) Auditor {
	return &defaultAuditor{
		auditDao:       auditDao,
		drMode:         drMode,
		statusResolver: statusResolver,
	}
}

func (a *defaultAuditor) AddEntityRequestStat(ctx context.Context, entityType EntityType, name string, instance string) {
	if !a.drMode.IsActive() {
		return
	}
	reqCtx := model.RequestContextOf(ctx)
	req := EntityRequestStatDto{
		Namespace:    reqCtx.Namespace,
		Microservice: reqCtx.Microservice,
		EntityType:   entityType,
		Name:         name,
		Instance:     instance,
	}
	go func() {
		// we need to execute tx in autonomous mode
		inner := context.Background()
		if err := a.auditDao.AddEntityRequestStat(inner, req.AsModel()); err != nil {
			log.Error("Error log entity request to audit: %w", err)
		}
	}()
}

func (a *defaultAuditor) GetAllEntityRequestsStat(ctx context.Context) (*[]EntityRequestStatDto, error) {
	requests, err := a.auditDao.GetAllEntityRequestsStat(ctx)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error getting stats from db: %w", err)
	}
	result := make([]EntityRequestStatDto, len(*requests))
	for i, request := range *requests {
		result[i] = request.AsDto()
	}
	return &result, nil
}

func (a *defaultAuditor) GetKafkaMonitoringEntities(ctx context.Context) (*[]KafkaStatLine, error) {
	log.InfoC(ctx, "Get kafka topics access audit info")
	list, err := a.auditDao.GetMonitoringTopics(ctx)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error get list of audit entities: %w", err)
	}

	var result []KafkaStatLine
	for _, topic := range *list {
		result = append(result, KafkaStatLine{
			Namespace:    topic.Namespace,
			BrokerHost:   topic.BrokerHost,
			Topic:        topic.Topic,
			BrokerStatus: a.statusResolver(topic.InstanceID),
		})
	}
	return &result, nil
}

func (a *defaultAuditor) GetRabbitMonitoringEntities(ctx context.Context) (*[]VhostStatLine, error) {
	log.InfoC(ctx, "Get rabbit vhost access audit info")
	list, err := a.auditDao.GetMonitoringVhosts(ctx)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error get list of audit entities: %w", err)
	}

	var result []VhostStatLine
	for _, vhost := range *list {
		result = append(result, VhostStatLine{
			Namespace:    vhost.Namespace,
			BrokerHost:   vhost.BrokerHost,
			Microservice: vhost.Microservice,
			Vhost:        vhost.Vhost,
			BrokerStatus: a.statusResolver(vhost.InstanceID),
		})
	}
	return &result, nil
}

func (a *defaultAuditor) CleanupData(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "cleanup audit data for namespace: %v", namespace)
	return a.auditDao.DeleteEntityRequestsStatByNamespace(ctx, namespace)
}
