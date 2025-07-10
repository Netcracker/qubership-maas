package rabbit_service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/go-errors/errors"
	"github.com/go-pg/pg/v10"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/monitoring"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/bg_service"
	"github.com/netcracker/qubership-maas/service/instance"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	"github.com/netcracker/qubership-maas/utils"
	"golang.org/x/exp/maps"
	"net/http"
	"strings"
)

var ErrAggregateConfigParsing = errors.New("configurator_service: bad input, check correctness of your YAML")

//go:generate mockgen -source=rabbit_service.go -destination=mock/rabbit_service.go
type RabbitService interface {
	LoadRabbitInstance(ctx context.Context, vhost *model.VHostRegistration) (*model.RabbitInstance, error)

	GetOrCreateVhost(ctx context.Context, instanceId string, classifier *model.Classifier, version *model.Version) (bool, *model.VHostRegistration, error)
	CloneVhost(ctx context.Context, instanceId string, classifier *model.Classifier, version *model.Version) (bool, *model.VHostRegistration, error)
	FindVhostByClassifier(ctx context.Context, classifier *model.Classifier) (*model.VHostRegistration, error)
	FindVhostByClassifierForHelper(ctx context.Context, classifier *model.Classifier) (*model.VHostRegistration, error)
	FindVhostsByNamespace(ctx context.Context, namespace string) ([]model.VHostRegistration, error)
	FindVhostWithSearchForm(ctx context.Context, searchForm *model.SearchForm) ([]model.VHostRegistration, error)
	RemoveVHosts(ctx context.Context, searchForm *model.SearchForm, defaultNamespace string) error

	CreateOrUpdateEntitiesV1(ctx context.Context, vHostRegistration *model.VHostRegistration, entities model.RabbitEntities) (*model.RabbitEntities, []model.UpdateStatus, error)
	ApplyPolicies(ctx context.Context, classifier model.Classifier, policies []interface{}) ([]interface{}, error)
	DeleteEntities(ctx context.Context, classifier model.Classifier, entities model.RabbitDeletions) (*model.RabbitDeletions, error)
	GetConfig(ctx context.Context, classifier model.Classifier) (*model.RabbitEntities, error)
	GetShovels(ctx context.Context, classifier model.Classifier) ([]model.Shovel, error)

	ChangeVersionRoutersActiveVersion(ctx context.Context, classifier model.Classifier, version string) error

	CreateAndRegisterVHost(ctx context.Context, instance *model.RabbitInstance, classifier *model.Classifier, version *model.Version) (*model.VHostRegistrationResponse, error)
	GetConnectionUrl(ctx context.Context, vhost *model.VHostRegistration) (string, error)
	GetApiUrl(ctx context.Context, instanceId string) (string, error)

	ApplyMsConfigAndVersionedEntitiesToDb(ctx context.Context, serviceName string, rabbitConfig *model.RabbitConfigReqDto, vhostId int, candidateVersion string, namespace string) ([]model.RabbitVersionedEntity, error)
	ApplyMssInActiveButNotInCandidateForVhost(ctx context.Context, vhost model.VHostRegistration, activeVersion string, candidateVersion string) error
	CreateVersionedEntities(ctx context.Context, namespace string, candidateVersion string) (model.RabbitEntities, []model.UpdateStatus, error)
	DeleteEntitiesByRabbitVersionedEntities(ctx context.Context, entities []model.RabbitVersionedEntity) (*model.RabbitEntities, error)
	GetLazyBindings(ctx context.Context, namespace string) ([]model.LazyBindingDto, error)

	RabbitBgValidation(ctx context.Context, namespace string, candidateVersion string) error

	// bg1 version
	ApplyBgStatus(ctx context.Context, bgStatusChange *bg_service.BgStatusChangeEvent) error
	// bg2 version
	Warmup(ctx context.Context, state *domain.BGState) error
	Commit(ctx context.Context, state *domain.BGState) error
	Promote(ctx context.Context, state *domain.BGState) error
	Rollback(ctx context.Context, state *domain.BGState) error
	DestroyDomain(ctx context.Context, namespaces *domain.BGNamespaces) error

	RecoverVhostsByNamespace(ctx context.Context, namespace string) error
	CleanupNamespace(ctx context.Context, namespace string) error

	RotatePasswords(ctx context.Context, searchForm *model.SearchForm) ([]model.VHostRegistration, error)
	ProcessExportedVhost(ctx context.Context, namespace string) error
}

const shovelQueue = "-sq"

var log logging.Logger

func init() {
	log = logging.GetLogger("rabbit_service")
}

type RabbitHelperFunc func(ctx context.Context, s RabbitService, instance *model.RabbitInstance, vhost *model.VHostRegistration, classifier *model.Classifier) (helper.RabbitHelper, error)

type KeyManager interface {
	SecurePassword(ctx context.Context, namespace string, password string, client string) (string, error)
	DeletePassword(ctx context.Context, key string) error
}

type RabbitServiceImpl struct {
	rabbitDao       RabbitServiceDao
	instanceService instance.RabbitInstanceService
	km              KeyManager
	resolve         func(ctx context.Context, instance string) (helper.RabbitHelper, string, error)
	getRabbitHelper RabbitHelperFunc
	auditService    monitoring.Auditor
	bgService       bg_service.BgService
	bgDomainService domain.BGDomainService
	authService     auth.AuthService
}

type VhostError struct {
	Err     error
	Message string
}

func (e VhostError) Error() string {
	return fmt.Sprintf("Rabbit vhost error, error: '%s', message: (%s)", e.Err.Error(), e.Message)
}

var (
	NotFoundError                      = errors.New("Registration not found")
	ErrNoDefaultRabbit                 = errors.New("service: default RabbitMQ instance is not configured")
	ErrBothDesignatorAndInstanceRabbit = errors.New("service: it could be no both instance id in vhost and instance designator in vhost's namespace: delete either designator or instance field in vhost")
	ErrVersEntitiesExistsDuringWarmup  = errors.New("rabbit service warmup: all versioned entities should be deleted before bg warmup, please check your maas configuration of mentioned microservices and do rolling update without 'versionedEnitites' section in configs")
)

func NewRabbitService(rabbitDao RabbitServiceDao, instanceService instance.RabbitInstanceService, keyManager KeyManager, auditService monitoring.Auditor, bgService bg_service.BgService, bgDomainService domain.BGDomainService, authService auth.AuthService) RabbitService {
	return NewRabbitServiceWithHelper(rabbitDao, instanceService, keyManager, getRabbitHelperByClassifierAndNamespace, auditService, bgService, bgDomainService, authService)
}
func NewRabbitServiceWithHelper(rabbitDao RabbitServiceDao, instanceService instance.RabbitInstanceService, keyManager KeyManager, rh RabbitHelperFunc, auditService monitoring.Auditor, bgService bg_service.BgService, bgDomainService domain.BGDomainService, authService auth.AuthService) RabbitService {
	rabbitService := RabbitServiceImpl{rabbitDao: rabbitDao, instanceService: instanceService, km: keyManager, getRabbitHelper: rh, auditService: auditService, bgService: bgService, bgDomainService: bgDomainService, authService: authService}

	if bgService != nil {
		bgService.AddBgStatusUpdateCallback(func(ctx context.Context, bgStatusChange *bg_service.BgStatusChangeEvent) {
			if err := rabbitService.ApplyBgStatus(ctx, bgStatusChange); err != nil {
				log.Error("Error update bg status: %s", err)
			}
		})
	}

	return &rabbitService
}

func getRabbitHelperByClassifierAndNamespace(ctx context.Context, s RabbitService, existedInstance *model.RabbitInstance, existedVhost *model.VHostRegistration, classifier *model.Classifier) (helper.RabbitHelper, error) {
	if existedInstance != nil && existedVhost != nil {
		return helper.NewRabbitHelper(*existedInstance, *existedVhost), nil
	}

	vhost, err := s.FindVhostByClassifierForHelper(ctx, classifier)
	if err != nil {
		log.ErrorC(ctx, "error during getting rabbit vhost: %v", err)
		return nil, err
	}
	if vhost == nil {
		log.WarnC(ctx, "didn't manage to find rabbit vhost: %v", err)
		return nil, NotFoundError
	}

	instance, err := s.LoadRabbitInstance(ctx, vhost)
	if err != nil {
		log.ErrorC(ctx, "Error resolving instance: %s", err)
		return nil, errors.New(fmt.Sprintf("Error resolving instance: %s", err))
	}

	return helper.NewRabbitHelper(*instance, *vhost), nil
}

func (s RabbitServiceImpl) ApplyBgStatus(ctx context.Context, bgStatusChange *bg_service.BgStatusChangeEvent) error {
	prevBgStatus := bgStatusChange.Prev
	bgStatus := bgStatusChange.Current
	namespace := bgStatusChange.Namespace

	if prevBgStatus == nil || prevBgStatus.Active != bgStatus.Active {
		log.InfoC(ctx, "Active version has changed, starting to apply new routes")
		vhosts, err := s.FindVhostsByNamespace(ctx, namespace)
		if err != nil {
			return utils.LogError(log, ctx, "error while searching virtual hosts by namespace: %v. Error: %w", namespace, err)
		}
		for _, vhost := range vhosts {
			classifier, err := model.ConvertToClassifier(vhost.Classifier)
			if err != nil {
				return utils.LogError(log, ctx, "Bad classifier: %v. Error: %w", classifier, err)
			}
			err = s.ChangeVersionRoutersActiveVersion(ctx, classifier, bgStatus.Active)
			if err != nil {
				return utils.LogError(log, ctx, "Error while changing version routers default version for vhost: %v. Error: %w", vhost, err)
			}
		}
	} else {
		log.InfoC(ctx, "Active version has not changed, no need to apply new routes")
	}

	if prevBgStatus != nil {
		versionsToDelete := utils.SubstractStringsArray(prevBgStatus.GetAllVersions(), bgStatus.GetAllVersions())
		log.InfoC(ctx, "Checking if previous bg status has versions that are not in new bg status; unnecessary versions that will be deleted: '%v'", versionsToDelete)
		for _, candidateVersion := range versionsToDelete {
			if candidateVersion == "" {
				continue
			}
			err := s.DeleteCandidateVersionRabbit(ctx, namespace, candidateVersion)
			if err != nil {
				return utils.LogError(log, ctx, "Error during DeleteCandidateVersionRabbit for version '%v' : %w", candidateVersion, err)
			}
		}
	}

	return nil
}

func (s RabbitServiceImpl) Commit(ctx context.Context, state *domain.BGState) error {
	log.InfoC(ctx, "Perform commit operation on rabbit subsystem for bgState: %+v", state)
	namespace := ""
	switch {
	case state.Origin.State == "idle":
		namespace = state.Origin.Name
	case state.Peer.State == "idle":
		namespace = state.Peer.Name
	default:
		return utils.LogError(log, ctx, "can't determine idle namespace (wrong BGStatus structure?): %w", msg.BadRequest)
	}

	if vhosts, err := s.FindVhostsByNamespace(ctx, namespace); err == nil {
		for _, vhost := range vhosts {
			classifier, _ := model.ConvertToClassifier(vhost.Classifier)
			if strings.HasSuffix(classifier.Name, "-exported") {
				continue
			}

			log.InfoC(ctx, "Delete vhost: %+v", vhost)
			if err := s.DeleteVHost(ctx, &vhost); err != nil {
				return utils.LogError(log, ctx, "Error during DeleteVHost while in rabbit Commit: '%w'", err)
			}
		}
	} else {
		return utils.LogError(log, ctx, "Error during FindVhostsByNamespace while in rabbit Commit: '%w'", err)
	}

	//exported vhost section
	err := s.ProcessExportedVhost(ctx, state.Origin.Name)
	if err != nil {
		return utils.LogError(log, ctx, "Error during ProcessExportedVhost while in rabbit Commit: %v", err)
	}

	log.InfoC(ctx, "Commit successfully finished for rabbit subsystem")
	return nil
}

