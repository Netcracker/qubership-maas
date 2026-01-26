package rabbit_service

import (
	"context"
	"errors"
	"fmt"

	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=rabbit_service_dao.go -destination=mock/rabbit_service_dao.go
type RabbitServiceDao interface {
	FindVhostWithSearchForm(ctx context.Context, search *model.SearchForm) ([]model.VHostRegistration, error)
	FindVhostByClassifier(ctx context.Context, classifier model.Classifier) (*model.VHostRegistration, error)
	FindVhostsByNamespace(ctx context.Context, namespace string) ([]model.VHostRegistration, error)
	InsertVhostRegistration(ctx context.Context, reg *model.VHostRegistration, ext func(reg *model.VHostRegistration) error) error
	UpdateVhostRegistration(ctx context.Context, reg *model.VHostRegistration, ext func(reg *model.VHostRegistration) error) error
	DeleteVhostRegistration(ctx context.Context, registration *model.VHostRegistration) error

	InsertRabbitMsConfig(ctx context.Context, msConfig *model.MsConfig) error
	InsertRabbitVersionedEntity(ctx context.Context, entity *model.RabbitVersionedEntity) error
	GetMsConfigsInActiveButNotInCandidateByVhost(ctx context.Context, vhostId int, activeVersion string, candidateVersion string) ([]model.MsConfig, error)
	GetMsConfigByVhostAndMsNameAndCandidateVersion(ctx context.Context, vhostId int, msName string, version string) (*model.MsConfig, error)
	GetMsConfigsByBgDomainAndCandidateVersion(ctx context.Context, namespace string, version string) ([]model.MsConfig, error)
	GetMsConfigsByNamespace(ctx context.Context, namespace string) ([]model.MsConfig, error)
	UpdateMsConfigActualVersion(ctx context.Context, msConfig model.MsConfig) error
	UpdateRabbitVersionedEntity(ctx context.Context, ent model.RabbitVersionedEntity) error
	GetRabbitVersEntitiesByVhostAndNameAndType(ctx context.Context, vhostId int, name string, entType string) ([]model.RabbitVersionedEntity, error)
	GetRabbitVersEntitiesByVhostAndNameAndTypeAndActualVersion(ctx context.Context, vhostId int, name string, entType string, actVersion string) ([]model.RabbitVersionedEntity, error)
	GetRabbitVersEntitiesByMsConfigIdAndType(ctx context.Context, msConfigId int, entType model.RabbitEntityType) ([]model.RabbitVersionedEntity, error)
	GetRabbitVersEntitiesByVhost(ctx context.Context, vhostId int) ([]model.RabbitVersionedEntity, error)
	GetRabbitLazyBindings(ctx context.Context, namespace string) ([]model.LazyBindingDto, error)
	DeleteAllVersEntitiesByMsConfigId(ctx context.Context, msConfigId int) error
	DeleteRabbitVersEntityFromDB(ctx context.Context, entity *model.RabbitVersionedEntity) error
	DeleteMsConfig(ctx context.Context, msConfig model.MsConfig) error
	DeleteMsConfigsByNamespace(ctx context.Context, namespace string) error

	GetRabbitEntitiesByVhostId(ctx context.Context, vhostId int) ([]model.RabbitEntity, error)
	UpsertNamedRabbitEntity(ctx context.Context, entity *model.RabbitEntity) error
	DeleteNamedRabbitEntity(ctx context.Context, entity *model.RabbitEntity) error
	InsertLazyBinding(ctx context.Context, binding *model.RabbitEntity) error
	UpdateLazyBinding(ctx context.Context, binding *model.RabbitEntity) error
	GetNotCreatedLazyBindingsByClassifier(ctx context.Context, classifier string) ([]model.RabbitEntity, error)
	DeleteLazyBindingBySourceAndDestinationAndClassifier(ctx context.Context, classifier string, entity interface{}) error

	CheckExchangesAndQueuesHaveUniqueName(ctx context.Context, vhostId int, candidateVersion string) error
	CheckNoQueuesWithoutBinding(ctx context.Context, vhostId int, candidateVersion string) error
	CheckBindingsHaveExistingESource(ctx context.Context, vhostId int, candidateVersion string) ([]model.RabbitVersionedEntity, error)
	CheckBindingsHaveExistingQDestination(ctx context.Context, vhostId int, candidateVersion string) error
	CheckBindingDeclaredInSameMsAsQ(ctx context.Context, vhostId int, candidateVersion string) error

	WithLock(ctx context.Context, lockId string, f func(ctx context.Context) error) error
}

type RabbitServiceDaoImpl struct {
	base           dao.BaseDao
	domainSupplier func(context.Context, string) (*domain.BGNamespaces, error)
}

func NewRabbitServiceDao(base dao.BaseDao, domainSupplier func(context.Context, string) (*domain.BGNamespaces, error)) RabbitServiceDao {
	return &RabbitServiceDaoImpl{base: base, domainSupplier: domainSupplier}
}

func (d *RabbitServiceDaoImpl) WithLock(ctx context.Context, lockId string, f func(ctx context.Context) error) error {
	return d.base.WithLock(ctx, lockId, f)
}

func (d *RabbitServiceDaoImpl) FindVhostWithSearchForm(ctx context.Context, search *model.SearchForm) ([]model.VHostRegistration, error) {
	log.InfoC(ctx, "Query vhost registrations by search form %+v", search)
	vhosts := new([]model.VHostRegistration)

	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		query := cnn.Table(model.VHostRegistration{}.TableName() + " AS rv").Distinct("rv.*")
		if !search.Classifier.IsEmpty() {
			query = query.Where("rv.classifier = ?", search.Classifier)
		}
		if search.Vhost != "" {
			query = query.Where("vhost=?", search.Vhost)
		}
		if search.User != "" {
			query = query.Where("user=?", search.User)
		}
		if search.Instance != "" {
			query = query.Where("instance=?", search.Instance)
		}
		if search.Namespace != "" {
			query = query.Where("rv.namespace = ?", search.Namespace)
		}

		return query.Find(vhosts).Error
	}); err != nil {
		log.ErrorC(ctx, "Error query by search form %+v, error: %s", search, err.Error())
		return nil, err
	}
	if len(*vhosts) == 0 {
		log.Info("Virtual hosts by search form '%+v' do not exist", search)
		return nil, nil
	}

	if !search.Classifier.IsEmpty() {
		if len(*vhosts) != 1 {
			return nil, utils.LogError(log, ctx, "conflict during FindVhostWithSearchForm: classifier is search form is not nil, so num of vhosts should be 1: %w", msg.Conflict)
		}
		(*vhosts)[0].Classifier = search.Classifier.ToJsonString()
	}

	log.InfoC(ctx, "Virtual hosts by search form %+v were found. Count: %d", search, len(*vhosts))
	return *vhosts, nil
}

