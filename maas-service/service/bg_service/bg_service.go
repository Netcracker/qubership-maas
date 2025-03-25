package bg_service

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/model"
	"maas/maas-service/utils"
	"maas/maas-service/utils/broadcast"
	"time"
)

//go:generate mockgen -source=bg_service.go -destination=mock_service/bg_service_mock.go
type BgService interface {
	ApplyBgStatus(ctx context.Context, namespace string, bgStatus *model.BgStatus) error
	GetBgStatusByNamespace(ctx context.Context, namespace string) (*model.BgStatus, error)
	GetActiveVersionByNamespace(ctx context.Context, namespace string) (string, error)
	CleanupNamespace(ctx context.Context, namespace string) error
	AddBgStatusUpdateCallback(cb func(ctx context.Context, bgStatusChange *BgStatusChangeEvent))
}

var log logging.Logger

func init() {
	log = logging.GetLogger("bg_service")
}

type BgStatusChangeEvent struct {
	Current   *model.BgStatus
	Prev      *model.BgStatus
	Namespace string
}

type BgServiceImpl struct {
	bgServiceDao            BgServiceDao
	bgStatusChangeBroadcast *broadcast.Broadcast[*BgStatusChangeEvent]
}

func NewBgService(dao BgServiceDao) *BgServiceImpl {
	return &BgServiceImpl{bgServiceDao: dao, bgStatusChangeBroadcast: broadcast.New[*BgStatusChangeEvent]()}
}

func (s *BgServiceImpl) AddBgStatusUpdateCallback(cb func(ctx context.Context, bgStatusChange *BgStatusChangeEvent)) {
	s.bgStatusChangeBroadcast.AddListener(cb)
}

func (bg BgServiceImpl) ApplyBgStatus(ctx context.Context, namespace string, bgStatus *model.BgStatus) error {
	bgStatus.Namespace = namespace
	bgStatus.Timestamp = time.Now()

	log.InfoC(ctx, "Starting to applying bg status: %+v", bgStatus)

	prevBgStatus, err := bg.bgServiceDao.GetBgStatusByNamespace(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error while GetBgStatusByNamespace. Error: %v ", err.Error())
		return err
	}

	log.InfoC(ctx, "Previous bg status: %+v", prevBgStatus)

	bg.bgStatusChangeBroadcast.ExecuteListeners(ctx, &BgStatusChangeEvent{Current: bgStatus, Prev: prevBgStatus, Namespace: namespace})

	//todo not add same rows if change is only date
	log.InfoC(ctx, "Inserting new bg status to DB")
	err = bg.bgServiceDao.InsertBgStatus(ctx, bgStatus)
	if err != nil {
		log.ErrorC(ctx, "Error while InsertBgStatus. Error: %v ", err.Error())
		return err
	}

	return nil
}

func (bg BgServiceImpl) GetBgStatusByNamespace(ctx context.Context, namespace string) (*model.BgStatus, error) {
	bgStatus, err := bg.bgServiceDao.GetBgStatusByNamespace(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during GetBgStatusByNamespace with namespace: %v, %w", namespace, err)
	}
	log.InfoC(ctx, "Bg status for namespace '%v': %+v", namespace, bgStatus)
	return bgStatus, nil
}

func (bg BgServiceImpl) GetActiveVersionByNamespace(ctx context.Context, namespace string) (string, error) {
	activeVersion, err := bg.bgServiceDao.GetActiveVersionByNamespace(ctx, namespace)
	if err != nil {
		return "", utils.LogError(log, ctx, "error during GetActiveVersionByNamespace with namespace: %v, %w", namespace, err)
	}
	log.InfoC(ctx, "Active version for namespace '%v': %+v", namespace, activeVersion)
	return activeVersion, nil
}

func (bg BgServiceImpl) CleanupNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "deleting cpMessages by namespace: %v", namespace)
	if err := bg.bgServiceDao.DeleteCpMessagesByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error deleting cpMessages by namespace: %w", err)
	}
	return nil
}
