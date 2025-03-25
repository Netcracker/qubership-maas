package cleanup

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/dao"
	"maas/maas-service/utils"
	"reflect"
	"runtime"
)

var log = logging.GetLogger("cleanup-service")

type NamespaceCleanupService struct {
	executors []func(context.Context, string) error
}

func NewNamespaceCleanupService(executors ...func(context.Context, string) error) *NamespaceCleanupService {
	return &NamespaceCleanupService{executors: executors}
}

func (gc *NamespaceCleanupService) DeleteNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Start cleanup procedure for namespace `%s'", namespace)

	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		for _, executor := range gc.executors {
			fname := runtime.FuncForPC(reflect.ValueOf(executor).Pointer()).Name()
			log.InfoC(ctx, "Execute cleanup: %v", fname)
			if err := executor(ctx, namespace); err != nil {
				return utils.LogError(log, ctx, "error cleanup namespace `%v' in %v: %w", namespace, fname, err)
			}
		}
		log.InfoC(ctx, "Cleanup procedure for namespace `%s' is finished", namespace)
		return nil
	})
}