func (d *RabbitServiceDaoImpl) FindVhostByClassifier(ctx context.Context, classifier model.Classifier) (*model.VHostRegistration, error) {
	log.InfoC(ctx, "FindVhostByClassifier in DB by classifier: '%v'", classifier)

	vhosts, err := d.FindVhostWithSearchForm(ctx, &model.SearchForm{
		Classifier: classifier,
	})

	if err != nil {
		return nil, utils.LogError(log, ctx, "error during FindVhostWithSearchForm while in FindVhostByClassifier: %w", err)
	}

	if vhosts == nil {
		return nil, nil
	}

	return &(vhosts)[0], nil
}

func (d *RabbitServiceDaoImpl) FindVhostsByNamespace(ctx context.Context, namespace string) ([]model.VHostRegistration, error) {
	log.InfoC(ctx, "FindRabbitVhostsByNamespace in DB by namespace: '%v'", namespace)
	if namespace == "" {
		return nil, utils.LogError(log, ctx, "namespace is empty in search criteria: %w", msg.BadRequest)
	}

	vhosts, err := d.FindVhostWithSearchForm(ctx, &model.SearchForm{
		Namespace: namespace,
	})

	if err != nil {
		return nil, utils.LogError(log, ctx, "error during FindVhostWithSearchForm while in FindRabbitVhostsByNamespace: %w", err)
	}

	return vhosts, nil
}

