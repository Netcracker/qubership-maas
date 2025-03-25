package controller

import (
	"github.com/gofiber/fiber/v2"
	"maas/maas-service/model"
	"maas/maas-service/service/kafka"
	"maas/maas-service/utils"
	"net/http"
)

type DiscrepancyController struct {
	kafkaService kafka.KafkaService
}

func NewDiscrepancyController(kafkaService kafka.KafkaService) *DiscrepancyController {
	return &DiscrepancyController{kafkaService: kafkaService}
}

// @Summary Get Report
// @Description Get Report
// @ID GetReport
// @Tags V2
// @Produce  json
// @Param namespace path string true "namespace"
// @Param status query string true "status"
// @Success 200 {array}    model.DiscrepancyReportItem
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/kafka/discrepancy-report/{namespace} [get]
func (c *DiscrepancyController) GetReport(fiberCtx *fiber.Ctx) error {
	namespace := fiberCtx.Params("namespace")
	statusParam := fiberCtx.Query("status")

	report, err := c.kafkaService.GetDiscrepancyReport(fiberCtx.UserContext(), namespace, func(item model.DiscrepancyReportItem) bool {
		if statusParam == "" {
			return true
		}
		return item.Status == statusParam
	})
	if err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "can not get kafka discrepancy report: %s", err.Error())
	}
	return RespondWithJson(fiberCtx, http.StatusOK, report)
}