func (s *RabbitServiceImpl) Warmup(ctx context.Context, bgState *domain.BGState) error {
	log.InfoC(ctx, "Starting to warmup rabbit for rabbit for bg state: %+v", bgState)

	var activeNamespace, candidateNamespace string
	var candidateVersion model.Version
	if bgState.Origin.State == "active" {
		activeNamespace = bgState.Origin.Name
		candidateNamespace = bgState.Peer.Name
		candidateVersion = bgState.Peer.Version
	} else {
		activeNamespace = bgState.Peer.Name
		candidateNamespace = bgState.Origin.Name
		candidateVersion = bgState.Origin.Version
	}

	vhosts, err := s.rabbitDao.FindVhostsByNamespace(ctx, activeNamespace)
	if err != nil {
		return utils.LogError(log, ctx, "Error during FindVhostsByNamespace while in rabbit Warmup: %v", err)
	}

	for _, oldVhost := range vhosts {
		classifier, _ := model.ConvertToClassifier(oldVhost.Classifier)
		if strings.HasSuffix(classifier.Name, "-exported") {
			continue
		}

		oldClassifier, _ := model.ConvertToClassifier(oldVhost.Classifier)
		if oldClassifier.Name == "exported" {
			continue
		}

		versEntities, err := s.rabbitDao.GetRabbitVersEntitiesByVhost(ctx, oldVhost.Id)
		if err != nil {
			return utils.LogError(log, ctx, "Error during GetRabbitVersEntitiesByVhost while in rabbit Warmup: %w", err)
		}

		if versEntities != nil {
			msNames := map[string]bool{}
			for _, versEnt := range versEntities {
				msg := fmt.Sprintf("'msName: %v, entName: %v, type: %v',", versEnt.MsConfig.MsName, versEnt.EntityName, versEnt.EntityType)
				msNames[msg] = true
			}

			return utils.LogError(log, ctx, "At least one versioned entity exists for microservices '%v': %w", maps.Keys(msNames), ErrVersEntitiesExistsDuringWarmup)
		}

		newClassifier, err := model.ConvertToClassifier(oldVhost.Classifier)
		if err != nil {
			return utils.LogError(log, ctx, "Error during ConvertToClassifier while in rabbit Warmup: %w", err)
		}

		newClassifier.Namespace = candidateNamespace

		oldVhost.Classifier = newClassifier.ToJsonString()

		log.InfoC(ctx, "Clone vhost during warmup with classifier: %+v", newClassifier)
		_, newVhost, err := s.CloneVhost(ctx, oldVhost.InstanceId, &newClassifier, &candidateVersion)
		if err != nil {
			return utils.LogError(log, ctx, "Error during CloneVhost while in rabbit Warmup: %v", err)
		}

		var entities model.RabbitEntities

		allEntities, err := s.rabbitDao.GetRabbitEntitiesByVhostId(ctx, oldVhost.Id)
		for _, entity := range allEntities {
			if entity.EntityType == model.ExchangeType.String() {
				entities.Exchanges = append(entities.Exchanges, entity.ClientEntity)
			}
			if entity.EntityType == model.QueueType.String() {
				entities.Queues = append(entities.Queues, entity.ClientEntity)
			}
			if entity.EntityType == model.BindingType.String() {
				entities.Bindings = append(entities.Bindings, entity.ClientEntity)
			}
		}

		_, _, err = s.CreateOrUpdateEntitiesV1(ctx, newVhost, entities)
		if err != nil {
			return utils.LogError(log, ctx, "Error during CreateOrUpdateEntitiesV1 while in rabbit Warmup: %v", err)
		}
	}

	//exported vhost section
	err = s.ProcessExportedVhost(ctx, bgState.Origin.Name)
	if err != nil {
		return utils.LogError(log, ctx, "Error during ProcessExportedVhost while in rabbit Warmup: %v", err)
	}

	return nil
}

func (s RabbitServiceImpl) Promote(ctx context.Context, state *domain.BGState) error {
	log.InfoC(ctx, "Perform promote operation on rabbit subsystem for bgState: %+v", state)
	activeNamespace := ""
	switch {
	case state.Origin.State == "active":
		activeNamespace = state.Origin.Name
	case state.Peer.State == "active":
		activeNamespace = state.Peer.Name
	default:
		return utils.LogError(log, ctx, "can't determine active activeNamespace (wrong BGStatus structure?): %w", msg.BadRequest)
	}

	vhosts, err := s.FindVhostsByNamespace(ctx, state.Origin.Name)
	if err != nil {
		return utils.LogError(log, ctx, "Error during FindVhostsByNamespace while in Promote: %w", err)
	}

	for _, vhost := range vhosts {
		classifier, _ := model.ConvertToClassifier(vhost.Classifier)
		if !strings.HasSuffix(classifier.Name, "-exported") {
			continue
		}

		err = s.ChangeExportedExchangesActiveVersionBg2(ctx, classifier, activeNamespace)
		if err != nil {
			return utils.LogError(log, ctx, "Error during ChangeExportedExchangesActiveVersionBg2 while in rabbit Promote: %v", err)
		}
	}

	log.InfoC(ctx, "Promote successfully finished for rabbit subsystem")
	return nil
}

func (s RabbitServiceImpl) Rollback(ctx context.Context, state *domain.BGState) error {
	log.InfoC(ctx, "Perform rollback operation on rabbit subsystem for bgState: %+v", state)
	activeNamespace := ""
	switch {
	case state.Origin.State == "active":
		activeNamespace = state.Origin.Name
	case state.Peer.State == "active":
		activeNamespace = state.Peer.Name
	default:
		return utils.LogError(log, ctx, "can't determine active activeNamespace (wrong BGStatus structure?): %w", msg.BadRequest)
	}

	vhosts, err := s.FindVhostsByNamespace(ctx, state.Origin.Name)
	if err != nil {
		return utils.LogError(log, ctx, "Error during FindVhostsByNamespace while in Rollback: %w", err)
	}

	for _, vhost := range vhosts {
		classifier, _ := model.ConvertToClassifier(vhost.Classifier)
		if !strings.HasSuffix(classifier.Name, "-exported") {
			continue
		}

		err = s.ChangeExportedExchangesActiveVersionBg2(ctx, classifier, activeNamespace)
		if err != nil {
			return utils.LogError(log, ctx, "Error during ChangeExportedExchangesActiveVersionBg2 while in rabbit Rollback: %v", err)
		}
	}

	log.InfoC(ctx, "Rollback successfully finished for rabbit subsystem")
	return nil
}

func (s RabbitServiceImpl) DeleteCandidateVersionRabbit(ctx context.Context, namespace string, candidateVersion string) error {
	log.InfoC(ctx, "DeleteCandidateVersion for namespace '%v' and candidateVersion '%v'", namespace, candidateVersion)

	msConfigs, err := s.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(ctx, namespace, candidateVersion)
	if err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigsByBgDomainAndCandidateVersion for namespace '%v' and version '%v': %v", namespace, candidateVersion, err)
		return err
	}

	var rabbitEntities []model.RabbitVersionedEntity

	log.InfoC(ctx, "MsConfigs to be deleted: %+v", msConfigs)
	for _, msConfig := range msConfigs {
		log.InfoC(ctx, "Calculating entities to delete for msConfig: %v", msConfig)
		if msConfig.Entities != nil {
			for _, ent := range msConfig.Entities {

				//we took msConfigs with relation of entities and vhost, so entities doesn't have msConfig link inside
				ent.MsConfig = &msConfig

				switch ent.EntityType {
				case model.ExchangeType.String():
					log.InfoC(ctx, "Adding Rabbit's versioned exchange to delete list for entity: %v", ent)

					//delete E row in entities table
					err := s.rabbitDao.DeleteRabbitVersEntityFromDB(ctx, ent)
					if err != nil {
						log.ErrorC(ctx, "Error during DeleteRabbitVersEntityFromDB for exchange entity '%v': '%v'", ent, err)
						return err
					}

					rabbitEntities = append(rabbitEntities, *ent)

					//check if there are no more E with this name field in db
					entities, err := s.rabbitDao.GetRabbitVersEntitiesByVhostAndNameAndType(ctx, msConfig.VhostID, ent.EntityName, ent.EntityType)
					if err != nil {
						log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByVhostAndNameAndType for exchange entity '%v': '%v'", ent, err)
						return err
					}

					//delete VR and AE, because no more such exchanges
					//VR name is just exchange name without version
					if len(entities) == 0 {
						log.InfoC(ctx, "Adding version router and its alternate exchange to delete list, because no more other versions of this exchange: %v", ent)
						//mimic VR as if it was versioned exchange
						rabbitEntities = append(rabbitEntities, model.RabbitVersionedEntity{
							MsConfig:     &msConfig,
							EntityType:   "exchange",
							EntityName:   ent.EntityName,
							RabbitEntity: map[string]interface{}{"name": ent.EntityName},
						})
					}
				case model.QueueType.String():
					log.InfoC(ctx, "Deleting queue entity in msConfig and adding for delete list in RabbitMQ if neccessary: %v", ent)

					//delete Q row in entities table
					err := s.rabbitDao.DeleteRabbitVersEntityFromDB(ctx, ent)
					if err != nil {
						log.ErrorC(ctx, "Error during DeleteRabbitVersEntityFromDB from DB for queue entity '%v': '%v'", ent, err)
						return err
					}

					//check if there are no more Q with this name field in db
					entities, err := s.rabbitDao.GetRabbitVersEntitiesByVhostAndNameAndTypeAndActualVersion(ctx, msConfig.VhostID, ent.EntityName, ent.EntityType, msConfig.ActualVersion)
					if err != nil {
						log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByVhostAndNameAndTypeAndActualVersion for queue entity '%v': '%v'", ent, err)
						return err
					}

					//delete Q only if no records of queue with this name for other MSs
					if len(entities) == 0 {
						log.InfoC(ctx, "Adding Q to delete list, because no more microservices uses it: %v", ent)
						rabbitEntities = append(rabbitEntities, *ent)
					}

				case model.BindingType.String():
					//delete B row in entities table
					err := s.rabbitDao.DeleteRabbitVersEntityFromDB(ctx, ent)
					if err != nil {
						log.ErrorC(ctx, "Error during DeleteRabbitVersEntityFromDB from DB for binding entity '%v': '%v'", ent, err)
						return err
					}
				}
			}
		}

		//deleting msConfig row
		err := s.rabbitDao.DeleteMsConfig(ctx, msConfig)
		if err != nil {
			log.ErrorC(ctx, "Error during DeleteMsConfig from DB for msConfig '%v': '%v'", msConfig, err)
			return err
		}

		log.InfoC(ctx, "msConfig was deleted successfully: '%v'", msConfig)
	}

	log.InfoC(ctx, "Rabbit entities to be deleted: '%+v'", rabbitEntities)
	_, err = s.DeleteEntitiesByRabbitVersionedEntities(ctx, rabbitEntities)
	if err != nil {
		log.ErrorC(ctx, "Error during DeleteEntitiesByRabbitVersionedEntities: '%v'", err)
		return err
	}

	log.InfoC(ctx, "Candidate '%v' was deleted successfully", candidateVersion)

	return nil
}

func (s RabbitServiceImpl) withVHostRequestAudit(ctx context.Context, handler func() (bool, *model.VHostRegistration, error)) (bool, *model.VHostRegistration, error) {
	found, reg, err := handler()
	if reg != nil && err == nil {
		s.auditService.AddEntityRequestStat(ctx, monitoring.EntityTypeVhost, reg.Vhost, reg.InstanceId)
	}
	return found, reg, err
}

func (s RabbitServiceImpl) GetOrCreateVhost(ctx context.Context, instanceId string, classifier *model.Classifier, version *model.Version) (bool, *model.VHostRegistration, error) {
	err := s.authService.CheckSecurityForSingleNamespace(ctx, model.RequestContextOf(ctx).Namespace, classifier)
	if err != nil {
		return false, nil, utils.LogError(log, ctx, "get or find vhost from namespaces other than request origin is denied: %w", err)
	}

	return s.getOrCreateVhostInternal(ctx, instanceId, classifier, version)
}

func (s RabbitServiceImpl) getOrCreateVhostInternal(ctx context.Context, instanceId string, classifier *model.Classifier, version *model.Version) (bool, *model.VHostRegistration, error) {
	return s.withVHostRequestAudit(ctx, func() (bool, *model.VHostRegistration, error) {
		var reg *model.VHostRegistration = nil

		var found = false
		opError := s.rabbitDao.WithLock(ctx, classifier.ToJsonString(), func(ctx context.Context) error {
			vHostRegistration, err := s.rabbitDao.FindVhostByClassifier(ctx, *classifier)
			if err != nil {
				return err
			}

			if vHostRegistration != nil {
				log.InfoC(ctx, "Virtual host was FOUND with classifier: '%+v'", classifier)

				if instanceId == "" || instanceId == vHostRegistration.InstanceId {
					found = true
					log.InfoC(ctx, "Instance in vhost request is empty or equal for existing vhost, returning existing vhost with classifier: %v", classifier)
					reg = vHostRegistration
					return nil

					//return, because we will NOT create vhost in new instance, just return existing
				} else {
					log.InfoC(ctx, "Instance in vhost request != instance in existing vhost, old vhost will be deleted from MaaS registry (but not in Rabbit) and new vhost will be created in new instance for vhost with classifier: %v", classifier)
					err = s.rabbitDao.DeleteVhostRegistration(ctx, vHostRegistration)
					if err != nil {
						log.ErrorC(ctx, "Error in DeleteVhostRegistration during GetOrCreateVhost: %v", err)
						return err
					}

					//no return, because we will create vhost in new instance
				}
			}

			log.InfoC(ctx, "Virtual host was NOT FOUND with classifier '%v'. Start creation.", classifier)

			// If no default instance for NEW vhost, then set it
			resolvedInstance, err := s.resolveRabbitInstance(ctx, instanceId, *classifier, classifier.Namespace)
			if err != nil {
				return utils.LogError(log, ctx, "Error during resolveRabbitInstance while GetOrCreateVhost: %w", err)
			}
			if resolvedInstance == nil {
				resolvedInstance, err = s.instanceService.GetDefault(ctx)
				if err != nil {
					return utils.LogError(log, ctx, "error in FindDefaultRabbitInstance during GetOrCreateVhost: %w", err)
				}
				if resolvedInstance == nil {
					return utils.LogError(log, ctx, "no RabbitMQ instance registered yet: %w", msg.BadRequest)
				}
				log.InfoC(ctx, "RabbitMQ instance '%s' is default", resolvedInstance)
			}

			_, err = s.CreateAndRegisterVHost(ctx, resolvedInstance, classifier, version)
			if err != nil {
				return err
			}
			vHostRegistration, err = s.rabbitDao.FindVhostByClassifier(ctx, *classifier)
			if err != nil {
				return err
			}
			if vHostRegistration == nil {
				log.ErrorC(ctx, "Vhost was not found after creation by classifier: %v", classifier)
				return fmt.Errorf("vhost was not found after creation by classifier: %v", classifier)
			}

			reg = vHostRegistration
			return nil
		})
		return found, reg, opError
	})
}