// insert new registration into database and run `ext' code inside one transaction
func (d *RabbitServiceDaoImpl) InsertVhostRegistration(ctx context.Context, reg *model.VHostRegistration, ext func(reg *model.VHostRegistration) error) error {
	log.InfoC(ctx, "Insert registration: %+v", reg)

	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Create(reg).Error; e != nil {
			// We have a unique index on classifier column (maas/maas-service/rabbitDao/3_rabbit.go), so if there was uniq integrity violation we should return a specific error
			if d.base.IsUniqIntegrityViolation(e) {
				return utils.LogError(log, ctx, "vhost with classifier %+v already registered: %w", reg.Classifier, msg.Conflict)
			}
			return e
		}

		return ext(reg)
	}); err != nil {
		return utils.LogError(log, ctx, "error insert registration to db: %w", err)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) UpdateVhostRegistration(ctx context.Context, reg *model.VHostRegistration, ext func(reg *model.VHostRegistration) error) error {
	log.InfoC(ctx, "Update registration in db: %+v", reg)

	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Save(reg).Error; e != nil {
			return e
		}

		return ext(reg)
	}); err != nil {
		return utils.LogError(log, ctx, "error update registration in db: %w", err)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) DeleteVhostRegistration(ctx context.Context, reg *model.VHostRegistration) error {
	log.InfoC(ctx, "Delete registration by vhost: %+v", reg)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Delete(reg).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error delete registration from db: %w", err)
	}
	return nil
}

//non-vers rabbit entities

func (d *RabbitServiceDaoImpl) GetRabbitEntitiesByVhostId(ctx context.Context, vhostId int) ([]model.RabbitEntity, error) {
	log.InfoC(ctx, "GetRabbitEntitiesByVhosId in rabbitDao for vhostId '%v'", vhostId)
	var rabbitEntities []model.RabbitEntity
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.
			Where("vhost_id=?", vhostId).
			Find(&rabbitEntities).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error during GetRabbitEntitiesByVhostId: %w", err)
	}
	return rabbitEntities, nil
}

func (d *RabbitServiceDaoImpl) InsertLazyBinding(ctx context.Context, binding *model.RabbitEntity) error {
	log.InfoC(ctx, "Inserting rabbit lazy binding: %+v", binding)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Clauses(clause.OnConflict{DoNothing: true}).Create(&binding).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error insert lazy binding to db: %w", err)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) UpdateLazyBinding(ctx context.Context, binding *model.RabbitEntity) error {
	log.InfoC(ctx, "Updating rabbit lazy binding: %+v", binding)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Model(&binding).Update("rabbit_entity", binding.RabbitEntity).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error updating lazy binding to db: %w", err)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) UpsertNamedRabbitEntity(ctx context.Context, entity *model.RabbitEntity) error {
	log.InfoC(ctx, "Upsert rabbit entity in db: %+v", entity)

	err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		var ent model.RabbitEntity

		result := cnn.
			Where("vhost_id=?", entity.VhostId).
			Where("client_entity=?", entity.ClientEntity).
			Find(&ent)

		if result.Error != nil {
			return utils.LogError(log, ctx, "error checking entity before upserting: %w", result.Error)
		}
		if result.RowsAffected == 1 {
			return nil
		}

		return cnn.Clauses(
			clause.OnConflict{
				Columns:   []clause.Column{{Name: "vhost_id"}, {Name: "entity_type"}, {Name: "entity_name"}},
				DoUpdates: clause.AssignmentColumns([]string{"client_entity", "rabbit_entity"}),
			}).Create(&entity).Error
	})

	if err != nil {
		return utils.LogError(log, ctx, "error upserting rabbit entity: %w", err)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) DeleteNamedRabbitEntity(ctx context.Context, entity *model.RabbitEntity) error {
	log.InfoC(ctx, "Delete rabbit entity: %+v", entity)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {

		var foreignEnt model.RabbitEntity

		if entity.EntityType == model.ExchangeType.String() {
			if e := cnn.
				Where("classifier=?", entity.Classifier).
				Where("entity_type=?", "binding").
				Where("binding_source=?", entity.EntityName).
				Delete(&foreignEnt).Error; e != nil {
				return e
			}
		}

		if entity.EntityType == model.QueueType.String() {
			if e := cnn.
				Where("classifier=?", entity.Classifier).
				Where("entity_type=?", "binding").
				Where("binding_destination=?", entity.EntityName).
				Delete(&foreignEnt).Error; e != nil {
				return e
			}
		}

		if e := cnn.
			Where("classifier=?", entity.Classifier).
			Where("entity_name=?", entity.EntityName).
			Where("entity_type=?", entity.EntityType).
			Delete(&entity).Error; e != nil {
			return e
		}
		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error deleting rabbit entity in db: %+v", err.Error())
		return err
	}

	return nil
}

