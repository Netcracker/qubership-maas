package controller

import (
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"golang.org/x/exp/constraints"
	api_version "maas/maas-service/api-version"
	"maas/maas-service/model"
	"maas/maas-service/monitoring"
	"maas/maas-service/msg"
	"maas/maas-service/service/auth"
	"maas/maas-service/service/cleanup"
	"maas/maas-service/utils"
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

type GeneralController struct {
	cleanupService *cleanup.NamespaceCleanupService
	authService    auth.AuthService
	auditService   monitoring.Auditor
}

var gLog logging.Logger

func init() {
	gLog = logging.GetLogger("general-controller")
}

func NewGeneralController(s *cleanup.NamespaceCleanupService, a auth.AuthService, auditService monitoring.Auditor) *GeneralController {
	return &GeneralController{s, a, auditService}
}

func (g *GeneralController) ApiVersion(ctx *fiber.Ctx) error {
	return RespondWithJson(ctx, http.StatusOK, api_version.ApiVersion)
}

// @Summary Delete Namespace using Manager Role
// @Description Delete Namespace
// @ID DeleteNamespace
// @Tags V1
// @Produce  json
// @Param	request 		body     model.Namespace  true   "Namespace"
// @Security BasicAuth[manager]
// @Success 200
// @Failure 400 {object}    map[string]string
// @Failure 500 {object}	map[string]string
// @Router /api/v1/namespace [delete]
func (g *GeneralController) DeleteNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	gLog.DebugC(ctx, "Received request to DeleteNamespace")
	var namespaceReq model.Namespace
	if err := json.Unmarshal(fiberCtx.Body(), &namespaceReq); err != nil {
		return utils.LogError(log, ctx, "can not unmarshal JSON request: %s: %w", err.Error(), msg.BadRequest)
	}

	err := g.cleanupService.DeleteNamespace(ctx, namespaceReq.Namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error while deleting namespace: %v. Error: %s", namespaceReq, err.Error())
	}
	return RespondWithJson(fiberCtx, http.StatusOK, nil)
}

// @Summary Get Monitoring Entities using Manager Role
// @Description Get Monitoring Entities
// @ID GetMonitoringEntities
// @Tags V2
// @Security BasicAuth[manager]
// @Produce  json
// @Success 200 {object}    string
// @Failure 500 {object}	map[string]string
// @Router /api/v2/monitoring/entity-distribution [get]
func (g *GeneralController) GetMonitoringEntities(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.InfoC(ctx, "Received request to GetRabbitMonitoringEntities")

	rabbitEntities, err := g.auditService.GetRabbitMonitoringEntities(ctx)
	if err != nil {
		return utils.LogError(log, ctx, "error while GetRabbitMonitoringEntities: %s", err.Error())
	}

	kafkaEntities, err := g.auditService.GetKafkaMonitoringEntities(ctx)
	if err != nil {
		return utils.LogError(log, ctx, "error while GetKafkaMonitoringEntities: %s", err.Error())
	}

	stat := formatPrometheusStatLines("maas_kafka", kafkaEntities, 1)
	stat += "\n"
	stat += formatPrometheusStatLines("maas_rabbit", rabbitEntities, 1)

	return fiberCtx.Status(http.StatusOK).SendString(stat)
}

// @Summary Get Monitoring Entity Requests
// @Description Get Monitoring Entity Requests
// @ID GetMonitoringEntityRequests
// @Tags V2
// @Produce  json
// @Success 200 {object}    string
// @Failure 500 {object}	map[string]string
// @Router /api/v2/monitoring/entity-request-audit [get]
func (g *GeneralController) GetMonitoringEntityRequests(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.InfoC(ctx, "Received request to GetMonitoringEntityRequests")
	requests, err := g.auditService.GetAllEntityRequestsStat(ctx)
	if err != nil {
		return utils.LogError(log, ctx, "error while GetMonitoringEntityRequests: %s", err.Error())
	}

	log.DebugC(ctx, "Sending monitoring entities requests: %v", requests)

	var monitoringEntities []string
	for _, request := range *requests {
		var entityType string
		var entityName string
		switch request.EntityType {
		case monitoring.EntityTypeTopic:
			entityType = "kafka"
			entityName = "topic"
		case monitoring.EntityTypeVhost:
			entityType = "rabbit"
			entityName = "vhost"
		default:
			panic(fmt.Sprintf("unsupported entity type: %d", request.EntityType))
		}
		monitoringEntities = append(monitoringEntities,
			fmt.Sprintf("%s{%s} %v",
				fmt.Sprintf("maas_%s_entity_requests", entityType),
				fmt.Sprintf(asPrometheusLabels(&request), entityName),
				strconv.FormatInt(request.RequestsTotal, 10),
			),
		)
	}
	return fiberCtx.Status(http.StatusOK).SendString(strings.Join(monitoringEntities, "\n"))
}

func formatPrometheusStatLines[T any, V constraints.Integer | constraints.Float](prefix string, entities *[]T, value V) string {
	var statLines []string
	for _, entity := range *entities {
		statLines = append(statLines, fmt.Sprintf("%s{%s} %v", prefix, asPrometheusLabels(&entity), value))
	}
	return strings.Join(statLines, "\n")
}

func asPrometheusLabels[T any](entity *T) string {
	var labels []string
	e := reflect.Indirect(reflect.ValueOf(entity))
	if e.Kind() != reflect.Struct {
		e = e.Elem()
	}
	for i := 0; i < e.NumField(); i++ {
		varName := strings.ToLower(e.Type().Field(i).Name)
		if tagName, ok := e.Type().Field(i).Tag.Lookup("prometheus_label"); ok {
			if tagName == "-" {
				continue
			}
			varName = tagName
		}
		varValue := e.Field(i).Interface()
		labels = append(labels, fmt.Sprintf("%s=\"%v\"", varName, varValue))
	}
	return strings.Join(labels, ", ")
}
