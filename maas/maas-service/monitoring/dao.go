package monitoring

import (
	"context"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

// FIXME generated mock move to package
//
//go:generate mockgen -source=dao.go -destination=dao_mock.go -package=monitoring
type Dao interface {
	AddEntityRequestStat(ctx context.Context, entityRequest *EntityRequestStat) error
	GetAllEntityRequestsStat(ctx context.Context) (*[]EntityRequestStat, error)
	DeleteEntityRequestsStatByNamespace(ctx context.Context, namespace string) error

	GetMonitoringVhosts(ctx context.Context) (*[]MonitoringVhost, error)
	GetMonitoringTopics(ctx context.Context) (*[]MonitoringTopic, error)
}

type DaoImpl struct {
	dao.BaseDao
}

func NewDao(baseDao dao.BaseDao) Dao {
	return &DaoImpl{baseDao}
}

func (d *DaoImpl) GetAllEntityRequestsStat(ctx context.Context) (*[]EntityRequestStat, error) {
	log.InfoC(ctx, "Getting all entity requests...")
	entityRequests := make([]EntityRequestStat, 0)
	if err := d.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		result := cnn.Find(&entityRequests)
		log.InfoC(ctx, "Found %d entity requests", result.RowsAffected)
		return result.Error
	}); err != nil {
		return nil, err
	}
	return &entityRequests, nil
}

func (d *DaoImpl) AddEntityRequestStat(ctx context.Context, entityRequest *EntityRequestStat) error {
	log.InfoC(ctx, "Adding entity request...")
	if err := d.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		result := cnn.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "namespace"}, {Name: "microservice"}, {Name: "entity_type"}, {Name: "name"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"requests_total":  gorm.Expr("entity_request_stat.requests_total + 1"),
				"last_request_ts": time.Now(),
			}),
		}).Create(entityRequest)

		return result.Error
	}); err != nil {
		return err
	}
	return nil
}

func (d *DaoImpl) DeleteEntityRequestsStatByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Deleting all entity requests for '%s' namespace", namespace)
	if err := d.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		result := cnn.Where("namespace = ?", namespace).Delete(&EntityRequestStat{})
		log.InfoC(ctx, "Deleted %d entity requests", result.RowsAffected)
		return result.Error
	}); err != nil {
		return err
	}
	return nil
}

func (d *DaoImpl) GetMonitoringVhosts(ctx context.Context) (*[]MonitoringVhost, error) {
	log.InfoC(ctx, "Get monitoring vhosts")
	var vhosts []MonitoringVhost
	if err := d.UsingDb(ctx, func(cnn *gorm.DB) error {
		//get all vhosts and add ms name for every ms if exists for vhost. distinct for same ms names with different versions
		return cnn.Raw(
			`SELECT v.namespace, c.ms_name as "microservice", v.vhost, v.instance as "instance_id", amqp_url as "broker_host"
					FROM rabbit_vhosts v
					LEFT JOIN (SELECT DISTINCT ms_name, vhost_id from rabbit_ms_configs ) AS c on v.id = c.vhost_id
					LEFT JOIN rabbit_instances i on v.instance = i.id
					`).Scan(&vhosts).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "Error during GetMonitoringVhosts in db: %w", err)
	}

	return &vhosts, nil
}

func (d *DaoImpl) GetMonitoringTopics(ctx context.Context) (*[]MonitoringTopic, error) {
	log.InfoC(ctx, "Get monitoring topics")
	var topics []MonitoringTopic
	if err := d.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Raw(
			`SELECT  t.namespace, t.topic, t.instance as "instance_id", addresses as "broker_host"
					FROM kafka_topics t 
					LEFT JOIN kafka_instances i on t.instance = i.id
					`).Scan(&topics).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "Error during GetMonitoringTopics in db: %w", err)
	}
	return &topics, nil
}