func (d *RabbitServiceDaoImpl) GetNotCreatedLazyBindingsByClassifier(ctx context.Context, classifier string) ([]model.RabbitEntity, error) {
	log.InfoC(ctx, "DAO request to GetNotCreatedLazyBindingsByClassifier for classifier '%v'", classifier)
	var lazyBindings []model.RabbitEntity
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.
			Where("classifier=?", classifier).
			Where("entity_type=?", "binding").
			Where("rabbit_entity IS NULL").
			Find(&lazyBindings).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during GetNotCreatedLazyBindingsByClassifier in db: %v", err.Error())
		return nil, err
	}

	if len(lazyBindings) == 0 {
		lazyBindings = nil
	}

	return lazyBindings, nil
}

func (d *RabbitServiceDaoImpl) DeleteLazyBindingBySourceAndDestinationAndClassifier(ctx context.Context, classifier string, entity interface{}) error {
	log.InfoC(ctx, "DAO request to DeleteLazyBindingBySourceAndDestinationAndClassifier for binding '%+v'", entity)

	source, destination, err := utils.ExtractSourceAndDestination(entity)
	if err != nil {
		log.ErrorC(ctx, "binding entity '%v' doesn't have source or destination field while in DeleteLazyBindingBySourceAndDestinationAndClassifier", entity)
		return err
	}

	lazyBinding := model.RabbitEntity{
		Classifier: classifier,
	}

	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.
			Where("classifier=?", classifier).
			Where("entity_type=?", "binding").
			Where("binding_source=?", source).
			Where("binding_destination=?", destination).
			Delete(&lazyBinding).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during DeleteLazyBindingBySourceAndDestinationAndClassifier from db: %v", err.Error())
		return err
	}
	return nil
}

//msconfigs

func (d *RabbitServiceDaoImpl) InsertRabbitMsConfig(ctx context.Context, msConfig *model.MsConfig) error {
	log.InfoC(ctx, "Inserting ms_config: %+v", msConfig)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Omit(clause.Associations).Create(msConfig).Error
	}); err != nil {
		log.ErrorC(ctx, "Error insert ms_config to db: %v", err.Error())
		return err
	}

	return nil
}

func (d *RabbitServiceDaoImpl) InsertRabbitVersionedEntity(ctx context.Context, entity *model.RabbitVersionedEntity) error {
	log.InfoC(ctx, "Inserting rabbit versioned entity: %+v", entity)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Omit(clause.Associations).Create(entity).Error
	}); err != nil {
		log.ErrorC(ctx, "Error insert entity to db: %+v", err.Error())
		return err
	}

	return nil
}

func (d *RabbitServiceDaoImpl) GetMsConfigsInActiveButNotInCandidateByVhost(ctx context.Context, vhostId int, activeVersion string, candidateVersion string) ([]model.MsConfig, error) {
	log.InfoC(ctx, "DAO request to GetMsConfigsInActiveButNotInCandidateByVhost for vhostId '%v', activeVersion '%v', candidateVersion '%v'", vhostId, activeVersion, candidateVersion)
	msNames := new([]string)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		err := cnn.Raw(
			`select ms_name from rabbit_ms_configs msName where msName.vhost_id = ? and msName.candidate_version = ?
					except
					select ms_name from rabbit_ms_configs msName where msName.vhost_id = ? and msName.candidate_version = ?`,
			vhostId, activeVersion, vhostId, candidateVersion).Scan(msNames).Error
		return err
	}); err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigsInActiveButNotInCandidateByVhost in db: %v", err.Error())
		return nil, err
	}

	var mss []model.MsConfig
	for _, msName := range *msNames {
		msConfig, err := d.GetMsConfigByVhostAndMsNameAndCandidateVersion(ctx, vhostId, msName, activeVersion)
		if err != nil {
			log.ErrorC(ctx, "Error during GetMsConfigByVhostAndMsNameAndCandidateVersion in db: %v", err.Error())
			return nil, err
		}
		mss = append(mss, *msConfig)
	}
	return mss, nil
}

func (d *RabbitServiceDaoImpl) GetMsConfigByVhostAndMsNameAndCandidateVersion(ctx context.Context, vhostId int, msName string, candidateVersion string) (*model.MsConfig, error) {
	log.InfoC(ctx, "DAO request to GetMsConfigByVhostAndMsNameAndCandidateVersion for vhostId '%v', msName '%v', candidateVersion '%v'", vhostId, msName, candidateVersion)
	var msConfig model.MsConfig
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.
			Where("vhost_id=?", vhostId).
			Where("ms_name=?", msName).
			Where("candidate_version=?", candidateVersion).
			Preload("Vhost").
			Preload("Entities").
			First(&msConfig).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		log.ErrorC(ctx, "Error during GetMsConfigByVhostAndMsNameAndCandidateVersion in db: %v", err.Error())
		return nil, err
	}
	return &msConfig, nil
}