// todo make single method
func (s RabbitServiceImpl) CloneVhost(ctx context.Context, instanceId string, classifier *model.Classifier, version *model.Version) (bool, *model.VHostRegistration, error) {
	return s.withVHostRequestAudit(ctx, func() (bool, *model.VHostRegistration, error) {
		var reg *model.VHostRegistration = nil

		var found = false
		opError := s.rabbitDao.WithLock(ctx, classifier.ToJsonString(), func(ctx context.Context) error {
			vHostRegistration, err := s.FindVhostByClassifierForHelper(ctx, classifier)
			if err != nil {
				return err
			}

			if vHostRegistration != nil {
				log.InfoC(ctx, "Virtual host was FOUND with classifier: '%+v'", classifier)

				if instanceId == "" || instanceId == vHostRegistration.InstanceId {
					found = true
					log.InfoC(ctx, "Instance in vhost request is empty or equal for existing vhost, returning existing vhost with classifier: %v", classifier)
					reg = vHostRegistration
					return nil

					//return, because we will NOT create vhost in new instance, just return existing
				} else {
					log.InfoC(ctx, "Instance in vhost request != instance in existing vhost, old vhost will be deleted from MaaS registry (but not in Rabbit) and new vhost will be created in new instance for vhost with classifier: %v", classifier)
					err = s.rabbitDao.DeleteVhostRegistration(ctx, vHostRegistration)
					if err != nil {
						log.ErrorC(ctx, "Error in DeleteVhostRegistration during GetOrCreateVhost: %v", err)
						return err
					}

					//no return, because we will create vhost in new instance
				}
			}

			log.InfoC(ctx, "Virtual host was NOT FOUND with classifier '%v'. Start creation.", classifier)

			resolvedInstance, err := s.resolveRabbitInstance(ctx, instanceId, *classifier, classifier.Namespace)
			if err != nil {
				return utils.LogError(log, ctx, "Error during resolveRabbitInstance while GetOrCreateVhost: %w", err)
			}

			// If no default instance for NEW vhost, then set it
			if resolvedInstance == nil {
				resolvedInstance, err = s.instanceService.GetDefault(ctx)
				if err != nil {
					return utils.LogError(log, ctx, "error in FindDefaultRabbitInstance during CloneVhost: %w", err)
				}
				if resolvedInstance == nil {
					return utils.LogError(log, ctx, "no RabbitMQ instance registered yet: %w", msg.BadRequest)
				}
				log.InfoC(ctx, "RabbitMQ instance '%s' is default", resolvedInstance)
			}

			_, err = s.CreateAndRegisterVHost(ctx, resolvedInstance, classifier, version)
			if err != nil {
				return err
			}
			vHostRegistration, err = s.FindVhostByClassifierForHelper(ctx, classifier)
			if err != nil {
				return err
			}
			if vHostRegistration == nil {
				log.ErrorC(ctx, "Vhost was not found after creation by classifier: %v", classifier)
				return fmt.Errorf("vhost was not found after creation by classifier: %v", classifier)
			}

			reg = vHostRegistration
			return nil
		})
		return found, reg, opError
	})
}

func (s RabbitServiceImpl) FindVhostByClassifier(ctx context.Context, classifier *model.Classifier) (*model.VHostRegistration, error) {
	log.InfoC(ctx, "Find vhost by classifier: %v", classifier)
	if model.SecurityContextOf(ctx).IsCompositeIsolationDisabled() {
		log.InfoC(ctx, "Composite isolation disabled by requester")
	} else {
		err := s.authService.CheckSecurityForBoundNamespaces(ctx, model.RequestContextOf(ctx).Namespace, classifier)
		if err != nil {
			return nil, utils.LogError(log, ctx, "vhost access error for classifier %v: %w", classifier, err)
		}
	}

	return s.rabbitDao.FindVhostByClassifier(ctx, *classifier)
}

func (s RabbitServiceImpl) FindVhostByClassifierForHelper(ctx context.Context, classifier *model.Classifier) (*model.VHostRegistration, error) {
	return s.rabbitDao.FindVhostByClassifier(ctx, *classifier)
}

func (s RabbitServiceImpl) FindVhostsByNamespace(ctx context.Context, namespace string) ([]model.VHostRegistration, error) {
	return s.rabbitDao.FindVhostsByNamespace(ctx, namespace)
}

func (s RabbitServiceImpl) FindVhostWithSearchForm(ctx context.Context, searchForm *model.SearchForm) ([]model.VHostRegistration, error) {
	return s.rabbitDao.FindVhostWithSearchForm(ctx, searchForm)
}

func (s RabbitServiceImpl) RemoveVHosts(ctx context.Context, searchForm *model.SearchForm, defaultNamespace string) error {
	if searchForm.IsEmpty() {
		return utils.LogError(log, ctx, "remove vhost clause can't be empty: %w", msg.BadRequest)
	}

	securityContext := model.SecurityContextOf(ctx)
	if !securityContext.UserHasRoles(model.ManagerRole) && searchForm.Namespace == "" {
		namespace := defaultNamespace
		log.WarnC(ctx, "Restrict search criteria for user with Agent role to namespaced: %v", namespace)
		searchForm.Namespace = namespace
	}

	log.InfoC(ctx, "Remove vhosts with search form '%+v'", searchForm)
	vhosts, err := s.rabbitDao.FindVhostWithSearchForm(ctx, searchForm)
	if err != nil {
		return utils.LogError(log, ctx, "Error occurred while searching for vhosts in database: '%w'", err)
	}

	if len(vhosts) == 0 {
		return utils.LogError(log, ctx, "no vhost registrations matched search were found: %w", msg.NotFound)
	}

	var lockError error
	for _, vhost := range vhosts {
		log.InfoC(ctx, "Preparing to delete vhost: %+v", vhost)
		lockError = s.rabbitDao.WithLock(ctx, vhost.Classifier, func(ctx context.Context) error {
			if err = s.DeleteVHost(ctx, &vhost); err != nil {
				log.ErrorC(ctx, "Error deleting vhost %v from rabbitmq: %s", vhost, err.Error())
				return errors.New("Error occurred while deleting vhost from rabbitmq")
			}
			return nil
		})
		if lockError != nil {
			log.ErrorC(ctx, "Error in transaction during vhosts deletion by filter: %v", lockError)
			break
		}
	}

	return lockError
}

func (s RabbitServiceImpl) CleanupNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "deleting vhosts by namespace: %v", namespace)
	searchForm := model.SearchForm{Namespace: namespace}

	if err := s.RemoveVHosts(ctx, &searchForm, model.RequestContextOf(ctx).Namespace); err != nil {
		switch {
		case errors.Is(err, msg.NotFound):
			log.DebugC(ctx, "no vhosts were found for namespace: %v", namespace)
		default:
			return utils.LogError(log, ctx, "error deleting vhosts for namespace: %w", err)
		}
	}

	log.InfoC(ctx, "deleting msConfigs by namespace: %v", namespace)
	if err := s.rabbitDao.DeleteMsConfigsByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error deleting msConfigs by namespace: %w", err)
	}

	return nil
}

func (s *RabbitServiceImpl) CreateAndRegisterVHost(ctx context.Context, instance *model.RabbitInstance, classifier *model.Classifier, version *model.Version) (*model.VHostRegistrationResponse, error) {
	// NOTE! Golang sorts classifer fields by default, so we can compare jsons as pure strings
	cs, _ := json.Marshal(&classifier)
	classifierStr := string(cs)
	password := utils.CompactUuid()
	namespace := classifier.Namespace

	var vhost *model.VHostRegistration
	if securedPassword, err := s.km.SecurePassword(ctx, namespace, password, ""); err != nil {
		log.ErrorC(ctx, "Error secure password using key-management: %v", err.Error())
		return nil, err
	} else {
		vhost = &model.VHostRegistration{
			InstanceId: instance.Id,
			Vhost:      s.FormatVHostName(*classifier, version),
			User:       fmt.Sprintf("maas.%s.%s", namespace, utils.CompactUuid()[0:10]),
			Password:   securedPassword,
			Namespace:  namespace,
			Classifier: classifierStr,
		}
	}

	rabbitHelper, err := s.getRabbitHelper(ctx, s, instance, vhost, classifier)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
		return nil, err
	}

	if err := s.rabbitDao.InsertVhostRegistration(ctx, vhost, func(reg *model.VHostRegistration) error {
		return rabbitHelper.CreateVHost(ctx)
	}); err != nil {
		if errors.Is(err, dao.ClassifierUniqIndexErr) {
			return nil, dao.ClassifierUniqIndexErr
		}
		s.km.DeletePassword(ctx, vhost.Password)
		log.Error("Registration error: %v", err.Error())
		return nil, err
	}

	cnnUrl := rabbitHelper.FormatCnnUrl(vhost.Vhost)
	resp := &model.VHostRegistrationResponse{
		Cnn:      cnnUrl,
		Username: vhost.User,
		Password: vhost.Password,
	}

	log.InfoC(ctx, "Successfully registered new vhost: %+v", resp)
	return resp, nil
}

func (s *RabbitServiceImpl) DeleteVHost(ctx context.Context, vhost *model.VHostRegistration) error {
	instance, err := s.LoadRabbitInstance(ctx, vhost)
	if err != nil {
		return err
	}
	rabbitHelper, err := s.getRabbitHelper(ctx, s, instance, vhost, nil)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper in DeleteVHost: %v", err)
		return err
	}

	// delete vhost and user in rabbit
	if err := rabbitHelper.DeleteVHost(ctx); err != nil {
		return utils.LogError(log, ctx, "Error delete vhost: %w", err)
	}

	// delete password in KMS if it prefixed by "km:"
	if err := s.km.DeletePassword(ctx, vhost.Password); err != nil {
		return utils.LogError(log, ctx, "Error delete password from keq management: %w", err)
	}

	// delete registration record in db
	if err := s.rabbitDao.DeleteVhostRegistration(ctx, vhost); err != nil {
		return utils.LogError(log, ctx, "Error delete registration record from db: %w", err)
	}

	log.InfoC(ctx, "Vhost with name '%s' was successfully deleted", vhost.Vhost)
	return nil
}

func (s *RabbitServiceImpl) ResolveRabbitInstanceById(ctx context.Context, instance string) (*model.RabbitInstance, error) {
	rabbitInstance := &model.RabbitInstance{}
	var err error

	if instance == "" {
		rabbitInstance, err = s.instanceService.GetDefault(ctx)
		if err != nil {
			if errors.Is(err, pg.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}

		if rabbitInstance == nil {
			return nil, nil
		}
		log.InfoC(ctx, "RabbitMQ instance '%s' is default", rabbitInstance.Id)
	} else {
		rabbitInstance, err = s.instanceService.GetById(ctx, instance)
		if err != nil {
			log.ErrorC(ctx, "Error during FindRabbitInstanceById in ResolveRabbitInstanceById: %v", err)
			return nil, err
		}
	}
	log.InfoC(ctx, "Instance was resolved: %+v", rabbitInstance)
	return rabbitInstance, nil
}

func (srv *RabbitServiceImpl) resolveRabbitInstance(ctx context.Context, instance string, classifier model.Classifier, namespace string) (*model.RabbitInstance, error) {
	log.InfoC(ctx, "Starting to resolve rabbit instance for instance '%v' and namespace '%v'", instance, namespace)
	designator, err := srv.instanceService.GetRabbitInstanceDesignatorByNamespace(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "error during getting rabbit instance designator for namespace '%+v': %v", namespace, err)
		return nil, err
	}

	if designator != nil {
		log.InfoC(ctx, "instance will be resolved for rabbit by instance designator found for namespace '%+v', designator: '%v'", namespace, designator)

		if instance != "" {
			errMsg := fmt.Sprintf("rabbit instance designator exists in namespace '%v', vhost instance field == '%v', you should use only one of these approaches.", namespace, instance)
			log.ErrorC(ctx, errMsg)
			return nil, VhostError{
				Err:     ErrBothDesignatorAndInstanceRabbit,
				Message: errMsg,
			}
		}

		instanceFromDesignator, match, matchedBy := srv.MatchDesignator(ctx, classifier, designator)
		if match {
			log.InfoC(ctx, "selector with ClassifierMatch '%+v' from instance designator was found for vhost with classifier '%+v'. Chosen instance is '%+v'", matchedBy, classifier, instanceFromDesignator)
		}

		if instanceFromDesignator == nil && designator.DefaultInstanceId != nil {
			instanceFromDesignator = designator.DefaultInstance
			log.InfoC(ctx, "no selector from instance designator was found for vhost with classifier '%s'. Default designator instance will be used :'%+v'", classifier, instanceFromDesignator)
		}

		if instanceFromDesignator == nil {
			log.InfoC(ctx, "Instance designator exists, but both selectors not matched and default designator instance is empty, instance resolved as 'nil' for vhost with classifier: %v", classifier)
			return nil, nil
		} else {
			log.InfoC(ctx, "Resolved Rabbit instance by instance designator for vhost with classifier '%+v' is '%+v'", classifier, instanceFromDesignator)
			return instanceFromDesignator, nil
		}
	}

	log.InfoC(ctx, "instance will be resolved by instance field in rabbit (or default), because instance designator was not found. rabbit instance field: '%v'", instance)

	if instance == "" {
		log.InfoC(ctx, "Rabbit instance is not specified in request, designator doesn't exist, instance resolved as 'nil' for vhost with classifier: %v", classifier)
		return nil, nil
	} else {
		rabbitInstance, err := srv.instanceService.GetById(ctx, instance)
		if err != nil {
			log.ErrorC(ctx, "Error in FindRabbitInstanceById during resolveRabbitInstance: %v", err)
			return nil, err
		}
		return rabbitInstance, nil
	}
}

