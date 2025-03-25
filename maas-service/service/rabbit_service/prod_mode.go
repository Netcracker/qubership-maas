package rabbit_service

import (
	"context"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/utils"
)

type ProdMode struct {
	RabbitService

	isProdMode bool
}

func NewProdMode(RabbitService RabbitService, isProdMode bool) RabbitService {
	return &ProdMode{RabbitService, isProdMode}
}

func (pm *ProdMode) RemoveVHosts(ctx context.Context, searchForm *model.SearchForm, defaultNamespace string) error {
	if pm.isProdMode {
		return utils.LogError(log, ctx, "VHost deletion is not allowed in production mode: %w", msg.BadRequest)
	}
	return pm.RabbitService.RemoveVHosts(ctx, searchForm, defaultNamespace)
}