func (d *RabbitServiceDaoImpl) GetMsConfigsByBgDomainAndCandidateVersion(ctx context.Context, namespace string, candidateVersion string) ([]model.MsConfig, error) {
	log.InfoC(ctx, "DAO request to GetMsConfigsByBgDomainAndCandidateVersion for namespace '%v', candidateVersion '%v'", namespace, candidateVersion)
	var msConfigs []model.MsConfig

	namespaces, err := d.domainSupplier(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during domainSupplier while in GetMsConfigsByBgDomainAndCandidateVersion: %v", err.Error())
		return nil, err
	}

	err = d.base.UsingDb(ctx, func(cnn *gorm.DB) error {

		query := cnn.
			Where("candidate_version=?", candidateVersion).
			Preload("Vhost").
			Preload("Entities")

		if namespaces != nil {
			query.Where("namespace=? OR namespace=?", namespaces.Origin, namespaces.Peer)
		} else {
			query.Where("namespace=?", namespace)
		}

		return query.Find(&msConfigs).Error
	})

	if err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigByVhostAndMsNameAndCandidateVersion in db: %v", err.Error())
		return nil, err
	}

	if len(msConfigs) == 0 {
		msConfigs = nil
	}

	return msConfigs, nil
}

func (d *RabbitServiceDaoImpl) GetMsConfigsByNamespace(ctx context.Context, namespace string) ([]model.MsConfig, error) {
	log.InfoC(ctx, "DAO request to GetMsConfigsByNamespace for namespace '%v'", namespace)
	var msConfigs []model.MsConfig

	namespaces, err := d.domainSupplier(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during domainSupplier while in GetMsConfigsByNamespace: %v", err.Error())
		return nil, err
	}

	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		query := cnn.
			Preload("Vhost").
			Preload("Entities")

		if namespaces != nil {
			query.Where("namespace=? OR namespace=?", namespaces.Origin, namespaces.Peer)
		} else {
			query.Where("namespace=?", namespace)
		}

		return query.Find(&msConfigs).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigsByNamespace in db: %v", err.Error())
		return nil, err
	}

	if len(msConfigs) == 0 {
		msConfigs = nil
	}

	return msConfigs, nil
}

func (d *RabbitServiceDaoImpl) UpdateMsConfigActualVersion(ctx context.Context, msConfig model.MsConfig) error {
	log.InfoC(ctx, "DAO request to UpdateMsConfigActualVersion for msConfig with name '%v'", msConfig.MsName)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Omit(clause.Associations).Save(&msConfig).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during UpdateMsConfigActualVersion in db: %v", err.Error())
		return nil
	}
	return nil
}

func (d *RabbitServiceDaoImpl) UpdateRabbitVersionedEntity(ctx context.Context, ent model.RabbitVersionedEntity) error {
	log.InfoC(ctx, "DAO request to UpdateRabbitVersionedEntity for entity with type '%v' and name '%v'", ent.EntityType, ent.EntityName)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Omit(clause.Associations).Save(&ent).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during UpdateRabbitVersionedEntity in db: %v", err.Error())
		return nil
	}
	return nil
}

func (d *RabbitServiceDaoImpl) GetRabbitVersEntitiesByVhostAndNameAndType(ctx context.Context, vhostId int, name string, entType string) ([]model.RabbitVersionedEntity, error) {
	log.InfoC(ctx, "DAO request to GetRabbitVersEntitiesByVhostAndNameAndType for vhostId '%v', name '%v', entity type '%v'", vhostId, name, entType)
	versEntities := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m on e.ms_config_id = m.id").
			Where("m.vhost_id=?", vhostId).
			Where("e.entity_name=?", name).
			Where("e.entity_type=?", entType).
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(versEntities).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByVhostAndNameAndType in db: %v", err.Error())
		return nil, err
	}

	if len(*versEntities) == 0 {
		return nil, nil
	}

	return *versEntities, nil
}