// LoadRabbitInstance loads rabbit instance for the vhost. model.Vhost#InstanceId must not be empty!
func (srv *RabbitServiceImpl) LoadRabbitInstance(ctx context.Context, vhost *model.VHostRegistration) (*model.RabbitInstance, error) {
	//instance is already set in topic
	instance, err := srv.instanceService.GetById(ctx, vhost.InstanceId)
	if err != nil {
		log.ErrorC(ctx, "Error during FindRabbitInstanceById in LoadRabbitInstance: %v", err)
		return nil, err
	}
	if instance == nil {
		return nil, errors.New(fmt.Sprintf("service: vhost with name '%s' refers on non-existent rabbit instance '%s'", vhost.Vhost, instance.Id))
	}
	return instance, nil
}

func (s *RabbitServiceImpl) FormatVHostName(classifier model.Classifier, version *model.Version) string {
	tenantPart := ""
	if tenantId := classifier.TenantId; tenantId != "" {
		tenantPart = fmt.Sprintf(".%s", tenantId)
	}

	name := classifier.Name
	namePart := fmt.Sprintf(".%s", name)

	versionPart := ""
	if version != nil && *version != "v1" {
		versionPart = "-" + string(*version)
	}

	vhostName := fmt.Sprintf("maas.%s%s%s%s", classifier.Namespace, tenantPart, namePart, versionPart)

	return vhostName
}

func (s *RabbitServiceImpl) GetConnectionUrl(ctx context.Context, vhost *model.VHostRegistration) (string, error) {
	instance, err := s.ResolveRabbitInstanceById(ctx, vhost.InstanceId)
	if err != nil {
		return "", utils.LogError(log, ctx, "error during resolving rabbit instance in GetConnectionUrl: %w", err)
	}

	if instance == nil || vhost == nil {
		return "", utils.LogError(log, ctx, "instance or vhost is nil in GetConnectionUrl, instance '%+v', vhost '%+v': %w", instance, vhost, msg.BadRequest)
	}

	rh := helper.NewRabbitHelper(*instance, *vhost)
	return rh.FormatCnnUrl(vhost.Vhost), nil
}

func (s *RabbitServiceImpl) GetApiUrl(ctx context.Context, instanceId string) (string, error) {
	instance, err := s.ResolveRabbitInstanceById(ctx, instanceId)
	if err != nil {
		return "", utils.LogError(log, ctx, "error during resolving rabbit instance in GetApiUrl: %w", err)
	}

	if instance == nil {
		return "", utils.LogError(log, ctx, "instance is nil in GetApiUrl: %w", msg.BadRequest)
	}

	return instance.ApiUrl, nil
}

// Config section
func (s *RabbitServiceImpl) CreateOrUpdateEntitiesV1(ctx context.Context, vHostRegistration *model.VHostRegistration, entities model.RabbitEntities) (*model.RabbitEntities, []model.UpdateStatus, error) {
	log.InfoC(ctx, "Starting to create/update entities for vhost with classifier: %+v", vHostRegistration.Classifier)

	result := &model.RabbitEntities{}
	var updateStatus []model.UpdateStatus
	var err error

	classifier, _ := model.ConvertToClassifier(vHostRegistration.Classifier)
	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
		return result, nil, err
	}

	resultIsNotNil := false
	for _, ent := range entities.Exchanges {
		resultIsNotNil = true

		//todo refactor interface{} to map[string]interface{}
		createdExchange, updateReason, err := rabbitHelper.CreateExchange(ctx, ent)
		if err != nil {
			return result, nil, err
		}

		if createdExchange != nil {
			result.Exchanges = append(result.Exchanges, createdExchange)
		}

		rabbitEntity, err := model.NewRabbitEntity(ent, createdExchange.(*map[string]interface{}), model.ExchangeType, *vHostRegistration, classifier)
		if err != nil {
			log.ErrorC(ctx, "Error during converting user entity to rabbitEntity: %v", err)
			return result, nil, err
		}

		err = s.rabbitDao.UpsertNamedRabbitEntity(ctx, rabbitEntity)
		if err != nil {
			log.ErrorC(ctx, "error in UpsertNamedRabbitEntity while in CreateOrUpdateEntitiesV1 for exchange: '%+v' error: %v", rabbitEntity, err)
			return result, nil, err
		}

		if updateReason != "" {
			updateStatus = append(updateStatus, model.UpdateStatus{Entity: createdExchange, Status: "updated", Reason: updateReason})
		}
	}

	//lazy binding check moved here, because lazy binding could exist in DB, but config can have both E and B and created binding will conflict with existing (but not created) lazy binding

	//select all lazy bindings with empty rabbit entity field by classifier
	lazyBindings, err := s.rabbitDao.GetNotCreatedLazyBindingsByClassifier(ctx, classifier.ToJsonString())
	if err != nil {
		log.ErrorC(ctx, "error GetNotCreatedLazyBindingsByClassifier while in CreateOrUpdateEntitiesV1: %v", err)
		return result, nil, err
	}

	//cycle through them and try to apply client entity field
	for _, lazyBinding := range lazyBindings {
		clientBinding := lazyBinding.ClientEntity
		createdBinding, err := rabbitHelper.CreateNormalOrLazyBinding(ctx, clientBinding)
		if err != nil {
			log.ErrorC(ctx, "error during CreateNormalOrLazyBinding while in CreateOrUpdateEntitiesV1 for binding: '%+v' error: %v", lazyBinding, err)
			return result, nil, err
		}

		//if success - update lazy binding RabbitEntity field with created binding
		if createdBinding != nil {
			lazyBinding.RabbitEntity = createdBinding
			err = s.rabbitDao.UpdateLazyBinding(ctx, &lazyBinding)
			if err != nil {
				log.ErrorC(ctx, "error UpdateLazyBinding while in CreateOrUpdateEntitiesV1 for binding: '%+v' error: %v", lazyBinding, err)
				return result, nil, err
			}
		}

	}

	for _, ent := range entities.Queues {
		resultIsNotNil = true

		//todo refactor interface{} to map[string]interface{}
		createdQueue, updateReason, err := rabbitHelper.CreateQueue(ctx, ent)
		if err != nil {
			return result, nil, err
		}

		if createdQueue != nil {
			result.Queues = append(result.Queues, createdQueue)
		}

		rabbitEntity, err := model.NewRabbitEntity(ent, createdQueue.(*map[string]interface{}), model.QueueType, *vHostRegistration, classifier)
		if err != nil {
			log.ErrorC(ctx, "Error during converting user entity to rabbitEntity: %v", err)
			return result, nil, err
		}

		err = s.rabbitDao.UpsertNamedRabbitEntity(ctx, rabbitEntity)
		if err != nil {
			log.ErrorC(ctx, "error in UpsertNamedRabbitEntity while in CreateOrUpdateEntitiesV1 for queue: '%+v' error: %v", rabbitEntity, err)
			return result, nil, err
		}

		if updateReason != "" {
			updateStatus = append(updateStatus, model.UpdateStatus{Entity: createdQueue, Status: "updated", Reason: updateReason})
		}
	}

	for _, ent := range entities.Bindings {
		resultIsNotNil = true

		binding := ent.(map[string]interface{})
		createdBinding, err := rabbitHelper.CreateNormalOrLazyBinding(ctx, binding)
		if err != nil {
			log.ErrorC(ctx, "error duringCreateNormalOrLazyBinding while in CreateOrUpdateEntitiesV1: %v", err)
			return result, nil, err
		}

		source, destination, err := utils.ExtractSourceAndDestination(binding)
		lazyBinding := model.RabbitEntity{
			VhostId:    vHostRegistration.Id,
			Namespace:  vHostRegistration.Namespace,
			Classifier: classifier.ToJsonString(),
			EntityType: "binding",
			BindingSource: sql.NullString{
				String: source,
				Valid:  source != "",
			},
			BindingDestination: sql.NullString{
				String: destination,
				Valid:  destination != "",
			},
			ClientEntity: binding,
			RabbitEntity: createdBinding,
		}

		err = s.rabbitDao.InsertLazyBinding(ctx, &lazyBinding)
		if err != nil {
			log.ErrorC(ctx, "error InsertLazyBinding while in CreateOrUpdateEntitiesV1: %v", err)
			return result, nil, err
		}

		//createdBinding will be nil if it is lazy binding and was not created in rabbit, so no adding to response
		if createdBinding != nil {
			result.Bindings = append(result.Bindings, createdBinding)
		}

		if !resultIsNotNil {
			result = nil
		}
	}

	//deletion section - delete lazy binding by classifier source and dest

	return result, updateStatus, nil
}

func (s *RabbitServiceImpl) ApplyPolicies(ctx context.Context, classifier model.Classifier, policies []interface{}) ([]interface{}, error) {
	var result []interface{}
	var err error

	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
		return result, err
	}

	if policies != nil {
		for _, ent := range policies {
			err := createEntity(ctx, ent, rabbitHelper.CreatePolicy, &result, nil)
			if err != nil {
				return result, err
			}
		}
	}

	return result, nil
}

func (s *RabbitServiceImpl) createVersionRouterWithItsAE(ctx context.Context, rabbitHelper helper.RabbitHelper, versionRouter string) error {
	//creating alternateExchange
	aeName := getAltExchNameByVersionRouter(versionRouter)
	ae := map[string]interface{}{
		"name": aeName,
		"type": "fanout",
	}
	log.InfoC(ctx, "Creating alternate exchange: %v", ae)
	err := createEntity(ctx, ae, rabbitHelper.CreateExchange, nil, nil)
	if err != nil {
		log.ErrorC(ctx, "Error during creating alternate exchange: %v error: %v", ae, err)
		return err
	}

	//creating versionRouter with binded ae
	vrExchangeArguments := map[string]interface{}{
		"alternate-exchange": aeName,
		"blue-green":         "true",
	}
	vrExchange := map[string]interface{}{
		"name":      versionRouter,
		"type":      "headers",
		"arguments": vrExchangeArguments,
	}
	log.InfoC(ctx, "Creating exchange with binded alternate exchange: %v", vrExchange)
	err = createEntity(ctx, vrExchange, rabbitHelper.CreateExchange, nil, nil)
	if err != nil {
		log.ErrorC(ctx, "Error during creating exchange: %v error: %v", vrExchange, err)
		return err
	}

	return nil
}

func (s *RabbitServiceImpl) DeleteEntities(ctx context.Context, classifier model.Classifier, entities model.RabbitDeletions) (*model.RabbitDeletions, error) {
	result := &model.RabbitDeletions{}

	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
		return result, err
	}

	resultIsNotNil := false

	if entities.Exchanges != nil {
		resultIsNotNil = true
		for _, ent := range entities.Exchanges {
			err := deleteEntity(ctx, ent, rabbitHelper.DeleteExchange, &result.Exchanges)
			if err != nil {
				return result, err
			}

			rabbitEntity, err := model.NewRabbitEntity(ent, nil, model.ExchangeType, model.VHostRegistration{}, classifier)
			if err != nil {
				log.ErrorC(ctx, "error in NewRabbitEntity while in DeleteEntities for exchange: '%+v' error: %v", ent, err)
				return result, err
			}
			err = s.rabbitDao.DeleteNamedRabbitEntity(ctx, rabbitEntity)
			if err != nil {
				log.ErrorC(ctx, "error in DeleteNamedRabbitEntity while in DeleteEntities for exchange: '%+v' error: %v", ent, err)
				return result, err
			}
		}
	}

	if entities.Queues != nil {
		resultIsNotNil = true
		for _, ent := range entities.Queues {
			err := deleteEntity(ctx, ent, rabbitHelper.DeleteQueue, &result.Queues)
			if err != nil {
				return result, err
			}

			rabbitEntity, err := model.NewRabbitEntity(ent, nil, model.QueueType, model.VHostRegistration{}, classifier)
			if err != nil {
				log.ErrorC(ctx, "error in NewRabbitEntity while in DeleteEntities for queue: '%+v' error: %v", ent, err)
				return result, err
			}
			err = s.rabbitDao.DeleteNamedRabbitEntity(ctx, rabbitEntity)
			if err != nil {
				log.ErrorC(ctx, "error in DeleteNamedRabbitEntity while in DeleteEntities for queue: '%+v' error: %v", ent, err)
				return result, err
			}
		}
	}

	if entities.Bindings != nil {
		resultIsNotNil = true
		for _, ent := range entities.Bindings {
			err := deleteEntity(ctx, ent, rabbitHelper.DeleteBinding, &result.Bindings)
			if err != nil {
				return result, err
			}

			//deletion section - delete lazy binding by classifier source and dest
			err = s.rabbitDao.DeleteLazyBindingBySourceAndDestinationAndClassifier(ctx, classifier.ToJsonString(), ent)
			if err != nil {
				log.ErrorC(ctx, "error during DeleteLazyBindingBySourceAndDestinationAndClassifier while in DeleteEntities: %v", err)
				return result, err
			}

		}
	}

	if entities.Policies != nil {
		resultIsNotNil = true
		for _, ent := range entities.Policies {
			err := deleteEntity(ctx, ent, rabbitHelper.DeletePolicy, &result.Policies)
			if err != nil {
				return result, err
			}
		}
	}

	if !resultIsNotNil {
		result = nil
	}
	return result, nil
}

