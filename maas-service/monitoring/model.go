package monitoring

import (
	"maas/maas-service/watchdog"
	"time"
)

type EntityType int

const (
	EntityTypeTopic EntityType = iota + 1
	EntityTypeVhost
)

type KafkaStatLine struct {
	Namespace    string          `prometheus_label:"namespace"`
	BrokerHost   string          `prometheus_label:"broker_host"`
	Topic        string          `prometheus_label:"topic"`
	BrokerStatus watchdog.Status `prometheus_label:"broker_status"`
}

type VhostStatLine struct {
	Namespace    string          `prometheus_label:"namespace"`
	BrokerHost   string          `prometheus_label:"broker_host"`
	Microservice string          `prometheus_label:"microservice"`
	Vhost        string          `prometheus_label:"vhost"`
	BrokerStatus watchdog.Status `prometheus_label:"broker_status"`
}

type EntityRequestStat struct {
	Namespace     string     `gorm:"primaryKey"`
	Microservice  string     `gorm:"primaryKey"`
	EntityType    EntityType `gorm:"primaryKey"`
	Name          string     `gorm:"primaryKey"`
	Instance      string
	LastRequestTs time.Time `gorm:"default:now()"`
	RequestsTotal int64     `gorm:"default:1"`
}

func (e EntityRequestStat) AsDto() EntityRequestStatDto {
	return EntityRequestStatDto{
		Namespace:     e.Namespace,
		Microservice:  e.Microservice,
		Name:          e.Name,
		Instance:      e.Instance,
		EntityType:    e.EntityType,
		lastRequestTs: e.LastRequestTs,
		RequestsTotal: e.RequestsTotal,
	}
}

func (e EntityRequestStat) TableName() string {
	return "entity_request_stat"
}

type EntityRequestStatDto struct {
	Namespace     string     `prometheus_label:"namespace"`
	Microservice  string     `prometheus_label:"microservice"`
	Name          string     `prometheus_label:"%[1]s"` // will be replaced later
	Instance      string     `prometheus_label:"instance"`
	EntityType    EntityType `prometheus_label:"-"`
	lastRequestTs time.Time  `prometheus_label:"-"`
	RequestsTotal int64      `prometheus_label:"-"`
}

func (er *EntityRequestStatDto) AsModel() *EntityRequestStat {
	return &EntityRequestStat{
		Namespace:     er.Namespace,
		Microservice:  er.Microservice,
		EntityType:    er.EntityType,
		Name:          er.Name,
		Instance:      er.Instance,
		LastRequestTs: er.lastRequestTs,
		RequestsTotal: er.RequestsTotal,
	}
}

type MonitoringVhost struct {
	Namespace    string
	Microservice string
	BrokerHost   string
	Vhost        string
	InstanceID   string
}

type MonitoringTopic struct {
	Namespace  string
	BrokerHost string
	Topic      string
	InstanceID string
}