func (d *RabbitServiceDaoImpl) GetRabbitVersEntitiesByVhost(ctx context.Context, vhostId int) ([]model.RabbitVersionedEntity, error) {
	log.InfoC(ctx, "DAO request to GetRabbitVersEntitiesByVhost for vhostId '%v'", vhostId)
	versEntities := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m on e.ms_config_id = m.id").
			Where("m.vhost_id=?", vhostId).
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(versEntities).Error
	}); err != nil {
		log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByVhost in db: %v", err.Error())
		return nil, err
	}

	if len(*versEntities) == 0 {
		return nil, nil
	}

	return *versEntities, nil
}

func (d *RabbitServiceDaoImpl) GetRabbitVersEntitiesByVhostAndNameAndTypeAndActualVersion(ctx context.Context, vhostId int, name string, entType string, actVersion string) ([]model.RabbitVersionedEntity, error) {
	log.InfoC(ctx, "DAO request to GetRabbitVersEntitiesByVhostAndNameAndTypeAndActualVersion for vhostId '%v', name '%v', entity type '%v', actualVersion '%v'", vhostId, name, entType, actVersion)
	versEntities := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m on e.ms_config_id = m.id").
			Where("m.vhost_id=?", vhostId).
			Where("m.actual_version=?", actVersion).
			Where("e.entity_name=?", name).
			Where("e.entity_type=?", entType).
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(versEntities).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error during GetRabbitVersEntitiesByVhostAndNameAndTypeAndActualVersion in db: %w", err)
	}

	if len(*versEntities) == 0 {
		return nil, nil
	}

	return *versEntities, nil
}

func (d *RabbitServiceDaoImpl) GetRabbitVersEntitiesByMsConfigIdAndType(ctx context.Context, msConfigId int, entType model.RabbitEntityType) ([]model.RabbitVersionedEntity, error) {
	log.InfoC(ctx, "DAO request to GetRabbitVersEntitiesByMsConfigIdAndType for msConfigId '%v', entity type '%v'", msConfigId, entType)
	versEntities := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.
			Where("ms_config_id=?", msConfigId).
			Where("entity_type=?", entType.String()).
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(versEntities).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error during GetRabbitVersEntitiesByVhostAndMsNameAndTypeAndVersion in db: %w", err)
	}

	if len(*versEntities) == 0 {
		return nil, nil
	}

	return *versEntities, nil
}

func (d *RabbitServiceDaoImpl) GetRabbitLazyBindings(ctx context.Context, namespace string) ([]model.LazyBindingDto, error) {
	log.InfoC(ctx, "DAO request to GetRabbitLazyBindings by namespace '%v'", namespace)
	lazyBindings := new([]model.LazyBindingDto)

	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Raw(
			`SELECT v.vhost, e.client_entity as entity, ms.candidate_version as exchange_version, ms.actual_version as queue_version
					FROM rabbit_versioned_entities e
					LEFT JOIN rabbit_ms_configs ms on e.ms_config_id = ms.id
					LEFT JOIN rabbit_vhosts v on v.id = ms.vhost_id
					WHERE ms.namespace=? AND e.rabbit_entity IS NULL`,
			namespace).Scan(lazyBindings).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error during GetRabbitLazyBindings in db: %w", err)
	}

	if len(*lazyBindings) == 0 {
		*lazyBindings = []model.LazyBindingDto{}
	}

	return *lazyBindings, nil
}

func (d *RabbitServiceDaoImpl) DeleteAllVersEntitiesByMsConfigId(ctx context.Context, msConfigId int) error {
	log.InfoC(ctx, "DAO request to DeleteAllVersEntitiesByMsConfigId for msConfigId '%v'", msConfigId)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Where("ms_config_id=?", msConfigId).Delete(&model.RabbitVersionedEntity{}).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error delete vers entities from db: %w", err)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) DeleteRabbitVersEntityFromDB(ctx context.Context, entity *model.RabbitVersionedEntity) error {
	log.InfoC(ctx, "DAO request to DeleteRabbitVersEntityFromDB for entity '%+v'", entity)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Delete(entity).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error during DeleteRabbitVersEntityFromDB from db: %w", err)
	}
	return nil
}

func (d *RabbitServiceDaoImpl) DeleteMsConfig(ctx context.Context, msConfig model.MsConfig) error {
	log.InfoC(ctx, "DAO request to DeleteMsConfig for msConfig '%+v'", msConfig)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Delete(&msConfig).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error during DeleteMsConfig from db: %w", err)
	}
	return nil
}