func (s *RabbitServiceImpl) DeleteEntitiesByRabbitVersionedEntities(ctx context.Context, versionedEntities []model.RabbitVersionedEntity) (*model.RabbitEntities, error) {
	result := &model.RabbitEntities{}

	for _, versionedEntity := range versionedEntities {
		if versionedEntity.MsConfig == nil || versionedEntity.MsConfig.Vhost == nil || versionedEntity.MsConfig.Vhost.Classifier == "" {
			log.ErrorC(ctx, "msConfig link is nil or its Vhost is nil or classifier is empty for versionedEntity '%v'", versionedEntity)
			return result, errors.New(fmt.Sprintf("msConfig link is nil or its Vhost is nil for versionedEntity '%v'", versionedEntity))
		}

		classifier, err := model.ConvertToClassifier(versionedEntity.MsConfig.Vhost.Classifier)
		if err != nil {
			log.ErrorC(ctx, "error during ConvertToClassifier for versionedEntity '%v': %v", versionedEntity, err)
			return result, err
		}
		rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
		if err != nil {
			log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
			return result, err
		}

		if versionedEntity.EntityType == "exchange" {
			err := deleteEntity(ctx, versionedEntity.RabbitEntity, rabbitHelper.DeleteExchange, &result.Exchanges)
			if err != nil {
				log.ErrorC(ctx, "error during exchange deletion for exchange '%+v': %v", versionedEntity, err)
				return result, err
			}
		} else if versionedEntity.EntityType == "queue" {
			err := deleteEntity(ctx, versionedEntity.RabbitEntity, rabbitHelper.DeleteQueue, &result.Queues)
			if err != nil {
				log.ErrorC(ctx, "error during queue deletion for queue '%+v': %v", versionedEntity, err)
				return result, err
			}
		} else if versionedEntity.EntityType == "binding" {
			// lazy binding might not be created, RabbitEntity field should be empty then
			if versionedEntity.RabbitEntity != nil {
				err := deleteEntity(ctx, versionedEntity.RabbitEntity, rabbitHelper.DeleteBinding, &result.Bindings)
				if err != nil {
					log.ErrorC(ctx, "error during binding deletion for binding '%+v': %v", versionedEntity, err)
					return result, err
				}
			}
		}
	}
	return result, nil
}

func (s *RabbitServiceImpl) GetConfig(ctx context.Context, classifier model.Classifier) (*model.RabbitEntities, error) {
	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		return nil, err
	}
	result, err := rabbitHelper.GetAllEntities(ctx)
	return &result, err
}

func (s *RabbitServiceImpl) GetShovels(ctx context.Context, classifier model.Classifier) ([]model.Shovel, error) {
	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		return nil, err
	}
	result, err := rabbitHelper.GetVhostShovels(ctx)
	return result, err
}

func createEntity(ctx context.Context, ent interface{}, handler helper.CreateEntityAction, result *[]interface{}, updateStatus *[]model.UpdateStatus) error {
	entity, updateReason, err := handler(ctx, ent)
	if err != nil {
		return err
	}
	if entity != nil {
		if result != nil {
			*result = append(*result, entity)
		}
	} else {
		return errors.Errorf("Entity was created without error, but get request returned nil for entity: %v", ent)
	}
	if updateReason != "" {
		if updateStatus != nil {
			*updateStatus = append(*updateStatus, model.UpdateStatus{entity, "updated", updateReason})
		}
	}
	return nil
}

func deleteEntity(ctx context.Context, ent interface{}, handler helper.EntityAction, result *[]interface{}) error {
	entity, err := handler(ctx, ent)
	if err != nil {
		return err
	}
	if entity != nil {
		if result != nil {
			*result = append(*result, entity)
		}
	}
	return nil
}

// Blue-green section
func (s *RabbitServiceImpl) ApplyMsConfigAndVersionedEntitiesToDb(ctx context.Context, serviceName string, rabbitConfig *model.RabbitConfigReqDto, vhostId int, candidateVersion string, namespace string) ([]model.RabbitVersionedEntity, error) {
	var entitiesToBeDeleted []model.RabbitVersionedEntity

	//bg2 warning about future migration
	if rabbitConfig.Spec.VersionedEntities != nil {
		log.WarnC(ctx, "WARNING: 'versionedEntities' section in maas configuration is not supported anymore. Please, switch it to 'entities'. Microservice name: '%v', versioned entities: %+v", serviceName, rabbitConfig.Spec.VersionedEntities)
	}

	//check if there is msConfig for this candidateVersion
	msConfig, err := s.rabbitDao.GetMsConfigByVhostAndMsNameAndCandidateVersion(ctx, vhostId, serviceName, candidateVersion)
	if err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigByVhostAndMsNameAndCandidateVersion: %v", err)
		return nil, err
	}

	//3 cases: deploy, update of previously deployed, update of MS that wasn't deployed
	isDeploy, wasDeployedBefore := isDeployOrUpdateOfMs(msConfig)
	if isDeploy {
		//todo think about how to not store not necessary empty config records. can't just not add empty ms config,
		// because we can have v1 active with e1, candidate v2 without e1, no record of empty config, then ApplyMssInActiveButNotInCandidateForVhost will find it and add e1
		//if rabbitConfig.Spec.VersionedEntities == nil {
		//	return nil, err
		//}

		//deploy, no previous MS config
		log.InfoC(ctx, "Deploy of new config case")
		msConfig := &model.MsConfig{
			Namespace:        namespace,
			MsName:           serviceName,
			VhostID:          vhostId,
			CandidateVersion: candidateVersion,
			ActualVersion:    candidateVersion,
			Config:           rabbitConfig.Spec,
		}

		err := s.rabbitDao.InsertRabbitMsConfig(ctx, msConfig)
		if err != nil {
			log.ErrorC(ctx, "Error during InsertRabbitMsConfig for config '%v': %v", msConfig, err)
			return nil, err
		}

		err = addAllEntitiesFromRabbitConfigToDb(ctx, s, *msConfig, rabbitConfig)
		if err != nil {
			log.ErrorC(ctx, "Error during addAllEntitiesFromRabbitConfigToDb in deploy case: %v", err)
			return nil, err
		}
	} else {
		//update of previously deployed, actual_version == candidate_version or update of MS that wasn't deployed actual_version != candidate_version
		log.InfoC(ctx, "Update of existing config case")

		//versioned entities could be nil (or whole config)
		if rabbitConfig.Spec.VersionedEntities == nil {
			rabbitConfig.Spec.VersionedEntities = &model.RabbitEntities{}
		}

		var newExchangesNames, newQueuesNames []string
		var oldBindings []model.RabbitVersionedEntity

		newExchangesNames, err = ExtractNames(rabbitConfig.Spec.VersionedEntities.Exchanges)
		if err != nil {
			log.ErrorC(ctx, err.Error())
			return nil, err
		}
		oldExchanges, err := s.rabbitDao.GetRabbitVersEntitiesByMsConfigIdAndType(ctx, msConfig.Id, model.ExchangeType)
		if err != nil {
			log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByMsConfigIdAndType for exchange: %v", err)
			return nil, err
		}
		entitiesToBeDeleted = append(entitiesToBeDeleted, SubstractEntities(oldExchanges, newExchangesNames)...)

		newQueuesNames, err = ExtractNames(rabbitConfig.Spec.VersionedEntities.Queues)
		if err != nil {
			log.ErrorC(ctx, err.Error())
			return nil, err
		}
		oldQueues, err := s.rabbitDao.GetRabbitVersEntitiesByMsConfigIdAndType(ctx, msConfig.Id, model.QueueType)
		if err != nil {
			log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByMsConfigIdAndType for queue: %v", err)
			return nil, err
		}
		//queue should be deleted only when it is related to MS where active_version == candidate_version, otherwise active version queue would be deleted
		if wasDeployedBefore {
			entitiesToBeDeleted = append(entitiesToBeDeleted, SubstractEntities(oldQueues, newQueuesNames)...)
		}

		oldBindings, err = s.rabbitDao.GetRabbitVersEntitiesByMsConfigIdAndType(ctx, msConfig.Id, model.BindingType)
		if err != nil {
			log.ErrorC(ctx, "Error during GetRabbitVersEntitiesByMsConfigIdAndType for binding: %v", err)
			return nil, err
		}
		entitiesToBeDeleted = append(entitiesToBeDeleted, oldBindings...)
		log.InfoC(ctx, "Entities to be deleted: %+v", entitiesToBeDeleted)

		//clear all vers entities from db, so we don't need to compare changes in db
		err = s.rabbitDao.DeleteAllVersEntitiesByMsConfigId(ctx, msConfig.Id)
		if err != nil {
			log.ErrorC(ctx, "Error during DeleteAllVersEntitiesByMsConfigId for msConfig '%v': %v", msConfig, err)
			return nil, err
		}

		//add all fresh entities (even if they were the same)
		err = addAllEntitiesFromRabbitConfigToDb(ctx, s, *msConfig, rabbitConfig)
		if err != nil {
			log.ErrorC(ctx, "Error during addAllEntitiesFromRabbitConfigToDb in update case: %v", err)
			return nil, err
		}

		log.InfoC(ctx, "Update of ms config which was not deployed case")
		if !wasDeployedBefore {
			msConfig.ActualVersion = candidateVersion
			err = s.rabbitDao.UpdateMsConfigActualVersion(ctx, *msConfig)
			if err != nil {
				log.ErrorC(ctx, "Error during UpdateMsConfigActualVersion: %v", err)
				return nil, err
			}
		}
	}

	return entitiesToBeDeleted, err
}

// returns isDeploy, wasDeployedBefore, err. if isDeploy, then just deployed. if not isDeploy and not wasDeployedBefore then it is update case but ms is deployed during this candidate
func isDeployOrUpdateOfMs(msConfig *model.MsConfig) (bool, bool) {
	if msConfig == nil {
		return true, false
	} else if msConfig.CandidateVersion == msConfig.ActualVersion {
		return false, true
	} else {
		return false, false
	}
}

func addAllEntitiesFromRabbitConfigToDb(ctx context.Context, s *RabbitServiceImpl, msConfig model.MsConfig, rabbitConfig *model.RabbitConfigReqDto) error {
	if rabbitConfig.Spec.VersionedEntities != nil {
		for _, exchange := range rabbitConfig.Spec.VersionedEntities.Exchanges {
			name, err := utils.ExtractName(exchange)
			if err != nil {
				return utils.LogError(log, ctx, "Error during ExtractName for exchange '%v': %w", exchange, err)
			}
			versEnt := &model.RabbitVersionedEntity{
				MsConfigId:   msConfig.Id,
				EntityType:   model.ExchangeType.String(),
				EntityName:   name,
				ClientEntity: exchange.(map[string]interface{}),
			}
			err = s.rabbitDao.InsertRabbitVersionedEntity(ctx, versEnt)
			if err != nil {
				return utils.LogError(log, ctx, "Error during InsertRabbitVersionedEntity for exchange '%v': %w", versEnt, err)
			}
		}

		for _, queue := range rabbitConfig.Spec.VersionedEntities.Queues {
			name, err := utils.ExtractName(queue)
			if err != nil {
				return utils.LogError(log, ctx, "Error during ExtractName for queue '%v': %w", queue, err)
			}
			versEnt := &model.RabbitVersionedEntity{
				MsConfigId:   msConfig.Id,
				EntityType:   model.QueueType.String(),
				EntityName:   name,
				ClientEntity: queue.(map[string]interface{}),
			}
			err = s.rabbitDao.InsertRabbitVersionedEntity(ctx, versEnt)
			if err != nil {
				return utils.LogError(log, ctx, "Error during InsertRabbitVersionedEntity for queue '%v': %w", versEnt, err)
			}
		}

		for _, binding := range rabbitConfig.Spec.VersionedEntities.Bindings {
			source, destination, err := utils.ExtractSourceAndDestination(binding)
			if err != nil {
				return utils.LogError(log, ctx, "Error during ExtractSourceAndDestination for binding '%v': %w", binding, err)
			}

			versEnt := &model.RabbitVersionedEntity{
				MsConfigId:         msConfig.Id,
				EntityType:         model.BindingType.String(),
				BindingSource:      source,
				BindingDestination: destination,
				ClientEntity:       binding.(map[string]interface{}),
			}
			err = s.rabbitDao.InsertRabbitVersionedEntity(ctx, versEnt)
			if err != nil {
				return utils.LogError(log, ctx, "Error during InsertRabbitVersionedEntity for entity '%v': %w", versEnt, err)
			}
		}
	}
	return nil
}

func (s *RabbitServiceImpl) ApplyMssInActiveButNotInCandidateForVhost(ctx context.Context, vhost model.VHostRegistration, activeVersion string, candidateVersion string) error {
	msConfigs, err := s.rabbitDao.GetMsConfigsInActiveButNotInCandidateByVhost(ctx, vhost.Id, activeVersion, candidateVersion)
	if err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigsInActiveButNotInCandidateByVhost: %v", err)
		return err
	}
	log.InfoC(ctx, "Microservices that are not in candidate, but in active version for vhost with classifier '%v' are '%v'", vhost.Classifier, msConfigs)

	for _, msConfig := range msConfigs {
		msConfig.Id = 0
		//actual version is the same as in current active version MS - it could be even v1 for active v2 and candidate v3
		msConfig.CandidateVersion = candidateVersion
		err := s.rabbitDao.InsertRabbitMsConfig(ctx, &msConfig)
		if err != nil {
			log.ErrorC(ctx, "Error during InsertRabbitMsConfig for config '%v': %v", msConfig, err)
			return err
		}

		for _, ent := range msConfig.Entities {
			ent.Id = 0
			ent.MsConfigId = msConfig.Id
			//For Q it should not be created in MaaS, because it is only shows that current MS uses this entity, but it is owned by another MS
			err = s.rabbitDao.InsertRabbitVersionedEntity(ctx, ent)
			if err != nil {
				log.ErrorC(ctx, "Error during InsertRabbitVersionedEntity for entity '%v': %v", ent, err)
				return err
			}
		}
	}
	return nil
}