func (d *RabbitServiceDaoImpl) CheckExchangesAndQueuesHaveUniqueName(ctx context.Context, vhostId int, candidateVersion string) error {
	log.InfoC(ctx, "DAO request to CheckExchangesAndQueuesHaveUniqueName for vhostId '%v' and candidateVersion '%v'", vhostId, candidateVersion)
	nonUniqueNames := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Select("e.entity_name").
			Joins("INNER JOIN rabbit_ms_configs as m on e.ms_config_id = m.id").
			Where("m.vhost_id=?", vhostId).
			Where("m.candidate_version=?", candidateVersion).
			Where("e.entity_name <> '' ").
			Group("e.entity_name").Group("e.entity_type").Group("m.vhost_id").
			Having("count(*) > 1").
			Find(nonUniqueNames).Error
	}); err != nil {
		err = fmt.Errorf("error during group by request in CheckExchangesAndQueuesHaveUniqueName in db: %w", err)
		log.ErrorC(ctx, err.Error())
		return err
	}

	if len(*nonUniqueNames) == 0 {
		return nil
	}

	var nonUniqueStrs []string
	for _, ent := range *nonUniqueNames {
		nonUniqueStrs = append(nonUniqueStrs, ent.EntityName)
	}

	//were rows in prev request => not unique names
	versEntities := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m on e.ms_config_id = m.id").
			Where("m.vhost_id=?", vhostId).
			Where("m.candidate_version=?", candidateVersion).
			Where("e.entity_name in ? ", nonUniqueStrs).
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Order("e.entity_name").
			Find(versEntities).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error during CheckExchangesAndQueuesHaveUniqueName in db: %w", err)
	}

	if len(*versEntities) != 0 {
		message := fmt.Sprintf("validation error - these exchanges or queues have non-unique names within vhost '%v' and candidateVersion '%v' :", (*versEntities)[0].MsConfig.Vhost.Classifier, candidateVersion)
		for _, ent := range *versEntities {
			newLine := fmt.Sprintf(" entity name '%v', entity type '%v', ms name '%v'; ", ent.EntityName, ent.EntityType, ent.MsConfig.MsName)
			message = message + " \n " + newLine
		}

		return fmt.Errorf("%v: %w", message, msg.BadRequest)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) CheckNoQueuesWithoutBinding(ctx context.Context, vhostId int, candidateVersion string) error {
	log.InfoC(ctx, "DAO request to CheckNoQueuesWithoutBinding for vhostId '%v' and candidateVersion '%v'", vhostId, candidateVersion)

	queuesWithoutBinding := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m ON e.ms_config_id = m.id").
			Joins("LEFT JOIN (rabbit_versioned_entities as b INNER JOIN rabbit_ms_configs as mb ON b.ms_config_id = mb.id) ON e.entity_name = b.binding_destination AND m.vhost_id = mb.vhost_id AND m.candidate_version = mb.candidate_version").
			Where("m.vhost_id=?", vhostId).
			Where("m.candidate_version=?", candidateVersion).
			Where("e.entity_type='queue'").
			Where("b.id IS NULL").
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(queuesWithoutBinding).Error

		//complex condition, because another vhost or version can have binding for queue with the same name
		//removed condition Where("b.entity_type='binding'"), because now only binding can have 'binding_destination' column filled
	}); err != nil {
		return utils.LogError(log, ctx, "error during request in CheckNoQueuesWithoutBinding in db: %w", err)
	}

	if len(*queuesWithoutBinding) != 0 {
		message := fmt.Sprintf("validation error - these queues have no bindings within vhost '%v' and candidateVersion '%v' :", (*queuesWithoutBinding)[0].MsConfig.Vhost.Classifier, candidateVersion)
		for _, ent := range *queuesWithoutBinding {
			newLine := fmt.Sprintf(" queue name '%v', ms name '%v'; ", ent.EntityName, ent.MsConfig.MsName)
			message = message + "\n" + newLine
		}
		return fmt.Errorf("%v: %w", message, msg.BadRequest)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) CheckBindingsHaveExistingESource(ctx context.Context, vhostId int, candidateVersion string) ([]model.RabbitVersionedEntity, error) {
	log.InfoC(ctx, "DAO request to CheckBindingsHaveExistingESource for vhostId '%v' and candidateVersion '%v'", vhostId, candidateVersion)
	incorrectBindings := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m ON e.ms_config_id = m.id").
			Joins("LEFT JOIN (rabbit_versioned_entities as s INNER JOIN rabbit_ms_configs as ms ON s.ms_config_id = ms.id) ON e.binding_source = s.entity_name AND m.vhost_id = ms.vhost_id AND m.candidate_version = ms.candidate_version").
			Where("m.vhost_id=?", vhostId).
			Where("m.candidate_version=?", candidateVersion).
			Where("e.entity_type='binding'").
			Where("s.id IS NULL").
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(incorrectBindings).Error
	}); err != nil {
		err = fmt.Errorf("error during CheckBindingsHaveExistingESource in db: %w", err)
		log.ErrorC(ctx, err.Error())
		return nil, err
	}

	return *incorrectBindings, nil
}