func (s *RabbitServiceImpl) CreateVersionedEntities(ctx context.Context, namespace string, candidateVersion string) (model.RabbitEntities, []model.UpdateStatus, error) {
	//created entities and update statuses as result values are not shown to user now in result, but could be implemented later
	log.InfoC(ctx, "Creating versioned entities in rabbit for namespace '%v' and candidateVersion '%v'", namespace, candidateVersion)
	result := model.RabbitEntities{}
	var updateStatus []model.UpdateStatus

	msConfigs, err := s.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(ctx, namespace, candidateVersion)
	if err != nil {
		log.ErrorC(ctx, "Error during GetMsConfigsByBgDomainAndCandidateVersion for namespace '%v' and version '%v': %v", namespace, candidateVersion, err)
		return result, nil, err
	}

	//bindings should be done at last order after Q and E are already created
	var bindingsToCreateLater []model.RabbitVersionedEntity
	for _, msConfig := range msConfigs {
		log.InfoC(ctx, "Creating versioned entities in rabbit for msConfig with name '%v' in vhost '%v' ", msConfig.MsName, msConfig.Vhost.Classifier)
		if msConfig.Entities == nil {
			continue
		}

		classifier, _ := model.ConvertToClassifier(msConfig.Vhost.Classifier)
		rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
		if err != nil {
			log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
			return result, nil, err
		}

		for _, ent := range msConfig.Entities {
			switch ent.EntityType {
			case model.ExchangeType.String():
				log.InfoC(ctx, "Creating versioned exchange in rabbit for entity: %+v", ent)
				exchange := ent.ClientEntity
				//contract: version router name is versioned exchange name without version
				versionRouterName, err := utils.ExtractName(exchange)
				if err != nil {
					log.ErrorC(ctx, "ExchangeType entity '%v' doesn't have name field", exchange)
					return result, nil, err
				}

				if versionRouterName == "" {
					log.ErrorC(ctx, "Name field in exchange should not be empty, exchange: %v", exchange)
					return result, nil, errors.New(fmt.Sprintf("Name field in exchange should not be empty, exchange: %v", exchange))
				}

				//check if such exchange exists, check if it is version router, if not - conflict, if not exists - create
				//3 options: not exists, exists and it is VR, exists and it is old rollout
				exists, isVersionRouter, err := s.checkExistenceOfVersionRouter(ctx, rabbitHelper, versionRouterName)
				if err != nil {
					return result, updateStatus, err
				}

				if exists && isVersionRouter {
					log.InfoC(ctx, "Version router with name '%v' already exists", versionRouterName)
				} else {
					if exists && !isVersionRouter {
						log.WarnC(ctx, "ExchangeType with such name exists, but it is not a version router (no specific alternate-exchange). It will be deleted and created as proper version router for migration. exchange: %v", exchange)
						_, err = rabbitHelper.DeleteExchange(ctx, exchange)
						if err != nil {
							return result, updateStatus, utils.LogError(log, ctx, "Error during deleting old exchange for version router creation: %w", err)
						}
					}

					//create vr
					log.InfoC(ctx, "Creating version router with name: %v", versionRouterName)
					err := s.createVersionRouterWithItsAE(ctx, rabbitHelper, versionRouterName)
					if err != nil {
						log.ErrorC(ctx, "Error during version router creation: %v", err)
						return result, updateStatus, err
					}
				}

				//creating versioned exchange (or updating)
				versionedExchangeName := getVersEntityNameByEntityNameAndVersion(versionRouterName, candidateVersion)
				versionedExchange := make(map[string]interface{})
				for key, value := range exchange {
					versionedExchange[key] = value
				}
				versionedExchange["name"] = versionedExchangeName

				log.InfoC(ctx, "Creating versioned exchange: %+v", versionedExchange)

				//todo not very beautiful design to get response as an array in arguments
				//adding rabbit response as RabbitEntity is a necessary step to save entity as it was created in Rabbit
				var tempExchange []interface{}
				err = createEntity(ctx, versionedExchange, rabbitHelper.CreateExchange, &tempExchange, &updateStatus)
				if err != nil {
					return result, nil, err
				}

				checkedTempExchange, ok := tempExchange[0].(*map[string]interface{})
				if !ok {
					err := fmt.Errorf("error during conversion temp exchange to *map[string]interface{} for '%+v'", tempExchange[0])
					log.ErrorC(ctx, err.Error())
					return result, nil, err
				}
				ent.RabbitEntity = *checkedTempExchange
				err = s.rabbitDao.UpdateRabbitVersionedEntity(ctx, *ent)
				if err != nil {
					return result, nil, err
				}

				result.Exchanges = append(result.Exchanges, tempExchange...)

				//bind vr to new versioned exchange
				veBindingArguments := map[string]interface{}{
					"version": candidateVersion,
				}
				veBinding := map[string]interface{}{
					"source":      versionRouterName,
					"destination": versionedExchangeName,
					"arguments":   veBindingArguments,
					"routing_key": "*",
				}
				err = createEntity(ctx, veBinding, rabbitHelper.CreateExchangeBinding, nil, nil)
				if err != nil {
					log.ErrorC(ctx, "Error during version router to versioned exchange binding creation: %v, err: %v", veBinding, err)
					return result, updateStatus, err
				}

				if candidateVersion == model.INITIAL_VERSION {
					aeName := getAltExchNameByVersionRouter(versionRouterName)

					//create new binding with new version
					log.InfoC(ctx, "Creating default route for version router: '%v' with ae: '%v' with version: '%v'", versionRouterName, aeName, candidateVersion)
					//bind ae to new versioned exchange
					aeBinding := map[string]interface{}{
						"source":      aeName,
						"destination": getVersEntityNameByEntityNameAndVersion(versionRouterName, candidateVersion),
					}
					err = createEntity(ctx, aeBinding, rabbitHelper.CreateExchangeBinding, nil, nil)
					if err != nil {
						log.ErrorC(ctx, fmt.Sprintf("Error during creating default route for version router: '%v' with ae: '%v' with version: '%v'", versionRouterName, aeName, candidateVersion))
						return result, updateStatus, err
					}
				}

			case model.QueueType.String():
				//we should not create Q if it is just row to know that MS uses this queue, but not owns
				if msConfig.CandidateVersion != msConfig.ActualVersion {
					log.InfoC(ctx, "Skipping queue creation, because is is not actual version: %v", ent)
					continue
				}

				log.InfoC(ctx, "Creating versioned queue in rabbit for entity: %+v", ent)
				queue := ent.ClientEntity

				//creating versioned queue (or updating)
				//is not nil, because did a validation before
				queueName, err := utils.ExtractName(queue)
				if err != nil {
					log.ErrorC(ctx, "QueueType entity '%v' doesn't have name field", queue)
					return result, nil, err
				}

				versionedQueue := make(map[string]interface{})
				for key, value := range queue {
					versionedQueue[key] = value
				}
				versionedQueue["name"] = getVersEntityNameByEntityNameAndVersion(queueName, candidateVersion)

				//todo not very beautiful design to get response as an array in arguments
				//adding rabbit response as RabbitEntity is a necessary step to save entity as it was created in Rabbit
				var tempQueue []interface{}
				err = createEntity(ctx, versionedQueue, rabbitHelper.CreateQueue, &tempQueue, &updateStatus)
				if err != nil {
					return result, nil, err
				}

				checkedTempQueue, ok := tempQueue[0].(*map[string]interface{})
				if !ok {
					err := fmt.Errorf("error during conversion temp queue to *map[string]interface{} for '%+v'", tempQueue[0])
					log.ErrorC(ctx, err.Error())
					return result, nil, err
				}
				ent.RabbitEntity = *checkedTempQueue
				err = s.rabbitDao.UpdateRabbitVersionedEntity(ctx, *ent)
				if err != nil {
					return result, nil, err
				}

				result.Queues = append(result.Queues, tempQueue...)

			case model.BindingType.String():
				bingingMsConfig := msConfig
				binding := *ent
				binding.MsConfig = &bingingMsConfig
				bindingsToCreateLater = append(bindingsToCreateLater, binding)
			}
		}
	}

	for _, bindingToCreateLater := range bindingsToCreateLater {
		log.InfoC(ctx, "Creating versioned binding in rabbit for entity: %+v", bindingToCreateLater)
		binding := bindingToCreateLater.ClientEntity
		//creating versioned binding
		sourceName, destinationName, err := utils.ExtractSourceAndDestination(binding)
		if err != nil {
			log.ErrorC(ctx, "binding entity %v doesn't have either source or destination field", binding)
			return result, nil, err
		}

		versionedBinding := make(map[string]interface{})
		for key, value := range binding {
			versionedBinding[key] = value
		}

		versionedBinding["source"] = getVersEntityNameByEntityNameAndVersion(sourceName, bindingToCreateLater.MsConfig.CandidateVersion)
		versionedBinding["destination"] = getVersEntityNameByEntityNameAndVersion(destinationName, bindingToCreateLater.MsConfig.ActualVersion)

		classifier, _ := model.ConvertToClassifier(bindingToCreateLater.MsConfig.Vhost.Classifier)
		rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
		if err != nil {
			log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
			return result, nil, err
		}

		createdBinding, err := rabbitHelper.CreateNormalOrLazyBinding(ctx, versionedBinding)
		if err != nil {
			log.ErrorC(ctx, "error duringCreateNormalOrLazyBinding while in CreateVersionedEntities: %v", err)
			return result, nil, err
		}

		//createdBinding will be nil if it is lazy binding and was not created in rabbit, so no update and adding to response
		if createdBinding != nil {
			bindingToCreateLater.RabbitEntity = createdBinding
			err = s.rabbitDao.UpdateRabbitVersionedEntity(ctx, bindingToCreateLater)
			if err != nil {
				return result, nil, err
			}
			result.Bindings = append(result.Bindings, createdBinding)
		}

	}

	return result, updateStatus, nil
}

func (s *RabbitServiceImpl) ChangeVersionRoutersActiveVersion(ctx context.Context, classifier model.Classifier, version string) error {
	log.InfoC(ctx, "Changing active version to '%v' for vhost with classifier '%v'", version, classifier)

	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
		return err
	}

	exchanges, err := rabbitHelper.GetAllExchanges(ctx)
	if err != nil {
		log.ErrorC(ctx, "Error during GetAllExchanges: %v", err.Error())
		return err
	}

	for _, exchange := range exchanges {
		exchangeName := exchange.(map[string]interface{})["name"].(string)
		const systemExchangesPrefix = "amq."
		if exchangeName == "" || strings.HasPrefix(exchangeName, systemExchangesPrefix) {
			log.DebugC(ctx, "Skipping exchanges with empty name and starting with 'amq.'")
			continue
		}

		exists, isVersionRouter, err := s.checkExistenceOfVersionRouter(ctx, rabbitHelper, exchangeName)
		if err != nil {
			log.ErrorC(ctx, "Error during cheching existence of version router: %v", err)
			return err
		}

		if exists {
			if !isVersionRouter {
				log.DebugC(ctx, "ExchangeType '%v' is not a version router", exchangeName)
				continue
			}
		}
		log.InfoC(ctx, "Changing active route for version router: %v", exchangeName)

		aeName := getAltExchNameByVersionRouter(exchangeName)

		//create new binding with new version
		log.InfoC(ctx, "Changing default route for version router: '%v' with ae: '%v' with version: '%v'", exchangeName, aeName, version)
		versionedExchangeName := getVersEntityNameByEntityNameAndVersion(exchangeName, version)
		//bind ae to new versioned exchange
		aeBinding := map[string]interface{}{
			"source":      aeName,
			"destination": versionedExchangeName,
		}

		//part of the design is that binding first created then deleted
		err = createEntity(ctx, aeBinding, rabbitHelper.CreateExchangeBinding, nil, nil)
		if err != nil {
			rabbitErr, ok := err.(*helper.RabbitHttpError)
			if !ok || rabbitErr.Code != http.StatusNotFound {
				err = fmt.Errorf("error during creating ae binding from ae with name '%v' to versioned exchange with name '%v', err: %w", aeName, versionedExchangeName, err)
				log.ErrorC(ctx, err.Error())
				return err
			} else {
				//no versioned exchange for such version
				log.WarnC(ctx, fmt.Sprintf("Default route for versioned exchange '%v' was not created, there is no versioned exchange for version '%v'", exchangeName, version))
			}
		}

		//get bindings, should be new current and old previous
		bindings, err := rabbitHelper.GetExchangeSourceBindings(ctx, map[string]interface{}{"name": aeName})
		if err != nil {
			log.ErrorC(ctx, "Error during getting exchange source bindings: %v", err)
			return err
		}

		//delete old binding
		for _, binding := range bindings {
			bingingMap := binding.(map[string]interface{})
			if bingingMap["destination"].(string) == versionedExchangeName {
				continue
			}

			log.DebugC(ctx, fmt.Sprintf("Deleting old version binding: %v", binding))
			deletedBinding, err := rabbitHelper.DeleteExchangeBinding(ctx, binding)
			if err != nil {
				log.ErrorC(ctx, "Error during deleting exchange source binding %+v: %v", binding, err)
				return err
			}
			if deletedBinding == nil {
				return errors.New(fmt.Sprintf("Logic error: binding was found but not deleted: %v", binding))
			}
		}

	}
	return nil
}

func (s *RabbitServiceImpl) ChangeExportedExchangesActiveVersionBg2(ctx context.Context, classifier model.Classifier, activeNamespace string) error {
	log.InfoC(ctx, "Changing active version to activeNamespace '%v' for vhost with classifier '%v' for bg2", activeNamespace, classifier)

	rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &classifier)
	if err != nil {
		log.ErrorC(ctx, "didn't manage to get rabbit helper: %v", err)
		return err
	}

	exchanges, err := rabbitHelper.GetAllExchanges(ctx)
	if err != nil {
		log.ErrorC(ctx, "Error during GetAllExchanges: %v", err.Error())
		return err
	}

	for _, exchange := range exchanges {
		exchangeName := exchange.(map[string]interface{})["name"].(string)
		const systemExchangesPrefix = "amq."
		if exchangeName == "" || strings.HasPrefix(exchangeName, systemExchangesPrefix) {
			log.DebugC(ctx, "Skipping exchanges with empty name and starting with 'amq.'")
			continue
		}

		exists, isVersionRouter, err := s.checkExistenceOfVersionRouter(ctx, rabbitHelper, exchangeName)
		if err != nil {
			log.ErrorC(ctx, "Error during cheching existence of version router: %v", err)
			return err
		}

		if exists {
			if !isVersionRouter {
				log.DebugC(ctx, "ExchangeType '%v' is not a version router", exchangeName)
				continue
			}
		}
		log.InfoC(ctx, "Changing active route for version router: %v", exchangeName)

		aeName := getAltExchNameByVersionRouter(exchangeName)

		//create new binding with new version
		log.InfoC(ctx, "Changing default route for version router: '%v' with ae: '%v' with activeNamespace: '%v'", exchangeName, aeName, activeNamespace)
		versionedExchangeName := getBg2ShovelQueueNameByNameAndNamespace(exchangeName, activeNamespace)
		//bind ae to new versioned exchange
		aeBinding := map[string]interface{}{
			"source":      aeName,
			"destination": versionedExchangeName,
		}

		//part of the design is that binding first created then deleted
		err = createEntity(ctx, aeBinding, rabbitHelper.CreateBinding, nil, nil)
		if err != nil {
			rabbitErr, ok := err.(helper.RabbitHelperError)
			if !ok || errors.Is(rabbitErr, helper.ErrNotFound) {
				err = fmt.Errorf("error during creating ae binding from ae with name '%v' to versioned queue with name '%v', err: %w", aeName, versionedExchangeName, err)
				log.ErrorC(ctx, err.Error())
				return err
			} else {
				//no versioned exchange for such version
				log.WarnC(ctx, fmt.Sprintf("Default route for versioned exchange '%v' was not created, there is no versioned exchange for activeNamespace '%v'", exchangeName, activeNamespace))
			}
		}

		//get bindings, should be new current and old previous
		bindings, err := rabbitHelper.GetExchangeSourceBindings(ctx, map[string]interface{}{"name": aeName})
		if err != nil {
			log.ErrorC(ctx, "Error during getting exchange source bindings: %v", err)
			return err
		}

		//delete old binding
		for _, binding := range bindings {
			bingingMap := binding.(map[string]interface{})
			if bingingMap["destination"].(string) == versionedExchangeName {
				continue
			}

			log.DebugC(ctx, fmt.Sprintf("Deleting old version binding: %v", binding))
			deletedBinding, err := rabbitHelper.DeleteBinding(ctx, binding)
			if err != nil {
				log.ErrorC(ctx, "Error during deleting exchange source binding %+v: %v", binding, err)
				return err
			}
			if deletedBinding == nil {
				return errors.New(fmt.Sprintf("Logic error: binding was found but not deleted: %v", binding))
			}
		}

	}
	return nil
}

func (s *RabbitServiceImpl) RabbitBgValidation(ctx context.Context, namespace string, candidateVersion string) error {
	vhosts, err := s.rabbitDao.FindVhostsByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error getting vhost by namespace: %v: %w", namespace, err)
	}

	for _, vhost := range vhosts {
		log.InfoC(ctx, "Starting validation of entities for vhost with classifier '%v'", vhost.Classifier)

		//E and Q have non-empty and unique name (non-emptiness is checked during ApplyMsConfigAndVersionedEntitiesToDb when inserting in DB)
		err = s.rabbitDao.CheckExchangesAndQueuesHaveUniqueName(ctx, vhost.Id, candidateVersion)
		if err != nil {
			return utils.LogError(log, ctx, "error check uniqueness: %w", err)
		}

		//No Q without bindings (just some possible error, because has no meaning)
		err = s.rabbitDao.CheckNoQueuesWithoutBinding(ctx, vhost.Id, candidateVersion)
		if err != nil {
			return utils.LogError(log, ctx, "check non-bound queues: %w", err)
		}

		//B have non-empty and existing E-source
		incorrectBindings, err := s.rabbitDao.CheckBindingsHaveExistingESource(ctx, vhost.Id, candidateVersion)
		if len(incorrectBindings) != 0 {
			//since PDSDNREQ-6232 lazy bindings were introduced, exchange existence is not mandatory
			msg := fmt.Sprintf("validation warning - these bindings have non-existing exchange source within vhost '%v' and candidateVersion '%v' :", (incorrectBindings)[0].MsConfig.Vhost.Classifier, candidateVersion)
			for _, ent := range incorrectBindings {
				newLine := fmt.Sprintf(" binding source '%v', binding destination '%v', ms name '%v'; ", ent.BindingSource, ent.BindingDestination, ent.MsConfig.MsName)
				msg = msg + "\n" + newLine
			}

			log.WarnC(ctx, "Some bindings have no declared exchanges, it either could be error or lazy bindings: %v", msg)
		}

		//B have non-empty and existing Q-destination
		err = s.rabbitDao.CheckBindingsHaveExistingQDestination(ctx, vhost.Id, candidateVersion)
		if err != nil {
			return utils.LogError(log, ctx, "check binding targets: %w", err)
		}

		//B declared in the same ms as Q
		err = s.rabbitDao.CheckBindingDeclaredInSameMsAsQ(ctx, vhost.Id, candidateVersion)
		if err != nil {
			return utils.LogError(log, ctx, "check bindings declaration bounds: %w", err)
		}
	}
	return nil
}

func (s *RabbitServiceImpl) GetLazyBindings(ctx context.Context, namespace string) ([]model.LazyBindingDto, error) {
	lazyBindings, err := s.rabbitDao.GetRabbitLazyBindings(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during GetRabbitLazyBindings while in ValidateRabbitConfigs rabbit service: %v", err)
		return nil, err
	}
	return lazyBindings, nil
}

func (s *RabbitServiceImpl) RecoverVhostsByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Starting to recover vhosts by namespace: %v", namespace)
	vhosts, err := s.FindVhostsByNamespace(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during FindVhostsByNamespace while in RecoverVhostsByNamespace for namespace: %v. Error: %v ", namespace, err.Error())
		return err
	}
	for _, vhost := range vhosts {
		classifier, err := model.ConvertToClassifier(vhost.Classifier)
		if err != nil {
			log.ErrorC(ctx, "Error during ConvertToClassifier while in RecoverVhostsByNamespace rabbit service for vhost: %v, err: %v", vhost.Vhost, err)
			return err
		}

		rabbitInstance, err := s.instanceService.GetById(ctx, vhost.InstanceId)
		if err != nil {
			log.ErrorC(ctx, "Error during FindRabbitInstanceById while in RecoverVhostsByNamespace rabbit service for vhost: %v, err: %v", vhost.Vhost, err)
			return err
		}

		rabbitHelper, err := s.getRabbitHelper(ctx, s, rabbitInstance, &vhost, &classifier)
		if err != nil {
			log.ErrorC(ctx, "Error during getRabbitHelper while in RecoverVhostsByNamespace rabbit service for vhost: %v, err: %v", vhost.Vhost, err)
			return err
		}

		code, err := rabbitHelper.CreateVHostAndReturnStatus(ctx)
		if err != nil {
			log.ErrorC(ctx, "Error during CreateVHost while in RecoverVhostsByNamespace rabbit service for vhost: %v, err: %v", vhost.Vhost, err)
			return err
		}
		if code == http.StatusCreated {
			log.WarnC(ctx, "Vhost didn't exist in rabbit and it was created for vhost: %v, err: %v", vhost.Vhost, err)
		}

		log.InfoC(ctx, "Starting to recover vhost: %v", vhost.Vhost)
		entitiesFromDb, err := s.rabbitDao.GetRabbitEntitiesByVhostId(ctx, vhost.Id)
		if err != nil {
			log.ErrorC(ctx, "Error during GetRabbitEntitiesByVhostId while in RecoverVhostsByNamespace rabbit service for vhost: %v, err: %v", vhost.Vhost, err)
			return err
		}

		var rabbitEntities model.RabbitEntities
		for _, ent := range entitiesFromDb {
			switch ent.EntityType {
			case model.ExchangeType.String():
				rabbitEntities.Exchanges = append(rabbitEntities.Exchanges, ent.ClientEntity)
			case model.QueueType.String():
				rabbitEntities.Queues = append(rabbitEntities.Queues, ent.ClientEntity)
			case model.BindingType.String():
				rabbitEntities.Bindings = append(rabbitEntities.Bindings, ent.ClientEntity)
			}
		}

		_, _, err = s.CreateOrUpdateEntitiesV1(ctx, &vhost, rabbitEntities)
		if err != nil {
			log.ErrorC(ctx, "Error during CreateOrUpdateEntitiesV1 while in RecoverVhostsByNamespace rabbit service for classifier: %v, err: %v", classifier, err)
			return err
		}
	}

	log.InfoC(ctx, "Starting to recover rabbit versioned entities by namespace: %v", namespace)
	//restore entities for all versions of namespace
	bgStatus, err := s.bgService.GetBgStatusByNamespace(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during GetBgStatusByNamespace while in RecoverVhostsByNamespace rabbit service: %v", err)
		return err
	}

	if bgStatus != nil {
		var versions []string
		if bgStatus.Active != "" {
			versions = append(versions, bgStatus.Active)
		}
		if bgStatus.Legacy != "" {
			versions = append(versions, bgStatus.Legacy)
		}
		versions = append(versions, bgStatus.Candidates...)

		for _, version := range versions {
			_, _, err = s.CreateVersionedEntities(ctx, namespace, version)
			if err != nil {
				log.ErrorC(ctx, "Error during CreateVersionedEntities for version '%v' while in RecoverVhostsByNamespace rabbit service: %v", version, err)
				return err
			}
		}

	}

	return nil
}

func (r RabbitServiceImpl) DestroyDomain(ctx context.Context, namespaces *domain.BGNamespaces) error {
	check := func(ns string) error {
		if registrations, err := r.rabbitDao.FindVhostsByNamespace(ctx, ns); err == nil {
			if len(registrations) > 0 {
				return utils.LogError(log, ctx, "namespace is not empty: found vhosts in namespace `%v': %w", ns, msg.Conflict)
			}
		} else {
			return utils.LogError(log, ctx, "error getting vhosts for `%v': %w", ns, err)
		}
		return nil
	}

	if err := check(namespaces.Origin); err != nil {
		return err
	}
	return check(namespaces.Peer)
}

func (s *RabbitServiceImpl) RotatePasswords(ctx context.Context, searchForm *model.SearchForm) ([]model.VHostRegistration, error) {
	if searchForm.IsEmpty() {
		return nil, utils.LogError(log, ctx, "remove vhost clause can't be empty while in RotatePasswords: %w", msg.BadRequest)
	}

	securityContext := model.SecurityContextOf(ctx)
	if !securityContext.UserHasRoles(model.ManagerRole) && searchForm.Namespace == "" {
		log.WarnC(ctx, "Restrict search criteria for user with Agent role to namespaced while in RotatePasswords: %v", model.RequestContextOf(ctx).Namespace)
		searchForm.Namespace = model.RequestContextOf(ctx).Namespace
	}

	log.InfoC(ctx, "RotatePasswords for vhosts with search form '%+v'", searchForm)
	vhosts, err := s.rabbitDao.FindVhostWithSearchForm(ctx, searchForm)
	if err != nil {
		return nil, utils.LogError(log, ctx, "Error occurred while searching for vhosts in database while in RotatePasswords: '%w'", err)
	}

	if len(vhosts) == 0 {
		return nil, utils.LogError(log, ctx, "no vhost registrations matched search were found while in RotatePasswords: %w", msg.NotFound)
	}

	var rotatedVhosts []model.VHostRegistration
	for _, vhost := range vhosts {
		log.InfoC(ctx, "Preparing to rotate password for vhost with classifier: %v", vhost.Classifier)

		rabbitInstance, err := s.LoadRabbitInstance(ctx, &vhost)
		if err != nil {
			return nil, utils.LogError(log, ctx, "error during LoadRabbitInstance while in RotatePasswords: %w", err)
		}

		classifier, err := model.ConvertToClassifier(vhost.Classifier)
		if err != nil {
			return nil, utils.LogError(log, ctx, "error during ConvertToClassifier while in RotatePasswords: %w", err)
		}

		rabbitHelper, err := s.getRabbitHelper(ctx, s, rabbitInstance, &vhost, &classifier)
		if err != nil {
			return nil, utils.LogError(log, ctx, "didn't manage to get rabbit helper while in RotatePasswords: %w", err)
		}

		newPassword := utils.CompactUuid()
		vhost.Password = utils.SecurePassword(newPassword)
		err = s.rabbitDao.UpdateVhostRegistration(ctx, &vhost, func(reg *model.VHostRegistration) error {
			return rabbitHelper.CreateOrUpdateUser(ctx, vhost.User, newPassword)
		})

		if err != nil {
			return nil, utils.LogError(log, ctx, "error during UpdateVhostRegistration while in RotatePasswords: %w", err)
		}

		vhost.Password = "***"
		rotatedVhosts = append(rotatedVhosts, vhost)

		log.InfoC(ctx, "Successfully rotated vhost with classifier: %v", vhost.Classifier)
	}

	return rotatedVhosts, nil
}