func (d *RabbitServiceDaoImpl) CheckBindingsHaveExistingQDestination(ctx context.Context, vhostId int, candidateVersion string) error {
	log.InfoC(ctx, "DAO request to CheckBindingsHaveExistingQDestination for vhostId '%v' and candidateVersion '%v'", vhostId, candidateVersion)
	incorrectBindings := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m ON e.ms_config_id = m.id").
			Joins("LEFT JOIN (rabbit_versioned_entities as d INNER JOIN rabbit_ms_configs as md ON d.ms_config_id = md.id) ON e.binding_destination = d.entity_name AND m.vhost_id = md.vhost_id AND m.candidate_version = md.candidate_version").
			Where("m.vhost_id=?", vhostId).
			Where("m.candidate_version=?", candidateVersion).
			Where("e.entity_type='binding'").
			Where("d.id IS NULL").
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(incorrectBindings).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error select binding targets: %w", err)
	}

	if len(*incorrectBindings) != 0 {
		message := fmt.Sprintf("validation error - these bindings have non-existing queue destination within vhost '%v' and candidateVersion '%v' :", (*incorrectBindings)[0].MsConfig.Vhost.Classifier, candidateVersion)
		for _, ent := range *incorrectBindings {
			newLine := fmt.Sprintf(" binding source '%v', binding destination '%v', ms name '%v'; ", ent.BindingSource, ent.BindingDestination, ent.MsConfig.MsName)
			message = message + "\n" + newLine
		}
		return fmt.Errorf("%v: %w", message, msg.BadRequest)
	}

	return nil
}

func (d *RabbitServiceDaoImpl) CheckBindingDeclaredInSameMsAsQ(ctx context.Context, vhostId int, candidateVersion string) error {
	log.InfoC(ctx, "DAO request to CheckBindingDeclaredInSameMsAsQ for vhostId '%v' and candidateVersion '%v'", vhostId, candidateVersion)
	incorrectBindings := new([]model.RabbitVersionedEntity)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Table(model.RabbitVersionedEntity{}.TableName()+" AS e").
			Joins("INNER JOIN rabbit_ms_configs as m ON e.ms_config_id = m.id").
			Joins("INNER JOIN (rabbit_versioned_entities as q INNER JOIN rabbit_ms_configs as mq ON q.ms_config_id = mq.id) ON q.entity_name = e.binding_destination AND m.vhost_id = mq.vhost_id AND m.candidate_version = mq.candidate_version").
			Where("m.vhost_id=?", vhostId).
			Where("m.candidate_version=?", candidateVersion).
			Where("e.entity_type='binding'").
			Where("q.entity_type='queue'").
			Where("e.ms_config_id <> q.ms_config_id").
			Preload("MsConfig").
			Preload("MsConfig.Vhost").
			Find(incorrectBindings).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error selecting dangled bindings: %w", err)
	}

	if len(*incorrectBindings) != 0 {
		message := fmt.Sprintf("validation error - these bindings are declared not in the same microservice config as their destination queue within vhost '%v' and candidateVersion '%v' :", (*incorrectBindings)[0].MsConfig.Vhost.Classifier, candidateVersion)
		for _, ent := range *incorrectBindings {
			newLine := fmt.Sprintf(" binding source '%v', binding destination '%v', binding ms name '%v'; ", ent.BindingSource, ent.BindingDestination, ent.MsConfig.MsName)
			message = message + "\n" + newLine
		}
		return fmt.Errorf("%v: %w", message, msg.BadRequest)
	}

	return nil
}

//bg

func (d *RabbitServiceDaoImpl) DeleteMsConfigsByNamespace(ctx context.Context, namespace string) error {
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).Delete(&model.MsConfig{}).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error deleting msConfigs by namespace `%v': %w", namespace, err)
	}
	return nil
}