func (s *RabbitServiceImpl) ProcessExportedVhost(ctx context.Context, namespace string) error {

	bgState, err := s.bgDomainService.GetCurrentBgStateByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "Error during GetCurrentBgStateByNamespace while in ProcessExportedVhost: %w", err)
	}

	var namespaces []string
	if bgState != nil {
		namespaces = append(namespaces, bgState.Origin.Name)
		namespaces = append(namespaces, bgState.Peer.Name)
	} else {
		namespaces = append(namespaces, namespace)
	}

	vhosts, err := s.FindVhostsByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "Error during FindVhostsByNamespace while in ProcessExportedVhost: %w", err)
	}

	//checking every vhost in origin (or single) namespace
	for _, vhost := range vhosts {
		classifier, _ := model.ConvertToClassifier(vhost.Classifier)

		if strings.HasSuffix(classifier.Name, "-exported") {
			continue
		}
		vhostInstance := vhost.InstanceId

		var exportedQueues []model.RabbitEntity
		var exportedQueuesEntities = make(map[string]model.Queue)
		var exportedQueuesVhosts = make(map[string][]model.VHostRegistration)

		var exportedExchanges []model.RabbitEntity
		var exportedExchangesEntities = make(map[string]model.Exchange)
		var exportedExchangeVhostsAndVersion = make(map[string][]model.VhostAndVersion)

		var exportedClassifier model.Classifier

		var activeNamespace = namespace

		if bgState != nil {
			exportedClassifier = model.Classifier{
				Name:      classifier.Name + "-exported",
				Namespace: bgState.Origin.Name,
			}
		} else {
			exportedClassifier = model.Classifier{
				Name:      classifier.Name + "-exported",
				Namespace: namespace,
			}
		}

		//in case of bg2 we have origin and peer, if no bg2 - only single namespace
		for _, namespace := range namespaces {
			classifier.Namespace = namespace

			vhost, err := s.rabbitDao.FindVhostByClassifier(ctx, classifier)
			if err != nil {
				return utils.LogError(log, ctx, "Error during FindVhostsByNamespace while in ProcessExportedVhost: %w", err)
			}

			//in case of commit and vhost doesn't exist
			if vhost == nil {
				continue
			}

			entities, err := s.rabbitDao.GetRabbitEntitiesByVhostId(ctx, vhost.Id)
			if err != nil {
				return utils.LogError(log, ctx, "Error during GetRabbitEntitiesByVhostId while in ProcessExportedVhost: %w", err)
			}

			for _, entity := range entities {
				if entity.ClientEntity["exported"] != true {
					continue
				}

				switch entity.EntityType {
				case model.QueueType.String():
					exportedQueues = append(exportedQueues, entity)
					exportedQueuesEntities[entity.EntityName.String] = entity.ClientEntity
					exportedQueuesVhosts[entity.EntityName.String] = append(exportedQueuesVhosts[entity.EntityName.String], *vhost)
				case model.ExchangeType.String():
					exportedExchanges = append(exportedExchanges, entity)
					exportedExchangesEntities[entity.EntityName.String] = entity.ClientEntity

					var version string
					if bgState != nil {
						if namespace == bgState.Origin.Name {
							version = string(bgState.Origin.Version)
						} else {
							version = string(bgState.Peer.Version)
						}

						if bgState.Origin.State == "active" {
							activeNamespace = bgState.Origin.Name
						} else {
							activeNamespace = bgState.Peer.Name
						}
					} else {
						version = "v1"
					}

					exportedExchangeVhostsAndVersion[entity.EntityName.String] = append(exportedExchangeVhostsAndVersion[entity.EntityName.String], model.VhostAndVersion{Vhost: *vhost, Version: version})
				default:
					continue
				}
			}
		}

		//no GetOrCreateVhost, because no need to create if no exportedQueues
		exportedVhost, err := s.rabbitDao.FindVhostByClassifier(ctx, exportedClassifier)
		if err != nil {
			return utils.LogError(log, ctx, "Error during FindVhostByClassifier exported while in ProcessExportedVhost: %w", err)
		}

		//exported vhost exists but now no queues are exported, so if exportedQueuesEntities exist then need to delete existing queues later
		if exportedVhost == nil && len(exportedQueuesEntities) == 0 && len(exportedExchangesEntities) == 0 {
			log.InfoC(ctx, "No exported queues and exchanges for namespaces '%v' and no exported vhost exists, skipping creation")
			continue
		}

		if exportedVhost == nil {
			//create exported vhost
			_, exportedVhost, err = s.getOrCreateVhostInternal(ctx, vhostInstance, &exportedClassifier, nil)
			if err != nil {
				return utils.LogError(log, ctx, "Error during GetOrCreateVhost exported while in ProcessExportedVhost: %w", err)
			}
		}

		rabbitHelper, err := s.getRabbitHelper(ctx, s, nil, nil, &exportedClassifier)
		if err != nil {
			return utils.LogError(log, ctx, "Error during getRabbitHelper exported while in ProcessExportedVhost: %w", err)
		}

		//delete all existing shovels for exported vhost
		err = rabbitHelper.DeleteShovelsForExportedVhost(ctx)
		if err != nil {
			return utils.LogError(log, ctx, "Error during DeleteShovelsForExportedVhost while in ProcessExportedVhost: %w", err)
		}

		//process queues
		//create queues and shovel
		for queueName, queue := range exportedQueuesEntities {
			_, _, err := rabbitHelper.CreateQueue(ctx, queue)
			if err != nil {
				return utils.LogError(log, ctx, "Error during CreateQueue exported while in ProcessExportedVhost: %w", err)
			}

			err = rabbitHelper.CreateShovelForExportedQueue(ctx, exportedQueuesVhosts[queueName], queue)
			if err != nil {
				return utils.LogError(log, ctx, "Error during CreateShovelForExportedQueue while in ProcessExportedVhost: %w", err)
			}
		}

		//delete queues that are in exported vhost but not in exportedQueuesEntities
		queuesInVhost, err := rabbitHelper.GetVhostQueues(ctx)
		if err != nil {
			return utils.LogError(log, ctx, "Error during GetVhostQueues while in ProcessExportedVhost: %w", err)
		}

		for _, queueInVhost := range queuesInVhost {
			queueName := queueInVhost["name"].(string)
			if strings.HasSuffix(queueName, shovelQueue) {
				continue
			}
			_, exists := exportedQueuesEntities[queueName]
			if !exists {
				_, err := rabbitHelper.DeleteQueue(ctx, queueInVhost)
				if err != nil {
					return utils.LogError(log, ctx, "Error during DeleteQueue exported while in ProcessExportedVhost: %w", err)
				}
			}
		}

		//process exchanges
		//create exchange and shovel
		for exchangeName, exchange := range exportedExchangesEntities {
			err = s.createVersionRouterWithItsAE(ctx, rabbitHelper, exchangeName)
			if err != nil {
				return utils.LogError(log, ctx, "Error during createVersionRouterWithItsAE exported while in ProcessExportedVhost: %w", err)
			}

			err = rabbitHelper.CreateQueuesAndShovelsForExportedExchange(ctx, exportedExchangeVhostsAndVersion[exchangeName], exchange)
			if err != nil {
				return utils.LogError(log, ctx, "Error during CreateShovelForExportedQueue while in ProcessExportedVhost: %w", err)
			}

			//bind ae
			aeName := getAltExchNameByVersionRouter(exchangeName)

			aeBinding := map[string]interface{}{
				"source":      aeName,
				"destination": getBg2ShovelQueueNameByNameAndNamespace(exchangeName, activeNamespace),
			}
			err = createEntity(ctx, aeBinding, rabbitHelper.CreateBinding, nil, nil)
			if err != nil {
				var helperErr helper.RabbitHelperError
				if errors.As(err, &helperErr) {
					switch helperErr.Err {
					case helper.ErrNotFound:
						log.InfoC(ctx, "no active version for exchangeName: %v", exchangeName)
					default:
						return utils.LogError(log, ctx, "RabbitHttpError: Error during creating ae binding while in ProcessExportedVhost: %w", err)
					}
				} else {
					return utils.LogError(log, ctx, "Error during creating ae binding while in ProcessExportedVhost: %w", err)
				}
			}
		}

		//delete exchanges that are in exported vhost but not in exportedQueuesEntities
		exchangesInVhost, err := rabbitHelper.GetVhostExchanges(ctx)
		if err != nil {
			return utils.LogError(log, ctx, "Error during GetVhostExchanges while in ProcessExportedVhost: %w", err)
		}

		for _, exchangeInVhost := range exchangesInVhost {
			exchName := exchangeInVhost["name"].(string)

			const systemExchangesPrefix = "amq."
			if exchName == "" || strings.HasPrefix(exchName, systemExchangesPrefix) || strings.HasSuffix(exchName, "-ae") {
				log.DebugC(ctx, "Skipping exchanges with empty name and starting with 'amq.'")
				continue
			}

			var queuesToDelete []model.Queue
			vhostAndVersion, exists := exportedExchangeVhostsAndVersion[exchName]
			if !exists {
				_, err := rabbitHelper.DeleteExchange(ctx, exchangeInVhost)
				if err != nil {
					return utils.LogError(log, ctx, "Error during DeleteExchange exported while in ProcessExportedVhost: %w", err)
				}

				aeName := getAltExchNameByVersionRouter(exchName)
				_, err = rabbitHelper.DeleteExchange(ctx, map[string]interface{}{"name": aeName})
				if err != nil {
					return utils.LogError(log, ctx, "Error during DeleteExchange ae while in ProcessExportedVhost: %w", err)
				}

				for _, queueInVhost := range queuesInVhost {
					queueName := queueInVhost["name"].(string)
					if strings.HasPrefix(queueName, exchName) && strings.HasSuffix(queueName, shovelQueue) {
						queuesToDelete = append(queuesToDelete, queueInVhost)
					}
				}
			} else {
				if len(vhostAndVersion) == 1 { //only one namespace has exported vhost, need to delete second queue if exists
					for _, queueInVhost := range queuesInVhost {
						queueName := queueInVhost["name"].(string)
						if strings.HasPrefix(queueName, exchName) && strings.HasSuffix(queueName, shovelQueue) && !strings.Contains(queueName, "-"+vhostAndVersion[0].Vhost.Namespace+"-") {
							queuesToDelete = append(queuesToDelete, queueInVhost)
						}
					}
				}
			}

			for _, queue := range queuesToDelete {
				_, err := rabbitHelper.DeleteQueue(ctx, queue)
				if err != nil {
					return utils.LogError(log, ctx, "Error during DeleteQueue of exchange while in ProcessExportedVhost: %w", err)
				}
			}
		}

	}
	return nil
}

// returns exists if there is exchange with such name, isVersionRouter if it has proper ae, err
func (s *RabbitServiceImpl) checkExistenceOfVersionRouter(ctx context.Context, rabbitHelper helper.RabbitHelper, versionRouter string) (bool, bool, error) {
	exch, err := rabbitHelper.GetExchange(ctx, map[string]interface{}{
		"name": versionRouter,
	})
	if err != nil {
		return false, false, err
	}
	if exch != nil {
		ae, err := rabbitHelper.GetExchange(ctx, map[string]interface{}{
			"name": getAltExchNameByVersionRouter(versionRouter),
		})
		if err != nil {
			return false, false, err
		}
		if ae != nil {
			argumentsMap, ok := (*exch.(*map[string]interface{}))["arguments"].(map[string]interface{})
			if !ok {
				err := fmt.Errorf("error during conversion exch to (*exch.(*map[string]interface{}))[\"arguments\"].(map[string]interface{}) for '%+v'", exch)
				log.ErrorC(ctx, err.Error())
				return false, false, err
			}
			if argumentsMap["alternate-exchange"] != getAltExchNameByVersionRouter(versionRouter) && argumentsMap["blue-green"] != "true" {
				log.ErrorC(ctx, "Conflict in rabbit - alternate exchange exists, but not set as argument to version router with name: %v", versionRouter)
				return false, false, errors.New(fmt.Sprintf("Conflict in rabbit - alternate exchange exists, but not set as argument to version router with name: %v", versionRouter))
			}
			//exists, isVersionRouter
			return true, true, nil
		} else {
			//exists, not isVersionRouter
			return true, false, nil
		}
	} else {
		//not exists
		return false, false, nil
	}
}

func (s *RabbitServiceImpl) MatchDesignator(ctx context.Context, classifier model.Classifier, designator *model.InstanceDesignatorRabbit) (*model.RabbitInstance, bool, model.ClassifierMatch) {
	instance, matched, matchedBy := instance.MatchDesignator(ctx, classifier, designator, func(namespace string) string {
		domainNamespace, err := s.bgDomainService.FindByNamespace(ctx, namespace)
		if err != nil {
			log.ErrorC(ctx, "can not get domain namespace for '%s': %w", namespace, err)
			return namespace
		}
		if domainNamespace == nil {
			log.ErrorC(ctx, "no domain namespace for '%s' has been found", namespace)
			return namespace
		}
		return domainNamespace.Origin
	})
	var rabbitInstance *model.RabbitInstance
	if instance != nil {
		rabbitInstance = instance.(*model.RabbitInstance)
	}
	return rabbitInstance, matched, matchedBy
}

// contract: ae ends with -ae ALTERNATE_EXCHANGE_SUFFIX const
func getAltExchNameByVersionRouter(versionRouter string) string {
	return fmt.Sprintf("%v-%v", versionRouter, model.ALTERNATE_EXCHANGE_SUFFIX)
}

// contract: versioned exchange ends with -v1, -v2, ...
func getVersEntityNameByEntityNameAndVersion(entName string, version string) string {
	return fmt.Sprintf("%v-%v", entName, version)
}

func getBg2ShovelQueueNameByNameAndNamespace(entName string, namespace string) string {
	return fmt.Sprintf("%v-%v%v", entName, namespace, shovelQueue)
}

func ExtractNames(entities []interface{}) ([]string, error) {
	var names []string
	for _, entity := range entities {
		name, err := utils.ExtractName(entity)
		if err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// difference returns the elements in `a` that aren't in `b`.
func SubstractEntities(a []model.RabbitVersionedEntity, b []string) []model.RabbitVersionedEntity {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []model.RabbitVersionedEntity
	for _, x := range a {
		if _, found := mb[x.EntityName]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}
